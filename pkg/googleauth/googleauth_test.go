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

package googleauth

import (
	"os"
	"path/filepath"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestFindGcloudMount(t *testing.T) {
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
			name: "gcloud ADC file mount via absolute target",
			mounts: []workspace.Mount{
				{Host: "$HOME/.config/gcloud/application_default_credentials.json", Target: "/home/agent/.config/gcloud/application_default_credentials.json"},
			},
			wantNil:         false,
			wantADCFilePath: filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
			wantMountHost:   "$HOME/.config/gcloud/application_default_credentials.json",
		},
		{
			name: "gcloud directory mount via absolute target",
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

			got := FindGcloudMount(tt.mounts, homeDir)

			if tt.wantNil {
				if got != nil {
					t.Errorf("FindGcloudMount() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("FindGcloudMount() = nil, want non-nil")
			}
			if got.ADCFilePath != tt.wantADCFilePath {
				t.Errorf("ADCFilePath = %q, want %q", got.ADCFilePath, tt.wantADCFilePath)
			}
			if got.Mount.Host != tt.wantMountHost {
				t.Errorf("Mount.Host = %q, want %q", got.Mount.Host, tt.wantMountHost)
			}
		})
	}
}

func TestLoadFrom(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for missing file", func(t *testing.T) {
		t.Parallel()

		creds, err := LoadFrom("/nonexistent/path/adc.json")
		if err != nil {
			t.Fatalf("LoadFrom() error = %v, want nil", err)
		}
		if creds != nil {
			t.Errorf("LoadFrom() = %+v, want nil", creds)
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

		creds, err := LoadFrom(adcPath)
		if err != nil {
			t.Fatalf("LoadFrom() error = %v", err)
		}
		if creds == nil {
			t.Fatal("LoadFrom() = nil, want non-nil")
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

		_, err := LoadFrom(adcPath)
		if err == nil {
			t.Fatal("LoadFrom() error = nil, want error for invalid JSON")
		}
	})
}
