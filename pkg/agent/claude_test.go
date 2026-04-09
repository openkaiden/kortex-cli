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

func TestClaude_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewClaude()
	if got := agent.SkillsDir(); got != "$HOME/.claude/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.claude/skills")
	}
}
