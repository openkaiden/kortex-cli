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

package autoconf

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// vertexEnvVars lists the environment variables required for Vertex AI configuration,
// in the order they will be written to config files.
var vertexEnvVars = []string{
	"CLAUDE_CODE_USE_VERTEX",
	"ANTHROPIC_VERTEX_PROJECT_ID",
	"CLOUD_ML_REGION",
}

// ADCContainerPath is the target path inside the workspace container for the
// ADC file mount, expressed with the $HOME variable that the runtime expands.
const ADCContainerPath = "$HOME/.config/gcloud/application_default_credentials.json"

// VertexConfig holds the Vertex AI environment variables detected from the environment.
// A non-nil value means all required env vars are set and the ADC file exists.
type VertexConfig struct {
	// EnvVars maps env var names to their values for the vars that were detected.
	EnvVars map[string]string
	// ADCHostPath is the host-side path to use in the workspace mount entry.
	// It always starts with $HOME. On Linux/macOS this equals ADCContainerPath;
	// on Windows it is computed from %APPDATA% relative to the home directory.
	ADCHostPath string
}

// VertexDetector detects Vertex AI configuration in the environment.
// Detect takes no parameters — data sources are baked in at construction time,
// consistent with the SecretDetector pattern.
type VertexDetector interface {
	Detect() (*VertexConfig, error)
}

// envVertexDetector is the default implementation, reading from os.LookupEnv
// and checking for the ADC file via os.Stat.
type envVertexDetector struct {
	lookupEnv func(string) (string, bool)
	statFile  func(string) error
	homeDir   string
}

var _ VertexDetector = (*envVertexDetector)(nil)

// NewVertexDetector returns a VertexDetector that reads from the process environment.
// Returns an error only if the home directory cannot be determined.
func NewVertexDetector() (VertexDetector, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	return &envVertexDetector{
		lookupEnv: os.LookupEnv,
		statFile:  func(p string) error { _, err := os.Stat(p); return err },
		homeDir:   homeDir,
	}, nil
}

// newVertexDetectorWithInjection creates an envVertexDetector with injectable
// lookupEnv and statFile functions. Used in tests.
func newVertexDetectorWithInjection(lookupEnv func(string) (string, bool), statFile func(string) error, homeDir string) VertexDetector {
	return &envVertexDetector{
		lookupEnv: lookupEnv,
		statFile:  statFile,
		homeDir:   homeDir,
	}
}

// Detect returns a non-nil VertexConfig when all three required environment
// variables (CLAUDE_CODE_USE_VERTEX, ANTHROPIC_VERTEX_PROJECT_ID, CLOUD_ML_REGION)
// are set to non-empty values and the credentials file exists on disk.
// The credentials file is looked up via GOOGLE_APPLICATION_CREDENTIALS first;
// if that variable is absent or empty the platform-specific ADC default path
// is used instead. Returns (nil, nil) when any condition is not met.
func (d *envVertexDetector) Detect() (*VertexConfig, error) {
	envVarValues := make(map[string]string, len(vertexEnvVars))
	for _, name := range vertexEnvVars {
		val, ok := d.lookupEnv(name)
		if !ok || val == "" {
			return nil, nil
		}
		envVarValues[name] = val
	}

	var detectPath, hostPath string
	if gac, ok := d.lookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok && gac != "" {
		detectPath = gac
		hostPath = credHostPath(gac, d.homeDir)
	} else {
		detectPath = adcDetectPath(d.homeDir)
		if detectPath == "" {
			return nil, nil
		}
		hostPath = adcConfigHostPath(d.homeDir)
		if hostPath == "" {
			return nil, nil
		}
	}

	if err := d.statFile(detectPath); err != nil {
		return nil, nil
	}

	return &VertexConfig{EnvVars: envVarValues, ADCHostPath: hostPath}, nil
}

// credHostPath converts an absolute credentials file path to the form used in
// workspace mount entries. When p is under homeDir it is expressed as
// $HOME/<rel> so the mount works on any machine; otherwise p is returned as-is.
func credHostPath(p, homeDir string) string {
	if homeDir == "" {
		return p
	}
	rel, err := filepath.Rel(homeDir, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return path.Join("$HOME", filepath.ToSlash(rel))
}
