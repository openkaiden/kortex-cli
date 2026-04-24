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

// Package googleauth provides utilities for detecting and using Google
// Application Default Credentials (ADC) when configuring workspaces.
package googleauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// ADCCredentials holds the fields from application_default_credentials.json
// that are needed to configure the Vertex AI app in OneCLI.
type ADCCredentials struct {
	RefreshToken   string `json:"refresh_token"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	QuotaProjectID string `json:"quota_project_id"`
}

// VertexAIFields returns the credential fields for the OneCLI vertex-ai connect API.
// If QuotaProjectID is empty in the ADC file, falls back to the
// ANTHROPIC_VERTEX_PROJECT_ID or GOOGLE_CLOUD_PROJECT env vars.
func (c *ADCCredentials) VertexAIFields() map[string]string {
	quotaProject := c.QuotaProjectID
	if quotaProject == "" {
		if p := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID"); p != "" {
			quotaProject = p
		} else {
			quotaProject = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
	}
	return map[string]string{
		"refreshToken":   c.RefreshToken,
		"clientId":       c.ClientID,
		"clientSecret":   c.ClientSecret,
		"quotaProjectId": quotaProject,
	}
}

const (
	containerHomeDir   = "/home/agent"
	containerGcloudDir = containerHomeDir + "/.config/gcloud"
	containerADCPath   = containerGcloudDir + "/application_default_credentials.json"
)

// GcloudMount records a workspace mount that targets the gcloud config directory
// or the ADC file directly, along with the resolved host path for the ADC file.
type GcloudMount struct {
	// Mount is the original workspace mount declaration.
	Mount workspace.Mount
	// ADCFilePath is the resolved host path to application_default_credentials.json.
	ADCFilePath string
}

// FindGcloudMount scans workspace mounts for one targeting the gcloud config
// directory ($HOME/.config/gcloud) or the ADC file directly. Returns nil if
// none found. Detection is based on the resolved container-side target path.
func FindGcloudMount(mounts []workspace.Mount, homeDir string) *GcloudMount {
	for _, m := range mounts {
		target := resolveGcloudTarget(m.Target)
		switch target {
		case containerGcloudDir:
			hostDir := resolveGcloudHost(m.Host, homeDir)
			return &GcloudMount{
				Mount:       m,
				ADCFilePath: filepath.Join(hostDir, "application_default_credentials.json"),
			}
		case containerADCPath:
			return &GcloudMount{
				Mount:       m,
				ADCFilePath: resolveGcloudHost(m.Host, homeDir),
			}
		}
	}
	return nil
}

// resolveGcloudHost expands $HOME in a mount host path and returns the absolute path.
// $SOURCES is not handled since gcloud config dirs are never under the source tree.
func resolveGcloudHost(host, homeDir string) string {
	if strings.HasPrefix(host, "$HOME") {
		rest := filepath.FromSlash(host[len("$HOME"):])
		return filepath.Join(homeDir, rest)
	}
	return filepath.Clean(host)
}

// resolveGcloudTarget expands $HOME in a mount target path and returns the
// absolute container-side path. Mirrors the logic in mounts.go:resolveTargetPath.
func resolveGcloudTarget(target string) string {
	if strings.HasPrefix(target, "$HOME") {
		return path.Join(containerHomeDir, target[len("$HOME"):])
	}
	return path.Clean(target)
}

// DefaultPath returns the standard host path for application default credentials.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"), nil
}

// LoadFrom reads application_default_credentials.json from the given path.
// Returns nil (no error) if the file does not exist.
func LoadFrom(adcPath string) (*ADCCredentials, error) {
	data, err := os.ReadFile(adcPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read ADC file: %w", err)
	}
	var creds ADCCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse ADC file: %w", err)
	}
	return &creds, nil
}

// Load reads application_default_credentials.json from the default location.
// Returns nil (no error) if the file does not exist.
func Load() (*ADCCredentials, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
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

// FakeADCJSON returns the bytes of a synthetic ADC file for use inside containers.
func FakeADCJSON() []byte {
	return fakeADCJSON
}

// VertexAIHosts lists the host patterns that must be excluded from network
// filtering when Vertex AI is configured. The approval handler uses glob
// syntax, so "*-aiplatform.googleapis.com" covers regional endpoints such as
// "us-central1-aiplatform.googleapis.com".
var VertexAIHosts = []string{
	"oauth2.googleapis.com",
	"aiplatform.googleapis.com",
	"*-aiplatform.googleapis.com",
}

// HostEnvVars reads Google-related env vars from the host and returns the
// workspace env vars that should be injected into the container.
//
// Mappings:
//   - CLOUD_ML_REGION      → CLOUD_ML_REGION, VERTEX_LOCATION
//   - ANTHROPIC_VERTEX_PROJECT_ID (or GOOGLE_CLOUD_PROJECT) → both vars
func HostEnvVars() map[string]string {
	vars := make(map[string]string)

	if region := os.Getenv("CLOUD_ML_REGION"); region != "" {
		vars["CLOUD_ML_REGION"] = region
		vars["VERTEX_LOCATION"] = region
	}

	projectID := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID != "" {
		vars["ANTHROPIC_VERTEX_PROJECT_ID"] = projectID
		vars["GOOGLE_CLOUD_PROJECT"] = projectID
	}

	return vars
}
