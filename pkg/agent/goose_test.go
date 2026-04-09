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

package agent

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestGoose_Name(t *testing.T) {
	t.Parallel()

	agent := NewGoose()
	if got := agent.Name(); got != "goose" {
		t.Errorf("Name() = %q, want %q", got, "goose")
	}
}

func TestGoose_SkipOnboarding_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewGoose()
	settings := make(map[string][]byte)

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	configYAML, exists := result[GooseConfigPath]
	if !exists {
		t.Fatalf("Expected %s to be created", GooseConfigPath)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(configYAML, &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if val, ok := config[gooseTelemetryKey]; !ok {
		t.Errorf("%s not set", gooseTelemetryKey)
	} else if val != false {
		t.Errorf("%s = %v, want false", gooseTelemetryKey, val)
	}
}

func TestGoose_SkipOnboarding_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	result, err := agent.SkipOnboarding(nil, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[GooseConfigPath]; !exists {
		t.Errorf("Expected %s to be created", GooseConfigPath)
	}
}

func TestGoose_SkipOnboarding_PreservesExistingTelemetryTrue(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	existingContent := []byte("GOOSE_MODEL: \"claude-sonnet-4-6\"\nGOOSE_TELEMETRY_ENABLED: true\n")
	settings := map[string][]byte{
		GooseConfigPath: existingContent,
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result[GooseConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if val, ok := config[gooseTelemetryKey]; !ok {
		t.Errorf("%s not set", gooseTelemetryKey)
	} else if val != true {
		t.Errorf("%s = %v, want true (user preference preserved)", gooseTelemetryKey, val)
	}
}

func TestGoose_SkipOnboarding_PreservesExistingTelemetryFalse(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	existingContent := []byte("GOOSE_TELEMETRY_ENABLED: false\n")
	settings := map[string][]byte{
		GooseConfigPath: existingContent,
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result[GooseConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if val, ok := config[gooseTelemetryKey]; !ok {
		t.Errorf("%s not set", gooseTelemetryKey)
	} else if val != false {
		t.Errorf("%s = %v, want false", gooseTelemetryKey, val)
	}
}

func TestGoose_SkipOnboarding_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	existingContent := []byte("GOOSE_MODEL: \"claude-sonnet-4-6\"\nGOOSE_PROVIDER: \"anthropic\"\n")
	settings := map[string][]byte{
		GooseConfigPath: existingContent,
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result[GooseConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if model, ok := config["GOOSE_MODEL"].(string); !ok || model != "claude-sonnet-4-6" {
		t.Errorf("GOOSE_MODEL = %v, want %q", config["GOOSE_MODEL"], "claude-sonnet-4-6")
	}

	if provider, ok := config["GOOSE_PROVIDER"].(string); !ok || provider != "anthropic" {
		t.Errorf("GOOSE_PROVIDER = %v, want %q", config["GOOSE_PROVIDER"], "anthropic")
	}

	if val, ok := config[gooseTelemetryKey]; !ok {
		t.Errorf("%s not set", gooseTelemetryKey)
	} else if val != false {
		t.Errorf("%s = %v, want false", gooseTelemetryKey, val)
	}
}

func TestGoose_SkipOnboarding_InvalidYAML(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	settings := map[string][]byte{
		GooseConfigPath: []byte("invalid: yaml: :::"),
	}

	_, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestGoose_SetModel_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewGoose()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	configYAML, exists := result[GooseConfigPath]
	if !exists {
		t.Fatalf("Expected %s to be created", GooseConfigPath)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(configYAML, &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if model, ok := config[gooseModelKey].(string); !ok || model != "model-from-flag" {
		t.Errorf("%s = %v, want %q", gooseModelKey, config[gooseModelKey], "model-from-flag")
	}
}

func TestGoose_SetModel_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	result, err := agent.SetModel(nil, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[GooseConfigPath]; !exists {
		t.Errorf("Expected %s to be created", GooseConfigPath)
	}
}

func TestGoose_SetModel_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	existingContent := []byte("GOOSE_TELEMETRY_ENABLED: false\nGOOSE_PROVIDER: \"anthropic\"\n")
	settings := map[string][]byte{
		GooseConfigPath: existingContent,
	}

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result[GooseConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	// Verify model was set
	if model, ok := config[gooseModelKey].(string); !ok || model != "model-from-flag" {
		t.Errorf("%s = %v, want %q", gooseModelKey, config[gooseModelKey], "model-from-flag")
	}

	// Verify existing fields are preserved
	if val, ok := config[gooseTelemetryKey]; !ok || val != false {
		t.Errorf("%s = %v, want false", gooseTelemetryKey, val)
	}

	if provider, ok := config["GOOSE_PROVIDER"].(string); !ok || provider != "anthropic" {
		t.Errorf("GOOSE_PROVIDER = %v, want %q", config["GOOSE_PROVIDER"], "anthropic")
	}
}

func TestGoose_SetModel_InvalidYAML(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	settings := map[string][]byte{
		GooseConfigPath: []byte("invalid: yaml: :::"),
	}

	_, err := agent.SetModel(settings, "model-from-flag")
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestGoose_SetModel_OverwritesExistingModel(t *testing.T) {
	t.Parallel()

	agent := NewGoose()

	existingContent := []byte("GOOSE_MODEL: \"original-model\"\nGOOSE_TELEMETRY_ENABLED: false\n")
	settings := map[string][]byte{
		GooseConfigPath: existingContent,
	}

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result[GooseConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	// Verify model was overwritten
	if model, ok := config[gooseModelKey].(string); !ok || model != "model-from-flag" {
		t.Errorf("%s = %v, want %q (should overwrite existing)", gooseModelKey, config[gooseModelKey], "model-from-flag")
	}

	// Verify other fields are preserved
	if val, ok := config[gooseTelemetryKey]; !ok || val != false {
		t.Errorf("%s = %v, want false", gooseTelemetryKey, val)
	}
}

func TestGoose_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewGoose()
	if got := agent.SkillsDir(); got != "$HOME/.agents/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.agents/skills")
	}
}
