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

package kubeconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
)

const tokenKubeconfig = `apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://api.cluster.example.com:6443
    certificate-authority-data: FAKECA==
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
    namespace: default
users:
- name: my-user
  user:
    token: sha256~real-token-value
`

const certKubeconfig = `apiVersion: v1
kind: Config
current-context: cert-context
clusters:
- name: cert-cluster
  cluster:
    server: https://api.cert.example.com:6443
contexts:
- name: cert-context
  context:
    cluster: cert-cluster
    user: cert-user
users:
- name: cert-user
  user:
    client-certificate-data: FAKECERT==
    client-key-data: FAKEKEY==
`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

func TestKubeconfigCredential_Name(t *testing.T) {
	t.Parallel()
	if got := New().Name(); got != "kubeconfig" {
		t.Errorf("Name() = %q, want %q", got, "kubeconfig")
	}
}

func TestKubeconfigCredential_ContainerFilePath(t *testing.T) {
	t.Parallel()
	want := "/home/agent/.kube/config"
	if got := New().ContainerFilePath(); got != want {
		t.Errorf("ContainerFilePath() = %q, want %q", got, want)
	}
}

func TestKubeconfigCredential_Detect(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	kubeDir := filepath.Join(homeDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	kubeConfigPath := filepath.Join(kubeDir, "config")

	tests := []struct {
		name          string
		setup         func()
		mounts        []workspace.Mount
		wantNil       bool
		wantHostPath  string
		wantMountHost string
	}{
		{
			name:    "no mounts",
			mounts:  nil,
			wantNil: true,
		},
		{
			name: "unrelated mount",
			mounts: []workspace.Mount{
				{Host: "$HOME/projects", Target: "$HOME/projects"},
			},
			wantNil: true,
		},
		{
			name: "file mount token-based",
			setup: func() {
				if err := os.WriteFile(kubeConfigPath, []byte(tokenKubeconfig), 0600); err != nil {
					t.Fatalf("write kubeconfig: %v", err)
				}
			},
			mounts: []workspace.Mount{
				{Host: "$HOME/.kube/config", Target: "$HOME/.kube/config"},
			},
			wantNil:       false,
			wantHostPath:  kubeConfigPath,
			wantMountHost: "$HOME/.kube/config",
		},
		{
			name: "directory mount token-based",
			setup: func() {
				if err := os.WriteFile(kubeConfigPath, []byte(tokenKubeconfig), 0600); err != nil {
					t.Fatalf("write kubeconfig: %v", err)
				}
			},
			mounts: []workspace.Mount{
				{Host: "$HOME/.kube", Target: "$HOME/.kube"},
			},
			wantNil:       false,
			wantHostPath:  kubeConfigPath,
			wantMountHost: "$HOME/.kube",
		},
		{
			name: "file mount cert-based returns nil",
			setup: func() {
				if err := os.WriteFile(kubeConfigPath, []byte(certKubeconfig), 0600); err != nil {
					t.Fatalf("write kubeconfig: %v", err)
				}
			},
			mounts: []workspace.Mount{
				{Host: "$HOME/.kube/config", Target: "$HOME/.kube/config"},
			},
			wantNil: true,
		},
		{
			name: "file mount but kubeconfig missing",
			setup: func() {
				_ = os.Remove(kubeConfigPath)
			},
			mounts: []workspace.Mount{
				{Host: "$HOME/.kube/config", Target: "$HOME/.kube/config"},
			},
			wantNil: true,
		},
		{
			name: "absolute host path token-based",
			setup: func() {
				if err := os.WriteFile(kubeConfigPath, []byte(tokenKubeconfig), 0600); err != nil {
					t.Fatalf("write kubeconfig: %v", err)
				}
			},
			mounts: []workspace.Mount{
				{Host: kubeConfigPath, Target: "$HOME/.kube/config"},
			},
			wantNil:       false,
			wantHostPath:  kubeConfigPath,
			wantMountHost: kubeConfigPath,
		},
		{
			name: "absolute container target token-based",
			setup: func() {
				if err := os.WriteFile(kubeConfigPath, []byte(tokenKubeconfig), 0600); err != nil {
					t.Fatalf("write kubeconfig: %v", err)
				}
			},
			mounts: []workspace.Mount{
				{Host: "$HOME/.kube/config", Target: "/home/agent/.kube/config"},
			},
			wantNil:       false,
			wantHostPath:  kubeConfigPath,
			wantMountHost: "$HOME/.kube/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: cannot use t.Parallel() because tests share the kubeconfig file.
			if tt.setup != nil {
				tt.setup()
			}

			cred := New()
			gotPath, gotMount := cred.Detect(tt.mounts, homeDir)

			if tt.wantNil {
				if gotPath != "" || gotMount != nil {
					t.Errorf("Detect() = (%q, %+v), want (\"\", nil)", gotPath, gotMount)
				}
				return
			}
			if gotPath == "" || gotMount == nil {
				t.Fatal("Detect() returned empty result, want non-nil match")
			}
			if gotPath != tt.wantHostPath {
				t.Errorf("Detect() hostFilePath = %q, want %q", gotPath, tt.wantHostPath)
			}
			if gotMount.Host != tt.wantMountHost {
				t.Errorf("Detect() intercepted.Host = %q, want %q", gotMount.Host, tt.wantMountHost)
			}
		})
	}
}

func TestKubeconfigCredential_FakeFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", tokenKubeconfig)

	cred := New()
	content, err := cred.FakeFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("FakeFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Fatal("FakeFile() returned empty content")
	}

	s := string(content)

	// Must contain placeholder token, not the real one.
	if !strings.Contains(s, tokenPlaceholder) {
		t.Errorf("FakeFile() does not contain placeholder %q", tokenPlaceholder)
	}
	if strings.Contains(s, "sha256~real-token-value") {
		t.Error("FakeFile() contains real token value")
	}

	// Must preserve the cluster server and current-context.
	if !strings.Contains(s, "api.cluster.example.com") {
		t.Error("FakeFile() does not contain cluster server hostname")
	}
	if !strings.Contains(s, "my-context") {
		t.Error("FakeFile() does not contain current context name")
	}

	// Must not contain entries from other contexts/users/clusters that weren't in this kubeconfig.
	// (Since tokenKubeconfig only has one context, this verifies pruning is consistent.)
	if strings.Contains(s, "cert-context") {
		t.Error("FakeFile() contains unrelated context")
	}
}

func TestKubeconfigCredential_FakeFile_MissingFile(t *testing.T) {
	t.Parallel()

	cred := New()
	_, err := cred.FakeFile("/nonexistent/path/config")
	if err == nil {
		t.Fatal("FakeFile() error = nil, want error for missing file")
	}
}

func TestKubeconfigCredential_HostPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", tokenKubeconfig)

	cred := New()
	patterns := cred.HostPatterns(kubeconfigPath)
	if len(patterns) == 0 {
		t.Fatal("HostPatterns() returned empty slice")
	}
	if patterns[0] != "api.cluster.example.com" {
		t.Errorf("HostPatterns()[0] = %q, want %q", patterns[0], "api.cluster.example.com")
	}
}

func TestKubeconfigCredential_HostPatterns_Missing(t *testing.T) {
	t.Parallel()

	cred := New()
	patterns := cred.HostPatterns("/nonexistent/config")
	if len(patterns) != 0 {
		t.Errorf("HostPatterns() = %v, want empty for missing file", patterns)
	}
}

func TestLoadKubeConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for missing file", func(t *testing.T) {
		t.Parallel()

		cfg, err := loadKubeConfig("/nonexistent/config")
		if err != nil {
			t.Fatalf("loadKubeConfig() error = %v, want nil", err)
		}
		if cfg != nil {
			t.Errorf("loadKubeConfig() = %+v, want nil", cfg)
		}
	})

	t.Run("parses token-based kubeconfig", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", tokenKubeconfig)

		cfg, err := loadKubeConfig(path)
		if err != nil {
			t.Fatalf("loadKubeConfig() error = %v", err)
		}
		if cfg == nil {
			t.Fatal("loadKubeConfig() = nil, want non-nil")
		}
		if cfg.CurrentContext != "my-context" {
			t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, "my-context")
		}
		if len(cfg.Clusters) != 1 {
			t.Fatalf("Clusters len = %d, want 1", len(cfg.Clusters))
		}
		if cfg.Clusters[0].Cluster.Server != "https://api.cluster.example.com:6443" {
			t.Errorf("Server = %q, want %q", cfg.Clusters[0].Cluster.Server, "https://api.cluster.example.com:6443")
		}
		user := findUser(cfg, "my-user")
		if user == nil {
			t.Fatal("findUser() = nil, want non-nil")
		}
		if user.Token != "sha256~real-token-value" {
			t.Errorf("Token = %q, want %q", user.Token, "sha256~real-token-value")
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", "}{invalid yaml")

		_, err := loadKubeConfig(path)
		if err == nil {
			t.Fatal("loadKubeConfig() error = nil, want error for invalid YAML")
		}
	})
}

// noContextKubeconfig has a current-context that references no entry in contexts.
const noContextKubeconfig = `apiVersion: v1
kind: Config
current-context: nonexistent
clusters:
- name: my-cluster
  cluster:
    server: https://api.cluster.example.com:6443
contexts: []
users: []
`

// noUserKubeconfig has a context that references a user absent from the users list.
const noUserKubeconfig = `apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: https://api.cluster.example.com:6443
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: nonexistent-user
users: []
`

// noClusterKubeconfig has a context that references a cluster absent from the clusters list.
const noClusterKubeconfig = `apiVersion: v1
kind: Config
current-context: my-context
clusters: []
contexts:
- name: my-context
  context:
    cluster: nonexistent-cluster
    user: my-user
users:
- name: my-user
  user:
    token: sha256~real-token-value
`

// emptyServerKubeconfig has a cluster whose server URL has an empty hostname.
const emptyServerKubeconfig = `apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: ""
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
users:
- name: my-user
  user:
    token: sha256~real-token-value
`

// invalidServerKubeconfig has a cluster whose server URL is syntactically invalid.
const invalidServerKubeconfig = `apiVersion: v1
kind: Config
current-context: my-context
clusters:
- name: my-cluster
  cluster:
    server: "http://[invalid"
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
users:
- name: my-user
  user:
    token: sha256~real-token-value
`

// fakeOnecliClient is a test double for onecli.Client.
type fakeOnecliClient struct {
	createSecretErr error
	createdSecrets  []onecli.CreateSecretInput
}

var _ onecli.Client = (*fakeOnecliClient)(nil)

func (f *fakeOnecliClient) CreateSecret(_ context.Context, input onecli.CreateSecretInput) (*onecli.Secret, error) {
	if f.createSecretErr != nil {
		return nil, f.createSecretErr
	}
	f.createdSecrets = append(f.createdSecrets, input)
	return &onecli.Secret{Name: input.Name}, nil
}
func (f *fakeOnecliClient) UpdateSecret(_ context.Context, _ string, _ onecli.UpdateSecretInput) error {
	return nil
}
func (f *fakeOnecliClient) ListSecrets(_ context.Context) ([]onecli.Secret, error) { return nil, nil }
func (f *fakeOnecliClient) DeleteSecret(_ context.Context, _ string) error         { return nil }
func (f *fakeOnecliClient) GetContainerConfig(_ context.Context) (*onecli.ContainerConfig, error) {
	return nil, nil
}
func (f *fakeOnecliClient) CreateRule(_ context.Context, _ onecli.CreateRuleInput) (*onecli.Rule, error) {
	return nil, nil
}
func (f *fakeOnecliClient) ListRules(_ context.Context) ([]onecli.Rule, error) { return nil, nil }
func (f *fakeOnecliClient) DeleteRule(_ context.Context, _ string) error       { return nil }
func (f *fakeOnecliClient) ConnectApp(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

func TestIsTokenBased(t *testing.T) {
	t.Parallel()

	t.Run("token-based returns true", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", tokenKubeconfig)
		cfg, _ := loadKubeConfig(path)
		if !isTokenBased(cfg) {
			t.Error("isTokenBased() = false, want true for token-based kubeconfig")
		}
	})

	t.Run("cert-based returns false", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", certKubeconfig)
		cfg, _ := loadKubeConfig(path)
		if isTokenBased(cfg) {
			t.Error("isTokenBased() = true, want false for cert-based kubeconfig")
		}
	})

	t.Run("context not found returns false", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", noContextKubeconfig)
		cfg, _ := loadKubeConfig(path)
		if isTokenBased(cfg) {
			t.Error("isTokenBased() = true, want false when context not found")
		}
	})

	t.Run("user not found returns false", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", noUserKubeconfig)
		cfg, _ := loadKubeConfig(path)
		if isTokenBased(cfg) {
			t.Error("isTokenBased() = true, want false when user not found")
		}
	})
}

func TestBuildFakeKubeConfig(t *testing.T) {
	t.Parallel()

	t.Run("context not found returns error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", noContextKubeconfig)
		cfg, _ := loadKubeConfig(path)
		_, err := buildFakeKubeConfig(cfg)
		if err == nil {
			t.Fatal("buildFakeKubeConfig() error = nil, want error when context not found")
		}
	})

	t.Run("cluster not found returns error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "config", noClusterKubeconfig)
		cfg, _ := loadKubeConfig(path)
		_, err := buildFakeKubeConfig(cfg)
		if err == nil {
			t.Fatal("buildFakeKubeConfig() error = nil, want error when cluster not found")
		}
	})
}

func TestKubeconfigCredential_FakeFile_InvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", "}{invalid yaml")

	cred := New()
	_, err := cred.FakeFile(kubeconfigPath)
	if err == nil {
		t.Fatal("FakeFile() error = nil, want error for invalid YAML")
	}
}

func TestKubeconfigCredential_FakeFile_NoContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", noContextKubeconfig)

	cred := New()
	_, err := cred.FakeFile(kubeconfigPath)
	if err == nil {
		t.Fatal("FakeFile() error = nil, want error when current context not found")
	}
}

func TestKubeconfigCredential_FakeFile_NoCluster(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", noClusterKubeconfig)

	cred := New()
	_, err := cred.FakeFile(kubeconfigPath)
	if err == nil {
		t.Fatal("FakeFile() error = nil, want error when cluster not found")
	}
}

func TestKubeconfigCredential_HostPatterns_NoContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", noContextKubeconfig)

	cred := New()
	if patterns := cred.HostPatterns(kubeconfigPath); len(patterns) != 0 {
		t.Errorf("HostPatterns() = %v, want empty when context not found", patterns)
	}
}

func TestKubeconfigCredential_HostPatterns_NoCluster(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", noClusterKubeconfig)

	cred := New()
	if patterns := cred.HostPatterns(kubeconfigPath); len(patterns) != 0 {
		t.Errorf("HostPatterns() = %v, want empty when cluster not found", patterns)
	}
}

func TestKubeconfigCredential_HostPatterns_EmptyHostname(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", emptyServerKubeconfig)

	cred := New()
	if patterns := cred.HostPatterns(kubeconfigPath); len(patterns) != 0 {
		t.Errorf("HostPatterns() = %v, want empty when server hostname is empty", patterns)
	}
}

func TestKubeconfigCredential_HostPatterns_InvalidURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kubeconfigPath := writeFile(t, dir, "config", invalidServerKubeconfig)

	cred := New()
	if patterns := cred.HostPatterns(kubeconfigPath); len(patterns) != 0 {
		t.Errorf("HostPatterns() = %v, want empty when server URL is invalid", patterns)
	}
}

func TestHostFromURL(t *testing.T) {
	t.Parallel()

	t.Run("valid URL returns hostname", func(t *testing.T) {
		t.Parallel()

		host, err := hostFromURL("https://api.example.com:6443")
		if err != nil {
			t.Fatalf("hostFromURL() error = %v", err)
		}
		if host != "api.example.com" {
			t.Errorf("hostFromURL() = %q, want %q", host, "api.example.com")
		}
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		t.Parallel()

		if _, err := hostFromURL("http://[invalid"); err == nil {
			t.Error("hostFromURL() error = nil, want error for invalid URL")
		}
	})
}

func TestSanitizeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"my-context", "my-context"},
		{"abc123", "abc123"},
		{"my/context", "my-context"},
		{"my context", "my-context"},
		{"a@b:c/d", "a-b-c-d"},
		{"--leading", "leading"},
		{"trailing--", "trailing"},
		{"multiple---dashes", "multiple-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeName(tt.input); got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKubeconfigCredential_Configure(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", tokenKubeconfig)

		client := &fakeOnecliClient{}
		cred := New()
		if err := cred.Configure(context.Background(), client, kubeconfigPath); err != nil {
			t.Fatalf("Configure() error = %v", err)
		}
		if len(client.createdSecrets) != 1 {
			t.Fatalf("Configure() created %d secrets, want 1", len(client.createdSecrets))
		}
		secret := client.createdSecrets[0]
		if secret.Value != "sha256~real-token-value" {
			t.Errorf("Configure() secret value = %q, want real token", secret.Value)
		}
		if secret.HostPattern != "api.cluster.example.com" {
			t.Errorf("Configure() hostPattern = %q, want cluster hostname", secret.HostPattern)
		}
		if secret.InjectionConfig == nil || secret.InjectionConfig.HeaderName != "Authorization" {
			t.Errorf("Configure() injection header = %v, want Authorization", secret.InjectionConfig)
		}
		if !strings.HasSuffix(secret.Name, "my-context") {
			t.Errorf("Configure() secret name = %q, want suffix %q", secret.Name, "my-context")
		}
	})

	t.Run("missing kubeconfig", func(t *testing.T) {
		t.Parallel()

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, "/nonexistent/config"); err == nil {
			t.Fatal("Configure() error = nil, want error for missing kubeconfig")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", "}{invalid yaml")

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error for invalid YAML")
		}
	})

	t.Run("context not found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", noContextKubeconfig)

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error when context not found")
		}
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", noUserKubeconfig)

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error when user not found")
		}
	})

	t.Run("cluster not found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", noClusterKubeconfig)

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error when cluster not found")
		}
	})

	t.Run("invalid server URL", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", invalidServerKubeconfig)

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error for invalid server URL")
		}
	})

	t.Run("empty server hostname", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", emptyServerKubeconfig)

		cred := New()
		if err := cred.Configure(context.Background(), &fakeOnecliClient{}, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error for empty server hostname")
		}
	})

	t.Run("provision error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		kubeconfigPath := writeFile(t, dir, "config", tokenKubeconfig)

		client := &fakeOnecliClient{createSecretErr: errors.New("provision failed")}
		cred := New()
		if err := cred.Configure(context.Background(), client, kubeconfigPath); err == nil {
			t.Fatal("Configure() error = nil, want error when provision fails")
		}
	})
}
