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

func TestClaude_Name(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	if got := agent.Name(); got != "claude" {
		t.Errorf("Name() = %q, want %q", got, "claude")
	}
}

func TestClaude_SkipOnboarding_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	// Verify .claude.json was created
	claudeJSON, exists := result[ClaudeJSONPath]
	if !exists {
		t.Fatalf("Expected %s to be created", ClaudeJSONPath)
	}

	// Parse and verify content
	var config map[string]interface{}
	if err := json.Unmarshal(claudeJSON, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check hasCompletedOnboarding
	if completed, ok := config["hasCompletedOnboarding"].(bool); !ok || !completed {
		t.Errorf("hasCompletedOnboarding = %v, want true", config["hasCompletedOnboarding"])
	}

	// Check projects
	projects, ok := config["projects"].(map[string]interface{})
	if !ok {
		t.Fatalf("projects is not a map: %v", config["projects"])
	}

	projectSettings, ok := projects["/workspace/sources"].(map[string]interface{})
	if !ok {
		t.Fatalf("project settings not found for /workspace/sources")
	}

	if trust, ok := projectSettings["hasTrustDialogAccepted"].(bool); !ok || !trust {
		t.Errorf("hasTrustDialogAccepted = %v, want true", projectSettings["hasTrustDialogAccepted"])
	}
}

func TestClaude_SkipOnboarding_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	result, err := agent.SkipOnboarding(nil, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[ClaudeJSONPath]; !exists {
		t.Errorf("Expected %s to be created", ClaudeJSONPath)
	}
}

func TestClaude_SkipOnboarding_PreservesUnknownFields(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	// Create settings with extra fields
	existingSettings := map[string]interface{}{
		"hasCompletedOnboarding": false,
		"customField":            "custom value",
		"nestedObject": map[string]interface{}{
			"foo": "bar",
			"baz": 123,
		},
		"arrayField": []string{"item1", "item2"},
		"projects": map[string]interface{}{
			"/other/project": map[string]interface{}{
				"hasTrustDialogAccepted": true,
				"customProjectField":     "value",
			},
		},
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string][]byte{
		ClaudeJSONPath: existingJSON,
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	// Parse result
	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify hasCompletedOnboarding was updated
	if completed, ok := config["hasCompletedOnboarding"].(bool); !ok || !completed {
		t.Errorf("hasCompletedOnboarding = %v, want true", config["hasCompletedOnboarding"])
	}

	// Verify custom fields are preserved
	if customField, ok := config["customField"].(string); !ok || customField != "custom value" {
		t.Errorf("customField = %v, want %q", config["customField"], "custom value")
	}

	// Verify nested object is preserved
	nestedObj, ok := config["nestedObject"].(map[string]interface{})
	if !ok {
		t.Fatalf("nestedObject is not preserved")
	}
	if nestedObj["foo"] != "bar" {
		t.Errorf("nestedObject.foo = %v, want %q", nestedObj["foo"], "bar")
	}
	if baz, ok := nestedObj["baz"].(float64); !ok || baz != 123 {
		t.Errorf("nestedObject.baz = %v, want 123", nestedObj["baz"])
	}

	// Verify array is preserved
	arrayField, ok := config["arrayField"].([]interface{})
	if !ok {
		t.Fatalf("arrayField is not preserved")
	}
	if len(arrayField) != 2 || arrayField[0] != "item1" || arrayField[1] != "item2" {
		t.Errorf("arrayField = %v, want [item1, item2]", arrayField)
	}

	// Verify existing project is preserved
	projects, ok := config["projects"].(map[string]interface{})
	if !ok {
		t.Fatalf("projects is not a map")
	}

	otherProject, ok := projects["/other/project"].(map[string]interface{})
	if !ok {
		t.Fatalf("existing project /other/project was not preserved")
	}
	if otherProject["customProjectField"] != "value" {
		t.Errorf("customProjectField = %v, want %q", otherProject["customProjectField"], "value")
	}

	// Verify new project was added
	newProject, ok := projects["/workspace/sources"].(map[string]interface{})
	if !ok {
		t.Fatalf("new project /workspace/sources was not added")
	}
	if trust, ok := newProject["hasTrustDialogAccepted"].(bool); !ok || !trust {
		t.Errorf("hasTrustDialogAccepted = %v, want true", newProject["hasTrustDialogAccepted"])
	}
}

func TestClaude_SkipOnboarding_DifferentWorkspacePaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		workspacePath string
	}{
		{
			name:          "podman path",
			workspacePath: "/workspace/sources",
		},
		{
			name:          "fake runtime path",
			workspacePath: "/project/sources",
		},
		{
			name:          "custom path",
			workspacePath: "/custom/workspace/location",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			agent := NewClaude()
			settings := make(map[string][]byte)

			result, err := agent.SkipOnboarding(settings, tc.workspacePath)
			if err != nil {
				t.Fatalf("SkipOnboarding() error = %v", err)
			}

			var config map[string]interface{}
			if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			projects, ok := config["projects"].(map[string]interface{})
			if !ok {
				t.Fatalf("projects is not a map")
			}

			projectSettings, ok := projects[tc.workspacePath].(map[string]interface{})
			if !ok {
				t.Fatalf("project settings not found for %s", tc.workspacePath)
			}

			if trust, ok := projectSettings["hasTrustDialogAccepted"].(bool); !ok || !trust {
				t.Errorf("hasTrustDialogAccepted = %v, want true", projectSettings["hasTrustDialogAccepted"])
			}
		})
	}
}

func TestClaude_SkipOnboarding_UpdatesExistingProject(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	// Create settings with existing project that has hasTrustDialogAccepted: false
	existingSettings := map[string]interface{}{
		"hasCompletedOnboarding": false,
		"projects": map[string]interface{}{
			"/workspace/sources": map[string]interface{}{
				"hasTrustDialogAccepted": false,
				"otherField":             "should be preserved",
			},
		},
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string][]byte{
		ClaudeJSONPath: existingJSON,
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	projects, ok := config["projects"].(map[string]interface{})
	if !ok {
		t.Fatalf("projects is not a map")
	}

	projectSettings, ok := projects["/workspace/sources"].(map[string]interface{})
	if !ok {
		t.Fatalf("project settings not found")
	}

	// Verify trust was updated
	if trust, ok := projectSettings["hasTrustDialogAccepted"].(bool); !ok || !trust {
		t.Errorf("hasTrustDialogAccepted = %v, want true", projectSettings["hasTrustDialogAccepted"])
	}

	// Verify other fields are preserved
	if otherField, ok := projectSettings["otherField"].(string); !ok || otherField != "should be preserved" {
		t.Errorf("otherField = %v, want %q", projectSettings["otherField"], "should be preserved")
	}
}

func TestClaude_SkipOnboarding_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	settings := map[string][]byte{
		ClaudeJSONPath: []byte("invalid json {{{"),
	}

	_, err := agent.SkipOnboarding(settings, "/workspace/sources")
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestClaude_SkipOnboarding_EmptyWorkspacePath(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.SkipOnboarding(settings, "")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	projects, ok := config["projects"].(map[string]interface{})
	if !ok {
		t.Fatalf("projects is not a map")
	}

	// Even with empty path, should create entry (though it's not useful)
	projectSettings, ok := projects[""].(map[string]interface{})
	if !ok {
		t.Fatalf("project settings not found for empty path")
	}

	if trust, ok := projectSettings["hasTrustDialogAccepted"].(bool); !ok || !trust {
		t.Errorf("hasTrustDialogAccepted = %v, want true", projectSettings["hasTrustDialogAccepted"])
	}
}

func TestClaude_SetModel_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	claudeJSON, exists := result[ClaudeSettingsPath]
	if !exists {
		t.Fatalf("Expected %s to be created", ClaudeSettingsPath)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(claudeJSON, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if model, ok := config["model"].(string); !ok || model != "model-from-flag" {
		t.Errorf("model = %v, want %q", config["model"], "model-from-flag")
	}
}

func TestClaude_SetModel_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	result, err := agent.SetModel(nil, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[ClaudeSettingsPath]; !exists {
		t.Errorf("Expected %s to be created", ClaudeSettingsPath)
	}
}

func TestClaude_SetModel_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	existingSettings := map[string]interface{}{
		"customField":  "custom value",
		"anotherField": 123,
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string][]byte{
		ClaudeSettingsPath: existingJSON,
	}

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeSettingsPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify model was set
	if model, ok := config["model"].(string); !ok || model != "model-from-flag" {
		t.Errorf("model = %v, want %q", config["model"], "model-from-flag")
	}

	// Verify existing fields are preserved
	if customField, ok := config["customField"].(string); !ok || customField != "custom value" {
		t.Errorf("customField = %v, want %q", config["customField"], "custom value")
	}

	if anotherField, ok := config["anotherField"].(float64); !ok || anotherField != 123 {
		t.Errorf("anotherField = %v, want 123", config["anotherField"])
	}
}

func TestClaude_SetModel_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	settings := map[string][]byte{
		ClaudeSettingsPath: []byte("invalid json {{{"),
	}

	_, err := agent.SetModel(settings, "model-from-flag")
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestClaude_SetModel_OverwritesExistingModel(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	existingSettings := map[string]interface{}{
		"model":      "original-model",
		"otherField": true,
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string][]byte{
		ClaudeSettingsPath: existingJSON,
	}

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeSettingsPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify model was overwritten
	if model, ok := config["model"].(string); !ok || model != "model-from-flag" {
		t.Errorf("model = %v, want %q (should overwrite existing)", config["model"], "model-from-flag")
	}

	// Verify other fields are preserved
	if otherField, ok := config["otherField"].(bool); !ok || !otherField {
		t.Errorf("otherField = %v, want true", config["otherField"])
	}
}

func TestClaude_SetModel_ProviderModelFormat(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "claude::gemma2:7b")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeSettingsPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if model, ok := config["model"].(string); !ok || model != "gemma2:7b" {
		t.Errorf("model = %v, want %q", config["model"], "gemma2:7b")
	}
}

func TestClaude_SetModel_ProviderModelURLFormat(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "claude::gemma2:7b::http://localhost:11434/v1")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeSettingsPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if model, ok := config["model"].(string); !ok || model != "gemma2:7b" {
		t.Errorf("model = %v, want %q", config["model"], "gemma2:7b")
	}
}

func TestClaude_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	if got := agent.SkillsDir(); got != "$HOME/.claude/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.claude/skills")
	}
}

func TestClaude_SetMCPServers_NilMCP(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := map[string][]byte{
		ClaudeJSONPath: []byte(`{"hasCompletedOnboarding": true}`),
	}

	result, err := agent.SetMCPServers(settings, nil)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	// Settings should be returned unchanged
	if string(result[ClaudeJSONPath]) != `{"hasCompletedOnboarding": true}` {
		t.Errorf("SetMCPServers() with nil MCP modified settings unexpectedly: %s", result[ClaudeJSONPath])
	}
}

func TestClaude_SetMCPServers_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	args := []string{"-y", "@modelcontextprotocol/server-filesystem"}
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "filesystem", Command: "npx", Args: &args},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result map")
	}
	if _, exists := result[ClaudeJSONPath]; !exists {
		t.Errorf("Expected %s to be created", ClaudeJSONPath)
	}
}

func TestClaude_SetMCPServers_CommandBased(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	args := []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"}
	env := map[string]string{"NODE_ENV": "production"}
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "filesystem", Command: "npx", Args: &args, Env: &env},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers is not a map: %v", config["mcpServers"])
	}

	fsServer, ok := mcpServers["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatalf("filesystem server not found or wrong type")
	}

	if fsServer["type"] != "stdio" {
		t.Errorf("type = %v, want %q", fsServer["type"], "stdio")
	}
	if fsServer["command"] != "npx" {
		t.Errorf("command = %v, want %q", fsServer["command"], "npx")
	}

	gotArgs, ok := fsServer["args"].([]interface{})
	if !ok {
		t.Fatalf("args is not a slice: %v", fsServer["args"])
	}
	if len(gotArgs) != 3 || gotArgs[0] != "-y" {
		t.Errorf("args = %v, want %v", gotArgs, args)
	}

	gotEnv, ok := fsServer["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("env is not a map: %v", fsServer["env"])
	}
	if gotEnv["NODE_ENV"] != "production" {
		t.Errorf("env.NODE_ENV = %v, want %q", gotEnv["NODE_ENV"], "production")
	}
}

func TestClaude_SetMCPServers_URLBased(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	headers := map[string]string{"Authorization": "Bearer token123"}
	mcp := &workspace.McpConfiguration{
		Servers: &[]workspace.McpServer{
			{Name: "remote", Url: "https://example.com/sse", Headers: &headers},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers is not a map: %v", config["mcpServers"])
	}

	remoteServer, ok := mcpServers["remote"].(map[string]interface{})
	if !ok {
		t.Fatalf("remote server not found or wrong type")
	}

	if remoteServer["type"] != "sse" {
		t.Errorf("type = %v, want %q", remoteServer["type"], "sse")
	}
	if remoteServer["url"] != "https://example.com/sse" {
		t.Errorf("url = %v, want %q", remoteServer["url"], "https://example.com/sse")
	}

	gotHeaders, ok := remoteServer["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers is not a map: %v", remoteServer["headers"])
	}
	if gotHeaders["Authorization"] != "Bearer token123" {
		t.Errorf("headers.Authorization = %v, want %q", gotHeaders["Authorization"], "Bearer token123")
	}
}

func TestClaude_SetMCPServers_URLBased_NoHeaders(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	mcp := &workspace.McpConfiguration{
		Servers: &[]workspace.McpServer{
			{Name: "simple", Url: "https://example.com/sse"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers := config["mcpServers"].(map[string]interface{})
	server := mcpServers["simple"].(map[string]interface{})

	if _, hasHeaders := server["headers"]; hasHeaders {
		t.Errorf("Expected no headers field when Headers is nil, got: %v", server["headers"])
	}
}

func TestClaude_SetMCPServers_Mixed(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "local-tool", Command: "python3", Args: &[]string{"/scripts/mcp.py"}},
		},
		Servers: &[]workspace.McpServer{
			{Name: "remote-api", Url: "https://api.example.com/mcp"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers is not a map")
	}
	if _, ok := mcpServers["local-tool"]; !ok {
		t.Error("Expected local-tool server to be present")
	}
	if _, ok := mcpServers["remote-api"]; !ok {
		t.Error("Expected remote-api server to be present")
	}
}

func TestClaude_SetMCPServers_PreservesExistingMCPServers(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	// Start with an existing mcpServers entry in .claude.json
	existing := map[string]interface{}{
		"hasCompletedOnboarding": true,
		"mcpServers": map[string]interface{}{
			"existing-server": map[string]interface{}{
				"type":    "stdio",
				"command": "existing-cmd",
				"args":    []interface{}{},
				"env":     map[string]interface{}{},
			},
		},
	}
	existingJSON, _ := json.Marshal(existing)

	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "new-tool", Command: "new-cmd"},
		},
	}

	result, err := agent.SetMCPServers(map[string][]byte{ClaudeJSONPath: existingJSON}, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers is not a map")
	}

	// Both old and new servers should be present
	if _, ok := mcpServers["existing-server"]; !ok {
		t.Error("existing-server was not preserved")
	}
	if _, ok := mcpServers["new-tool"]; !ok {
		t.Error("new-tool was not added")
	}
	if config["hasCompletedOnboarding"] != true {
		t.Error("hasCompletedOnboarding was not preserved")
	}
}

func TestClaude_SetMCPServers_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	existing := map[string]interface{}{
		"model":                  "claude-opus-4-6",
		"hasCompletedOnboarding": true,
	}
	existingJSON, _ := json.Marshal(existing)

	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "tool", Command: "mytool"},
		},
	}

	result, err := agent.SetMCPServers(map[string][]byte{ClaudeJSONPath: existingJSON}, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if config["model"] != "claude-opus-4-6" {
		t.Errorf("model field was not preserved: %v", config["model"])
	}
	if config["hasCompletedOnboarding"] != true {
		t.Errorf("hasCompletedOnboarding was not preserved: %v", config["hasCompletedOnboarding"])
	}
	if _, ok := config["mcpServers"]; !ok {
		t.Error("mcpServers was not added")
	}
}

func TestClaude_SetMCPServers_CommandNoArgs(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "tool", Command: "mytool"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpServers := config["mcpServers"].(map[string]interface{})
	server := mcpServers["tool"].(map[string]interface{})

	// Args should default to empty slice, env to empty map
	args, ok := server["args"].([]interface{})
	if !ok || len(args) != 0 {
		t.Errorf("args = %v, want empty slice", server["args"])
	}
	envMap, ok := server["env"].(map[string]interface{})
	if !ok || len(envMap) != 0 {
		t.Errorf("env = %v, want empty map", server["env"])
	}
}

func TestClaude_SetMCPServers_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "tool", Command: "mytool"},
		},
	}

	_, err := agent.SetMCPServers(map[string][]byte{ClaudeJSONPath: []byte("invalid json {{{")}, mcp)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestClaude_ApprovePresetKey_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := make(map[string][]byte)

	result, err := agent.ApprovePresetKey(settings, []string{"placeholder"})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	claudeJSON, exists := result[ClaudeJSONPath]
	if !exists {
		t.Fatalf("Expected %s to be created", ClaudeJSONPath)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(claudeJSON, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	resp, ok := config["customApiKeyResponses"].(map[string]interface{})
	if !ok {
		t.Fatalf("customApiKeyResponses missing or wrong type: %v", config["customApiKeyResponses"])
	}
	approved, ok := resp["approved"].([]interface{})
	if !ok || len(approved) != 1 || approved[0] != "placeholder" {
		t.Errorf("approved = %v, want [placeholder]", resp["approved"])
	}
}

func TestClaude_ApprovePresetKey_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	result, err := agent.ApprovePresetKey(nil, []string{"placeholder"})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[ClaudeJSONPath]; !exists {
		t.Errorf("Expected %s to be created", ClaudeJSONPath)
	}
}

func TestClaude_ApprovePresetKey_EmptyKeys(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	settings := map[string][]byte{
		ClaudeJSONPath: []byte(`{"existingField": "value"}`),
	}

	result, err := agent.ApprovePresetKey(settings, []string{})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	// settings returned unchanged — no customApiKeyResponses added
	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}
	if _, ok := config["customApiKeyResponses"]; ok {
		t.Error("customApiKeyResponses should not be present when no keys provided")
	}
}

func TestClaude_ApprovePresetKey_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	existing := map[string]interface{}{
		"hasCompletedOnboarding": true,
		"someOtherField":         "keep me",
	}
	existingJSON, _ := json.Marshal(existing)

	result, err := agent.ApprovePresetKey(map[string][]byte{ClaudeJSONPath: existingJSON}, []string{"placeholder"})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if v, ok := config["hasCompletedOnboarding"].(bool); !ok || !v {
		t.Errorf("hasCompletedOnboarding = %v, want true", config["hasCompletedOnboarding"])
	}
	if v, ok := config["someOtherField"].(string); !ok || v != "keep me" {
		t.Errorf("someOtherField = %v, want %q", config["someOtherField"], "keep me")
	}
}

func TestClaude_ApprovePresetKey_MergesWithExisting(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	existing := map[string]interface{}{
		"customApiKeyResponses": map[string]interface{}{
			"approved": []interface{}{"existing-key"},
		},
	}
	existingJSON, _ := json.Marshal(existing)

	result, err := agent.ApprovePresetKey(map[string][]byte{ClaudeJSONPath: existingJSON}, []string{"placeholder"})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	resp := config["customApiKeyResponses"].(map[string]interface{})
	approved := resp["approved"].([]interface{})
	if len(approved) != 2 {
		t.Fatalf("approved len = %d, want 2: %v", len(approved), approved)
	}
	// sorted: existing-key, placeholder
	if approved[0] != "existing-key" || approved[1] != "placeholder" {
		t.Errorf("approved = %v, want [existing-key placeholder]", approved)
	}
}

func TestClaude_ApprovePresetKey_Deduplicates(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	existing := map[string]interface{}{
		"customApiKeyResponses": map[string]interface{}{
			"approved": []interface{}{"placeholder"},
		},
	}
	existingJSON, _ := json.Marshal(existing)

	result, err := agent.ApprovePresetKey(map[string][]byte{ClaudeJSONPath: existingJSON}, []string{"placeholder", "placeholder"})
	if err != nil {
		t.Fatalf("ApprovePresetKey() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClaudeJSONPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	resp := config["customApiKeyResponses"].(map[string]interface{})
	approved := resp["approved"].([]interface{})
	if len(approved) != 1 || approved[0] != "placeholder" {
		t.Errorf("approved = %v, want [placeholder]", approved)
	}
}

func TestClaude_ApprovePresetKey_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaude()

	_, err := agent.ApprovePresetKey(map[string][]byte{ClaudeJSONPath: []byte("invalid json {{{")}, []string{"placeholder"})
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
