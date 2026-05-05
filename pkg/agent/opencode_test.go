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
	"encoding/json"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestOpenCode_Name(t *testing.T) {
	t.Parallel()

	agent := NewOpenCode()
	if got := agent.Name(); got != "opencode" {
		t.Errorf("Name() = %q, want %q", got, "opencode")
	}
}

func TestOpenCode_SkipOnboarding(t *testing.T) {
	t.Parallel()

	t.Run("no existing settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SkipOnboarding(settings, "/workspace/sources")
		if err != nil {
			t.Fatalf("SkipOnboarding() error = %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result map")
		}
	})

	t.Run("nil settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		result, err := agent.SkipOnboarding(nil, "/workspace/sources")
		if err != nil {
			t.Fatalf("SkipOnboarding() error = %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result map")
		}
	})

	t.Run("preserves existing settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		existingSettings := map[string][]byte{
			"some/other/file": []byte("existing content"),
		}

		result, err := agent.SkipOnboarding(existingSettings, "/workspace/sources")
		if err != nil {
			t.Fatalf("SkipOnboarding() error = %v", err)
		}

		if string(result["some/other/file"]) != "existing content" {
			t.Errorf("Existing settings were not preserved")
		}
	})
}

func TestOpenCode_SetModel(t *testing.T) {
	t.Parallel()

	t.Run("no existing settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "anthropic/claude-sonnet-4-5")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result map")
		}

		configJSON, exists := result[OpenCodeConfigPath]
		if !exists {
			t.Fatalf("Expected %s to be created", OpenCodeConfigPath)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "anthropic/claude-sonnet-4-5" {
			t.Errorf("model = %v, want %q", config["model"], "anthropic/claude-sonnet-4-5")
		}
	})

	t.Run("nil settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		result, err := agent.SetModel(nil, "some-model-id")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result map")
		}

		if _, exists := result[OpenCodeConfigPath]; !exists {
			t.Errorf("Expected %s to be created", OpenCodeConfigPath)
		}
	})

	t.Run("preserves existing settings", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		existingSettings := map[string][]byte{
			"some/other/file": []byte("existing content"),
		}

		result, err := agent.SetModel(existingSettings, "some-model-id")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		if string(result["some/other/file"]) != "existing content" {
			t.Errorf("Existing settings were not preserved")
		}

		if _, exists := result[OpenCodeConfigPath]; !exists {
			t.Errorf("Expected %s to be created", OpenCodeConfigPath)
		}
	})

	t.Run("preserves existing config fields", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		existingConfig := map[string]interface{}{
			"someOtherField": "some-value",
			"anotherField":   123,
		}
		existingJSON, _ := json.Marshal(existingConfig)

		settings := map[string][]byte{
			OpenCodeConfigPath: existingJSON,
		}

		result, err := agent.SetModel(settings, "new-model-id")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["someOtherField"] != "some-value" {
			t.Errorf("someOtherField = %v, want %q", config["someOtherField"], "some-value")
		}
		if config["anotherField"] != float64(123) {
			t.Errorf("anotherField = %v, want 123", config["anotherField"])
		}
		if config["model"] != "new-model-id" {
			t.Errorf("model = %v, want %q", config["model"], "new-model-id")
		}
	})

	t.Run("overwrites existing model", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		existingConfig := map[string]interface{}{
			"model": "old-model",
		}
		existingJSON, _ := json.Marshal(existingConfig)

		settings := map[string][]byte{
			OpenCodeConfigPath: existingJSON,
		}

		result, err := agent.SetModel(settings, "new-model-id")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "new-model-id" {
			t.Errorf("model = %v, want %q (should overwrite existing)", config["model"], "new-model-id")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()

		settings := map[string][]byte{
			OpenCodeConfigPath: []byte("invalid json"),
		}

		_, err := agent.SetModel(settings, "some-model-id")
		if err == nil {
			t.Fatal("Expected error for invalid JSON")
		}
	})

	t.Run("provider::model configures provider with default URL", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "ollama::gemma4:26b")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "ollama/gemma4:26b" {
			t.Errorf("model = %v, want %q", config["model"], "ollama/gemma4:26b")
		}

		providers := config["provider"].(map[string]interface{})
		ollama := providers["ollama"].(map[string]interface{})
		options := ollama["options"].(map[string]interface{})

		if got := options["baseURL"].(string); got != "http://host.containers.internal:11434/v1" {
			t.Errorf("baseURL = %q, want default ollama URL", got)
		}

		models := ollama["models"].(map[string]interface{})
		modelEntry := models["gemma4:26b"].(map[string]interface{})
		if name := modelEntry["name"].(string); name != "gemma4:26b" {
			t.Errorf("model name = %q, want %q", name, "gemma4:26b")
		}
		if launch := modelEntry["_launch"].(bool); !launch {
			t.Errorf("_launch = %v, want true", launch)
		}
	})

	t.Run("provider::model::baseURL configures provider with custom URL", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "ollama::gemma4:26b::http://192.168.1.50:11434/v1")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "ollama/gemma4:26b" {
			t.Errorf("model = %v, want %q", config["model"], "ollama/gemma4:26b")
		}

		providers := config["provider"].(map[string]interface{})
		ollama := providers["ollama"].(map[string]interface{})
		options := ollama["options"].(map[string]interface{})

		if got := options["baseURL"].(string); got != "http://192.168.1.50:11434/v1" {
			t.Errorf("baseURL = %q, want %q", got, "http://192.168.1.50:11434/v1")
		}
	})

	t.Run("provider::model::localhost URL converted to container host", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "ollama::gemma4:26b::http://localhost:11434/v1")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		providers := config["provider"].(map[string]interface{})
		ollama := providers["ollama"].(map[string]interface{})
		options := ollama["options"].(map[string]interface{})

		if got := options["baseURL"].(string); got != "http://host.containers.internal:11434/v1" {
			t.Errorf("baseURL = %q, want %q", got, "http://host.containers.internal:11434/v1")
		}
	})

	t.Run("ramalama provider with default URL", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "ramalama::granite3.3:8b")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "ramalama/granite3.3:8b" {
			t.Errorf("model = %v, want %q", config["model"], "ramalama/granite3.3:8b")
		}

		providers := config["provider"].(map[string]interface{})
		ramalama := providers["ramalama"].(map[string]interface{})
		options := ramalama["options"].(map[string]interface{})

		if got := options["baseURL"].(string); got != "http://host.containers.internal:8080/v1" {
			t.Errorf("baseURL = %q, want default ramalama URL", got)
		}
	})

	t.Run("provider with empty model name returns error", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		_, err := agent.SetModel(settings, "ollama::")
		if err == nil {
			t.Fatal("Expected error for empty model name")
		}
	})

	t.Run("unknown provider without baseURL returns error", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		_, err := agent.SetModel(settings, "unknown::some-model")
		if err == nil {
			t.Fatal("Expected error for unknown provider without baseURL")
		}
	})

	t.Run("unknown provider with baseURL succeeds", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "custom::my-model::http://my-server:9090/v1")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "custom/my-model" {
			t.Errorf("model = %v, want %q", config["model"], "custom/my-model")
		}

		providers := config["provider"].(map[string]interface{})
		custom := providers["custom"].(map[string]interface{})
		options := custom["options"].(map[string]interface{})

		if got := options["baseURL"].(string); got != "http://my-server:9090/v1" {
			t.Errorf("baseURL = %q, want %q", got, "http://my-server:9090/v1")
		}
	})

	t.Run("plain model ID without provider", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := make(map[string][]byte)

		result, err := agent.SetModel(settings, "anthropic/claude-sonnet-4-6")
		if err != nil {
			t.Fatalf("SetModel() error = %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(result[OpenCodeConfigPath], &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if config["model"] != "anthropic/claude-sonnet-4-6" {
			t.Errorf("model = %v, want %q", config["model"], "anthropic/claude-sonnet-4-6")
		}

		if _, ok := config["provider"]; ok {
			t.Error("Plain model ID should not create provider block")
		}
	})
}

func TestOpenCode_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewOpenCode()
	if got := agent.SkillsDir(); got != "$HOME/.opencode/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.opencode/skills")
	}
}

func TestOpenCode_SetMCPServers(t *testing.T) {
	t.Parallel()

	t.Run("nil MCP returns settings unchanged", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := map[string][]byte{
			OpenCodeConfigPath: []byte(`{"model":"some-model"}`),
		}

		result, err := agent.SetMCPServers(settings, nil)
		if err != nil {
			t.Fatalf("SetMCPServers() error = %v", err)
		}

		if string(result[OpenCodeConfigPath]) != `{"model":"some-model"}` {
			t.Errorf("SetMCPServers() with nil MCP modified settings unexpectedly: %s", result[OpenCodeConfigPath])
		}
	})

	t.Run("nil settings returns nil", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		mcp := &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "test", Command: "npx", Args: &[]string{"-y", "test-server"}},
			},
		}

		result, err := agent.SetMCPServers(nil, mcp)
		if err != nil {
			t.Fatalf("SetMCPServers() error = %v", err)
		}

		if result != nil {
			t.Errorf("SetMCPServers() with nil settings should return nil, got %v", result)
		}
	})

	t.Run("non-nil MCP returns settings unchanged", func(t *testing.T) {
		t.Parallel()

		agent := NewOpenCode()
		settings := map[string][]byte{
			OpenCodeConfigPath: []byte(`{"model":"some-model"}`),
			"some/other/file":  []byte("existing content"),
		}
		mcp := &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "test", Command: "npx", Args: &[]string{"-y", "test-server"}},
			},
		}

		result, err := agent.SetMCPServers(settings, mcp)
		if err != nil {
			t.Fatalf("SetMCPServers() error = %v", err)
		}

		if string(result[OpenCodeConfigPath]) != `{"model":"some-model"}` {
			t.Errorf("SetMCPServers() modified config unexpectedly: %s", result[OpenCodeConfigPath])
		}
		if string(result["some/other/file"]) != "existing content" {
			t.Errorf("SetMCPServers() modified other settings unexpectedly")
		}
	})
}
