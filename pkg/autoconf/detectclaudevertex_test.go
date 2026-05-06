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
	"errors"
	"path/filepath"
	"testing"
)

// fullEnv returns a lookupEnv stub with all three required Vertex env vars set.
func fullEnv() func(string) (string, bool) {
	return func(name string) (string, bool) {
		switch name {
		case "CLAUDE_CODE_USE_VERTEX":
			return "1", true
		case "ANTHROPIC_VERTEX_PROJECT_ID":
			return "my-project", true
		case "CLOUD_ML_REGION":
			return "us-east5", true
		}
		return "", false
	}
}

// fullEnvWithGAC returns a lookupEnv stub with all three required Vertex env
// vars set plus GOOGLE_APPLICATION_CREDENTIALS set to gacPath.
// When gacPath is empty the variable is reported as absent.
func fullEnvWithGAC(gacPath string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == "GOOGLE_APPLICATION_CREDENTIALS" {
			if gacPath == "" {
				return "", false
			}
			return gacPath, true
		}
		return fullEnv()(name)
	}
}

// adcExists is a statFile stub that reports the ADC file as present.
func adcExists(_ string) error { return nil }

// adcMissing is a statFile stub that reports the ADC file as absent.
func adcMissing(_ string) error { return errors.New("file not found") }

func TestVertexDetector_AllConditionsMet(t *testing.T) {
	t.Parallel()

	d := newVertexDetectorWithInjection(fullEnv(), adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil VertexConfig, got nil")
	}
	if cfg.EnvVars["CLAUDE_CODE_USE_VERTEX"] != "1" {
		t.Errorf("expected CLAUDE_CODE_USE_VERTEX=1, got %q", cfg.EnvVars["CLAUDE_CODE_USE_VERTEX"])
	}
	if cfg.EnvVars["ANTHROPIC_VERTEX_PROJECT_ID"] != "my-project" {
		t.Errorf("expected ANTHROPIC_VERTEX_PROJECT_ID=my-project, got %q", cfg.EnvVars["ANTHROPIC_VERTEX_PROJECT_ID"])
	}
	if cfg.EnvVars["CLOUD_ML_REGION"] != "us-east5" {
		t.Errorf("expected CLOUD_ML_REGION=us-east5, got %q", cfg.EnvVars["CLOUD_ML_REGION"])
	}
	if cfg.ADCHostPath == "" {
		t.Error("expected ADCHostPath to be set")
	}
}

func TestVertexDetector_MissingClaudeCodeUseVertex(t *testing.T) {
	t.Parallel()

	lookup := func(name string) (string, bool) {
		if name == "CLAUDE_CODE_USE_VERTEX" {
			return "", false
		}
		return fullEnv()(name)
	}
	d := newVertexDetectorWithInjection(lookup, adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil VertexConfig when CLAUDE_CODE_USE_VERTEX is unset, got %+v", cfg)
	}
}

func TestVertexDetector_MissingProjectID(t *testing.T) {
	t.Parallel()

	lookup := func(name string) (string, bool) {
		if name == "ANTHROPIC_VERTEX_PROJECT_ID" {
			return "", false
		}
		return fullEnv()(name)
	}
	d := newVertexDetectorWithInjection(lookup, adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil when ANTHROPIC_VERTEX_PROJECT_ID is unset, got %+v", cfg)
	}
}

func TestVertexDetector_MissingRegion(t *testing.T) {
	t.Parallel()

	lookup := func(name string) (string, bool) {
		if name == "CLOUD_ML_REGION" {
			return "", false
		}
		return fullEnv()(name)
	}
	d := newVertexDetectorWithInjection(lookup, adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil when CLOUD_ML_REGION is unset, got %+v", cfg)
	}
}

func TestVertexDetector_ADCFileMissing(t *testing.T) {
	t.Parallel()

	d := newVertexDetectorWithInjection(fullEnv(), adcMissing, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil when ADC file is absent, got %+v", cfg)
	}
}

func TestVertexDetector_GOOGLE_APPLICATION_CREDENTIALS_UsedWhenSet(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	customCreds := filepath.Join(homeDir, "custom", "creds.json")

	d := newVertexDetectorWithInjection(fullEnvWithGAC(customCreds), adcExists, homeDir)
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config when GOOGLE_APPLICATION_CREDENTIALS is set")
	}
	want := "$HOME/custom/creds.json"
	if cfg.ADCHostPath != want {
		t.Errorf("ADCHostPath: want %q, got %q", want, cfg.ADCHostPath)
	}
}

func TestVertexDetector_GOOGLE_APPLICATION_CREDENTIALS_FileMissing(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	customCreds := filepath.Join(homeDir, "custom", "creds.json")

	d := newVertexDetectorWithInjection(fullEnvWithGAC(customCreds), adcMissing, homeDir)
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when GOOGLE_APPLICATION_CREDENTIALS file is missing")
	}
}

func TestVertexDetector_GOOGLE_APPLICATION_CREDENTIALS_EmptyFallsBackToADC(t *testing.T) {
	t.Parallel()

	// Empty GOOGLE_APPLICATION_CREDENTIALS → fall back to platform-specific ADC path.
	d := newVertexDetectorWithInjection(fullEnvWithGAC(""), adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config when falling back to default ADC path")
	}
}

func TestVertexDetector_GOOGLE_APPLICATION_CREDENTIALS_OutsideHome(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	otherDir := t.TempDir() // sibling of homeDir — not under homeDir
	customCreds := filepath.Join(otherDir, "creds.json")

	d := newVertexDetectorWithInjection(fullEnvWithGAC(customCreds), adcExists, homeDir)
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// Path is not under homeDir → returned as-is (absolute path).
	if cfg.ADCHostPath != customCreds {
		t.Errorf("ADCHostPath: want %q, got %q", customCreds, cfg.ADCHostPath)
	}
}

func TestVertexDetector_EmptyEnvVarValue(t *testing.T) {
	t.Parallel()

	lookup := func(name string) (string, bool) {
		if name == "CLAUDE_CODE_USE_VERTEX" {
			return "", true // present but empty
		}
		return fullEnv()(name)
	}
	d := newVertexDetectorWithInjection(lookup, adcExists, "/home/user")
	cfg, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil when env var is present but empty, got %+v", cfg)
	}
}
