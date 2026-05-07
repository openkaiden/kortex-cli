/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

// Package kubeconfig implements the credential.Credential interface for
// Kubernetes token-based authentication. When a workspace mount targets the
// host kubeconfig file or directory and the current context uses token-based
// auth, this credential intercepts the mount and substitutes a pruned
// kubeconfig containing only the current context with a placeholder token.
// The real token is forwarded through the OneCLI proxy as a Bearer
// Authorization header on requests to the cluster.
package kubeconfig

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/onecli"
)

const (
	containerHomeDir  = "/home/agent"
	containerKubeDir  = containerHomeDir + "/.kube"
	containerKubePath = containerKubeDir + "/config"
	tokenPlaceholder  = "sha256-placeholder"
)

// kubeConfig is a minimal representation of a kubeconfig file.
type kubeConfig struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	CurrentContext string         `yaml:"current-context"`
	Clusters       []namedCluster `yaml:"clusters"`
	Contexts       []namedContext `yaml:"contexts"`
	Users          []namedUser    `yaml:"users"`
}

type namedCluster struct {
	Name    string      `yaml:"name"`
	Cluster clusterInfo `yaml:"cluster"`
}

type clusterInfo struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify,omitempty"`
}

type namedContext struct {
	Name    string      `yaml:"name"`
	Context contextInfo `yaml:"context"`
}

type contextInfo struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

type namedUser struct {
	Name string   `yaml:"name"`
	User userInfo `yaml:"user"`
}

type userInfo struct {
	Token                 string `yaml:"token,omitempty"`
	ClientCertificate     string `yaml:"client-certificate,omitempty"`
	ClientCertificateData string `yaml:"client-certificate-data,omitempty"`
	ClientKey             string `yaml:"client-key,omitempty"`
	ClientKeyData         string `yaml:"client-key-data,omitempty"`
}

// kubeconfigCredential implements credential.Credential for Kubernetes token auth.
type kubeconfigCredential struct{}

// Compile-time check that kubeconfigCredential implements the Credential interface.
var _ credential.Credential = (*kubeconfigCredential)(nil)

// New returns a new kubeconfig Credential implementation.
func New() credential.Credential {
	return &kubeconfigCredential{}
}

// Name returns the credential identifier.
func (o *kubeconfigCredential) Name() string {
	return "kubeconfig"
}

// ContainerFilePath returns the kubeconfig path inside the container.
func (o *kubeconfigCredential) ContainerFilePath() string {
	return containerKubePath
}

// Detect scans workspace mounts for one whose target resolves to the kubeconfig
// path inside the container (/home/agent/.kube/config or /home/agent/.kube).
// If found, the real kubeconfig is read from the host path and checked for
// token-based authentication. Returns the host kubeconfig path and the
// intercepted mount, or ("", nil) when the mount is absent or authentication
// is not token-based.
func (o *kubeconfigCredential) Detect(mounts []workspace.Mount, homeDir string) (string, *workspace.Mount) {
	for i := range mounts {
		m := mounts[i]
		target := resolveTarget(m.Target)

		var kubeconfigHostPath string
		switch target {
		case containerKubePath:
			kubeconfigHostPath = resolveHost(m.Host, homeDir)
		case containerKubeDir:
			kubeconfigHostPath = filepath.Join(resolveHost(m.Host, homeDir), "config")
		default:
			continue
		}

		cfg, err := loadKubeConfig(kubeconfigHostPath)
		if err != nil || cfg == nil {
			continue
		}
		if !isTokenBased(cfg) {
			return "", nil
		}
		return kubeconfigHostPath, &mounts[i]
	}
	return "", nil
}

// FakeFile returns a minimal kubeconfig containing only the current context,
// its cluster (with server and CA intact), and the current user with the real
// token replaced by a placeholder.
func (o *kubeconfigCredential) FakeFile(hostFilePath string) ([]byte, error) {
	cfg, err := loadKubeConfig(hostFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading kubeconfig from %s: %w", hostFilePath, err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("kubeconfig not found at %s", hostFilePath)
	}

	fake, err := buildFakeKubeConfig(cfg)
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(fake)
	if err != nil {
		return nil, fmt.Errorf("marshaling fake kubeconfig: %w", err)
	}
	return data, nil
}

// Configure reads the real kubeconfig at hostFilePath, extracts the current
// context's token, and registers an OneCLI secret that injects the token as an
// Authorization: Bearer header for requests to the cluster's API server.
func (o *kubeconfigCredential) Configure(ctx context.Context, client onecli.Client, hostFilePath string) error {
	cfg, err := loadKubeConfig(hostFilePath)
	if err != nil {
		return fmt.Errorf("reading kubeconfig from %s: %w", hostFilePath, err)
	}
	if cfg == nil {
		return fmt.Errorf("kubeconfig not found at %s", hostFilePath)
	}

	ctxInfo := findContext(cfg, cfg.CurrentContext)
	if ctxInfo == nil {
		return fmt.Errorf("current context %q not found in kubeconfig", cfg.CurrentContext)
	}
	user := findUser(cfg, ctxInfo.User)
	if user == nil {
		return fmt.Errorf("user %q not found in kubeconfig", ctxInfo.User)
	}
	cluster := findCluster(cfg, ctxInfo.Cluster)
	if cluster == nil {
		return fmt.Errorf("cluster %q not found in kubeconfig", ctxInfo.Cluster)
	}

	serverHost, err := hostFromURL(cluster.Cluster.Server)
	if err != nil {
		return fmt.Errorf("parsing cluster server URL %q: %w", cluster.Cluster.Server, err)
	}

	if serverHost == "" {
		return fmt.Errorf("cluster server URL %q has no hostname", cluster.Cluster.Server)
	}

	provisioner := onecli.NewSecretProvisioner(client)
	return provisioner.ProvisionSecrets(ctx, []onecli.CreateSecretInput{
		{
			Name:        "kubeconfig-" + sanitizeName(cfg.CurrentContext),
			Type:        "generic",
			Value:       user.Token,
			HostPattern: serverHost,
			InjectionConfig: &onecli.InjectionConfig{
				HeaderName:  "Authorization",
				ValueFormat: "Bearer {value}",
			},
		},
	})
}

// HostPatterns returns the API server hostname derived from the kubeconfig
// current context so it can be allowed in deny-mode networking.
func (o *kubeconfigCredential) HostPatterns(hostFilePath string) []string {
	cfg, err := loadKubeConfig(hostFilePath)
	if err != nil || cfg == nil {
		return nil
	}
	ctxInfo := findContext(cfg, cfg.CurrentContext)
	if ctxInfo == nil {
		return nil
	}
	cluster := findCluster(cfg, ctxInfo.Cluster)
	if cluster == nil {
		return nil
	}
	host, err := hostFromURL(cluster.Cluster.Server)
	if err != nil || host == "" {
		return nil
	}
	return []string{host}
}

// loadKubeConfig reads and parses the kubeconfig at path.
// Returns (nil, nil) if the file does not exist.
func loadKubeConfig(path string) (*kubeConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var cfg kubeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing kubeconfig YAML: %w", err)
	}
	return &cfg, nil
}

// isTokenBased reports whether the kubeconfig's current context authenticates
// via a bearer token (as opposed to client certificates or other methods).
func isTokenBased(cfg *kubeConfig) bool {
	ctxInfo := findContext(cfg, cfg.CurrentContext)
	if ctxInfo == nil {
		return false
	}
	user := findUser(cfg, ctxInfo.User)
	if user == nil {
		return false
	}
	return user.Token != ""
}

// buildFakeKubeConfig returns a pruned kubeconfig containing only the current
// context, its cluster, and the current user with a placeholder token.
func buildFakeKubeConfig(cfg *kubeConfig) (*kubeConfig, error) {
	ctxInfo := findContext(cfg, cfg.CurrentContext)
	if ctxInfo == nil {
		return nil, fmt.Errorf("current context %q not found in kubeconfig", cfg.CurrentContext)
	}
	cluster := findCluster(cfg, ctxInfo.Cluster)
	if cluster == nil {
		return nil, fmt.Errorf("cluster %q not found in kubeconfig", ctxInfo.Cluster)
	}
	return &kubeConfig{
		APIVersion:     cfg.APIVersion,
		Kind:           cfg.Kind,
		CurrentContext: cfg.CurrentContext,
		Clusters:       []namedCluster{*cluster},
		Contexts: []namedContext{{
			Name:    cfg.CurrentContext,
			Context: *ctxInfo,
		}},
		Users: []namedUser{{
			Name: ctxInfo.User,
			User: userInfo{Token: tokenPlaceholder},
		}},
	}, nil
}

func findContext(cfg *kubeConfig, name string) *contextInfo {
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Name == name {
			return &cfg.Contexts[i].Context
		}
	}
	return nil
}

func findCluster(cfg *kubeConfig, name string) *namedCluster {
	for i := range cfg.Clusters {
		if cfg.Clusters[i].Name == name {
			return &cfg.Clusters[i]
		}
	}
	return nil
}

func findUser(cfg *kubeConfig, name string) *userInfo {
	for i := range cfg.Users {
		if cfg.Users[i].Name == name {
			return &cfg.Users[i].User
		}
	}
	return nil
}

func hostFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

var nonAlphanumRun = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeName(s string) string {
	return strings.Trim(nonAlphanumRun.ReplaceAllString(s, "-"), "-")
}

// resolveHost expands $HOME in a mount host path and returns the absolute path.
func resolveHost(host, homeDir string) string {
	if strings.HasPrefix(host, "$HOME") {
		rest := filepath.FromSlash(host[len("$HOME"):])
		return filepath.Join(homeDir, rest)
	}
	return filepath.Clean(host)
}

// resolveTarget expands $HOME in a mount target path and returns the absolute
// container-side path.
func resolveTarget(target string) string {
	if strings.HasPrefix(target, "$HOME") {
		return path.Join(containerHomeDir, target[len("$HOME"):])
	}
	return path.Clean(target)
}
