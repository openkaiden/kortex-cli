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

package gcloud

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
)

// fakeOnecliClient is a test double for onecli.Client that records ConnectApp calls.
type fakeOnecliClient struct {
	connectAppErr      error
	connectAppProvider string
	connectAppFields   map[string]string
}

var _ onecli.Client = (*fakeOnecliClient)(nil)

func (f *fakeOnecliClient) CreateSecret(_ context.Context, _ onecli.CreateSecretInput) (*onecli.Secret, error) {
	return nil, nil
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
func (f *fakeOnecliClient) ConnectApp(_ context.Context, provider string, fields map[string]string) error {
	f.connectAppProvider = provider
	f.connectAppFields = fields
	return f.connectAppErr
}

func TestGcloudCredential_Name(t *testing.T) {
	t.Parallel()
	if got := New().Name(); got != "gcloud" {
		t.Errorf("Name() = %q, want %q", got, "gcloud")
	}
}

func TestGcloudCredential_ContainerFilePath(t *testing.T) {
	t.Parallel()
	want := "/home/agent/.config/gcloud/application_default_credentials.json"
	if got := New().ContainerFilePath(); got != want {
		t.Errorf("ContainerFilePath() = %q, want %q", got, want)
	}
}

func TestGcloudCredential_Detect(t *testing.T) {
	t.Parallel()

	homeDir := "/home/testuser"

	tests := []struct {
		name            string
		mounts          []workspace.Mount
		wantNil         bool
		wantADCFilePath string
		wantMountHost   string
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
			name: "gcloud directory mount via $HOME",
			mounts: []workspace.Mount{
				{Host: "$HOME/.config/gcloud", Target: "$HOME/.config/gcloud"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud",
		},
		{
			name: "gcloud directory mount via absolute host path",
			mounts: []workspace.Mount{
				{Host: "/home/testuser/.config/gcloud", Target: "$HOME/.config/gcloud"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join("/home/testuser/.config/gcloud", "application_default_credentials.json"),
			wantMountHost:   "/home/testuser/.config/gcloud",
		},
		{
			name: "gcloud ADC file mount via $HOME",
			mounts: []workspace.Mount{
				{Host: "$HOME/.config/gcloud/application_default_credentials.json", Target: "$HOME/.config/gcloud/application_default_credentials.json"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud/application_default_credentials.json",
		},
		{
			name: "gcloud ADC file mount via absolute container target",
			mounts: []workspace.Mount{
				{Host: "$HOME/.config/gcloud/application_default_credentials.json", Target: "/home/agent/.config/gcloud/application_default_credentials.json"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud/application_default_credentials.json",
		},
		{
			name: "gcloud directory mount via absolute container target",
			mounts: []workspace.Mount{
				{Host: "$HOME/.config/gcloud", Target: "/home/agent/.config/gcloud"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud",
		},
		{
			name: "first matching mount wins",
			mounts: []workspace.Mount{
				{Host: "$HOME/other", Target: "$HOME/other"},
				{Host: "$HOME/.config/gcloud", Target: "$HOME/.config/gcloud"},
				{Host: "$HOME/.config/gcloud/application_default_credentials.json", Target: "$HOME/.config/gcloud/application_default_credentials.json"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
			if gotPath != tt.wantADCFilePath {
				t.Errorf("Detect() hostFilePath = %q, want %q", gotPath, tt.wantADCFilePath)
			}
			if gotMount.Host != tt.wantMountHost {
				t.Errorf("Detect() intercepted.Host = %q, want %q", gotMount.Host, tt.wantMountHost)
			}
		})
	}
}

func TestGcloudCredential_FakeFile(t *testing.T) {
	t.Parallel()

	cred := New()
	content, err := cred.FakeFile("")
	if err != nil {
		t.Fatalf("FakeFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Fatal("FakeFile() returned empty content")
	}
	// Must contain placeholder values, not real credentials.
	s := string(content)
	for _, placeholder := range []string{"FAKE_CLIENT_ID", "FAKE_CLIENT_SECRET", "FAKE_REFRESH_TOKEN"} {
		if !strings.Contains(s, placeholder) {
			t.Errorf("FakeFile() content does not contain %q", placeholder)
		}
	}
}

func TestGcloudCredential_HostPatterns(t *testing.T) {
	t.Parallel()

	cred := New()
	patterns := cred.HostPatterns("")
	if len(patterns) == 0 {
		t.Fatal("HostPatterns() returned empty slice")
	}
	// Must include Google OAuth and Vertex AI endpoints.
	found := make(map[string]bool)
	for _, p := range patterns {
		found[p] = true
	}
	for _, want := range []string{"oauth2.googleapis.com", "aiplatform.googleapis.com"} {
		if !found[want] {
			t.Errorf("HostPatterns() missing %q", want)
		}
	}
}

func TestLoadADC(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for missing file", func(t *testing.T) {
		t.Parallel()

		creds, err := loadADC("/nonexistent/path/adc.json")
		if err != nil {
			t.Fatalf("loadADC() error = %v, want nil", err)
		}
		if creds != nil {
			t.Errorf("loadADC() = %+v, want nil", creds)
		}
	})

	t.Run("parses valid ADC file", func(t *testing.T) {
		t.Parallel()

		adcPath := filepath.Join(t.TempDir(), "adc.json")
		content := []byte(`{
			"refresh_token": "test-token",
			"client_id": "test-client-id",
			"client_secret": "test-secret",
			"quota_project_id": "test-project"
		}`)
		if err := os.WriteFile(adcPath, content, 0644); err != nil {
			t.Fatalf("failed to write test ADC file: %v", err)
		}

		creds, err := loadADC(adcPath)
		if err != nil {
			t.Fatalf("loadADC() error = %v", err)
		}
		if creds == nil {
			t.Fatal("loadADC() = nil, want non-nil")
		}
		if creds.RefreshToken != "test-token" {
			t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "test-token")
		}
		if creds.ClientID != "test-client-id" {
			t.Errorf("ClientID = %q, want %q", creds.ClientID, "test-client-id")
		}
		if creds.QuotaProjectID != "test-project" {
			t.Errorf("QuotaProjectID = %q, want %q", creds.QuotaProjectID, "test-project")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		adcPath := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(adcPath, []byte("not json"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := loadADC(adcPath)
		if err == nil {
			t.Fatal("loadADC() error = nil, want error for invalid JSON")
		}
	})

	t.Run("returns error for unreadable file", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("file permission bits are not enforced on Windows")
		}
		if os.Getuid() == 0 {
			t.Skip("running as root: permission checks do not apply")
		}

		adcPath := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(adcPath, []byte(`{}`), 0000); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := loadADC(adcPath)
		if err == nil {
			t.Fatal("loadADC() error = nil, want error for unreadable file")
		}
		if errors.Is(err, os.ErrNotExist) {
			t.Errorf("loadADC() returned ErrNotExist, want a permission error")
		}
	})
}

func TestVertexAIFields(t *testing.T) {
	t.Run("uses quota_project_id from ADC when present", func(t *testing.T) {
		t.Parallel()

		creds := &adcCredentials{
			RefreshToken:   "tok",
			ClientID:       "id",
			ClientSecret:   "sec",
			QuotaProjectID: "adc-project",
		}
		fields := creds.vertexAIFields()
		if fields["quotaProjectId"] != "adc-project" {
			t.Errorf("quotaProjectId = %q, want %q", fields["quotaProjectId"], "adc-project")
		}
		if fields["refreshToken"] != "tok" {
			t.Errorf("refreshToken = %q, want %q", fields["refreshToken"], "tok")
		}
	})

	t.Run("falls back to ANTHROPIC_VERTEX_PROJECT_ID when quota_project_id empty", func(t *testing.T) {
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "env-vertex-project")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "should-not-be-used")

		creds := &adcCredentials{RefreshToken: "tok"}
		fields := creds.vertexAIFields()
		if fields["quotaProjectId"] != "env-vertex-project" {
			t.Errorf("quotaProjectId = %q, want %q", fields["quotaProjectId"], "env-vertex-project")
		}
	})

	t.Run("falls back to GOOGLE_CLOUD_PROJECT when ANTHROPIC_VERTEX_PROJECT_ID unset", func(t *testing.T) {
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "gcp-project")

		creds := &adcCredentials{RefreshToken: "tok"}
		fields := creds.vertexAIFields()
		if fields["quotaProjectId"] != "gcp-project" {
			t.Errorf("quotaProjectId = %q, want %q", fields["quotaProjectId"], "gcp-project")
		}
	})

	t.Run("empty quotaProjectId when no env vars set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")

		creds := &adcCredentials{RefreshToken: "tok"}
		fields := creds.vertexAIFields()
		if fields["quotaProjectId"] != "" {
			t.Errorf("quotaProjectId = %q, want empty string", fields["quotaProjectId"])
		}
	})
}

func TestGcloudCredential_Configure(t *testing.T) {
	t.Parallel()

	writeADC := func(t *testing.T, content string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write ADC file: %v", err)
		}
		return p
	}

	validADC := `{
		"refresh_token": "real-token",
		"client_id": "real-client-id",
		"client_secret": "real-secret",
		"quota_project_id": "real-project"
	}`

	t.Run("success: calls ConnectApp with parsed fields", func(t *testing.T) {
		t.Parallel()

		adcPath := writeADC(t, validADC)
		client := &fakeOnecliClient{}
		cred := New()

		if err := cred.Configure(context.Background(), client, adcPath); err != nil {
			t.Fatalf("Configure() error = %v", err)
		}
		if client.connectAppProvider != "vertex-ai" {
			t.Errorf("ConnectApp provider = %q, want %q", client.connectAppProvider, "vertex-ai")
		}
		if client.connectAppFields["refreshToken"] != "real-token" {
			t.Errorf("refreshToken = %q, want %q", client.connectAppFields["refreshToken"], "real-token")
		}
		if client.connectAppFields["quotaProjectId"] != "real-project" {
			t.Errorf("quotaProjectId = %q, want %q", client.connectAppFields["quotaProjectId"], "real-project")
		}
	})

	t.Run("error: ADC file not found", func(t *testing.T) {
		t.Parallel()

		client := &fakeOnecliClient{}
		cred := New()

		err := cred.Configure(context.Background(), client, "/nonexistent/adc.json")
		if err == nil {
			t.Fatal("Configure() error = nil, want error for missing file")
		}
		if !strings.Contains(err.Error(), "ADC file not found") {
			t.Errorf("error %q does not mention 'ADC file not found'", err.Error())
		}
	})

	t.Run("error: invalid JSON in ADC file", func(t *testing.T) {
		t.Parallel()

		adcPath := writeADC(t, "not valid json")
		client := &fakeOnecliClient{}
		cred := New()

		err := cred.Configure(context.Background(), client, adcPath)
		if err == nil {
			t.Fatal("Configure() error = nil, want error for invalid JSON")
		}
	})

	t.Run("error: ConnectApp returns error", func(t *testing.T) {
		t.Parallel()

		adcPath := writeADC(t, validADC)
		client := &fakeOnecliClient{connectAppErr: errors.New("vertex-ai unavailable")}
		cred := New()

		err := cred.Configure(context.Background(), client, adcPath)
		if err == nil {
			t.Fatal("Configure() error = nil, want error from ConnectApp")
		}
		if !strings.Contains(err.Error(), "vertex-ai unavailable") {
			t.Errorf("error %q does not mention 'vertex-ai unavailable'", err.Error())
		}
	})
}

func TestGcloudCredential_EnvVars(t *testing.T) {
	t.Run("empty map when no env vars set", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")

		vars := New().EnvVars("")
		if len(vars) != 0 {
			t.Errorf("EnvVars() = %v, want empty map", vars)
		}
	})

	t.Run("CLOUD_ML_REGION propagates to CLOUD_ML_REGION and VERTEX_LOCATION", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "us-central1")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")

		vars := New().EnvVars("")
		if vars["CLOUD_ML_REGION"] != "us-central1" {
			t.Errorf("CLOUD_ML_REGION = %q, want %q", vars["CLOUD_ML_REGION"], "us-central1")
		}
		if vars["VERTEX_LOCATION"] != "us-central1" {
			t.Errorf("VERTEX_LOCATION = %q, want %q", vars["VERTEX_LOCATION"], "us-central1")
		}
	})

	t.Run("ANTHROPIC_VERTEX_PROJECT_ID propagates to both project vars", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-vertex-project")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")

		vars := New().EnvVars("")
		if vars["ANTHROPIC_VERTEX_PROJECT_ID"] != "my-vertex-project" {
			t.Errorf("ANTHROPIC_VERTEX_PROJECT_ID = %q, want %q", vars["ANTHROPIC_VERTEX_PROJECT_ID"], "my-vertex-project")
		}
		if vars["GOOGLE_CLOUD_PROJECT"] != "my-vertex-project" {
			t.Errorf("GOOGLE_CLOUD_PROJECT = %q, want %q", vars["GOOGLE_CLOUD_PROJECT"], "my-vertex-project")
		}
	})

	t.Run("GOOGLE_CLOUD_PROJECT used when ANTHROPIC_VERTEX_PROJECT_ID unset", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "gcp-fallback")

		vars := New().EnvVars("")
		if vars["ANTHROPIC_VERTEX_PROJECT_ID"] != "gcp-fallback" {
			t.Errorf("ANTHROPIC_VERTEX_PROJECT_ID = %q, want %q", vars["ANTHROPIC_VERTEX_PROJECT_ID"], "gcp-fallback")
		}
		if vars["GOOGLE_CLOUD_PROJECT"] != "gcp-fallback" {
			t.Errorf("GOOGLE_CLOUD_PROJECT = %q, want %q", vars["GOOGLE_CLOUD_PROJECT"], "gcp-fallback")
		}
	})

	t.Run("all env vars set produces all four output vars", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "europe-west4")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "full-project")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")

		vars := New().EnvVars("")
		for _, key := range []string{"CLOUD_ML_REGION", "VERTEX_LOCATION", "ANTHROPIC_VERTEX_PROJECT_ID", "GOOGLE_CLOUD_PROJECT"} {
			if vars[key] == "" {
				t.Errorf("EnvVars() missing or empty key %q", key)
			}
		}
		if vars["CLOUD_ML_REGION"] != "europe-west4" {
			t.Errorf("CLOUD_ML_REGION = %q, want %q", vars["CLOUD_ML_REGION"], "europe-west4")
		}
		if vars["GOOGLE_CLOUD_PROJECT"] != "full-project" {
			t.Errorf("GOOGLE_CLOUD_PROJECT = %q, want %q", vars["GOOGLE_CLOUD_PROJECT"], "full-project")
		}
	})
}
