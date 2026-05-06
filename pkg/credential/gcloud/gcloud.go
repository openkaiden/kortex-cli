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

// Package gcloud implements the credential.Credential interface for Google Cloud
// Application Default Credentials (ADC). When a workspace mount targets the
// gcloud config directory or the ADC file directly, this credential intercepts
// the mount and substitutes a placeholder file. Real authentication is forwarded
// through the OneCLI proxy via the Vertex AI app connection.
package gcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/onecli"
)

const (
	containerHomeDir   = "/home/agent"
	containerGcloudDir = containerHomeDir + "/.config/gcloud"
	containerADCPath   = containerGcloudDir + "/application_default_credentials.json"
)

// adcCredentials holds the fields from application_default_credentials.json
// needed to configure the Vertex AI app in OneCLI.
type adcCredentials struct {
	RefreshToken   string `json:"refresh_token"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	QuotaProjectID string `json:"quota_project_id"`
}

// vertexAIFields returns the credential fields for the OneCLI vertex-ai connect API.
func (c *adcCredentials) vertexAIFields() map[string]string {
	return map[string]string{
		"refreshToken":   c.RefreshToken,
		"clientId":       c.ClientID,
		"clientSecret":   c.ClientSecret,
		"quotaProjectId": c.QuotaProjectID,
	}
}

// vertexAIHosts lists the host patterns that must be allowed in deny-mode
// networking when Vertex AI is configured.
var vertexAIHosts = []string{
	"oauth2.googleapis.com",
	"aiplatform.googleapis.com",
	"*-aiplatform.googleapis.com",
}

// fakeADCJSON is written into the workspace container at the standard gcloud
// credentials path. It has synthetic values so that gcloud-aware tools see a
// credentials file without exposing real secrets; actual auth goes through the
// OneCLI proxy which holds the real refresh token.
var fakeADCJSON = []byte(`{
  "account": "",
  "client_id": "FAKE_CLIENT_ID",
  "client_secret": "FAKE_CLIENT_SECRET",
  "quota_project_id": "FAKE_QUOTA_PROJECT_ID",
  "refresh_token": "FAKE_REFRESH_TOKEN",
  "type": "authorized_user",
  "universe_domain": "googleapis.com"
}
`)

// gcloudCredential implements credential.Credential for Google Cloud ADC.
type gcloudCredential struct{}

// Compile-time check that gcloudCredential implements the Credential interface.
var _ credential.Credential = (*gcloudCredential)(nil)

// New returns a new gcloud Credential implementation.
func New() credential.Credential {
	return &gcloudCredential{}
}

// Name returns the credential identifier.
func (g *gcloudCredential) Name() string {
	return "gcloud"
}

// ContainerFilePath returns the ADC file path inside the container.
func (g *gcloudCredential) ContainerFilePath() string {
	return containerADCPath
}

// Detect scans workspace mounts for one targeting the gcloud config directory
// or the ADC file directly. The target path is checked after resolving $HOME.
// Returns the host path to the real ADC file and the intercepted mount, or
// ("", nil) when no matching mount is found.
func (g *gcloudCredential) Detect(mounts []workspace.Mount, homeDir string) (string, *workspace.Mount) {
	for i := range mounts {
		m := mounts[i]
		target := resolveTarget(m.Target)
		switch target {
		case containerGcloudDir:
			hostDir := resolveHost(m.Host, homeDir)
			return filepath.Join(hostDir, "application_default_credentials.json"), &mounts[i]
		case containerADCPath:
			return resolveHost(m.Host, homeDir), &mounts[i]
		}
	}
	return "", nil
}

// FakeFile returns the static placeholder ADC JSON. The real hostFilePath is
// not needed because the fake file is always the same regardless of credentials.
func (g *gcloudCredential) FakeFile(_ string) ([]byte, error) {
	return fakeADCJSON, nil
}

// Configure reads the real ADC file from hostFilePath and calls the OneCLI
// vertex-ai connect API with the parsed credentials.
func (g *gcloudCredential) Configure(ctx context.Context, client onecli.Client, hostFilePath string) error {
	adc, err := loadADC(hostFilePath)
	if err != nil {
		return fmt.Errorf("reading ADC from %s: %w", hostFilePath, err)
	}
	if adc == nil {
		return fmt.Errorf("ADC file not found at %s", hostFilePath)
	}
	if err := client.ConnectApp(ctx, "vertex-ai", adc.vertexAIFields()); err != nil {
		return fmt.Errorf("configuring Vertex AI: %w", err)
	}
	return nil
}

// HostPatterns returns the static list of Google auth and Vertex AI endpoints
// that must be allowed in deny-mode networking when this credential is active.
func (g *gcloudCredential) HostPatterns(_ string) []string {
	result := make([]string, len(vertexAIHosts))
	copy(result, vertexAIHosts)
	return result
}

// loadADC reads application_default_credentials.json from adcPath.
// Returns (nil, nil) if the file does not exist.
func loadADC(adcPath string) (*adcCredentials, error) {
	data, err := os.ReadFile(adcPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var creds adcCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing ADC JSON: %w", err)
	}
	return &creds, nil
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
