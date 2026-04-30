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
	"time"
)

func TestCursor_Name(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	if got := agent.Name(); got != "cursor" {
		t.Errorf("Name() = %q, want %q", got, "cursor")
	}
}

func TestCursor_SkipOnboarding_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	settings := make(map[string][]byte)

	before := time.Now().UTC()
	result, err := agent.SkipOnboarding(settings, "/workspace/sources")
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	expectedPath := ".cursor/projects/workspace-sources/.workspace-trusted"
	trustedFile, exists := result[expectedPath]
	if !exists {
		t.Fatalf("Expected %s to be created", expectedPath)
	}

	var content map[string]interface{}
	if err := json.Unmarshal(trustedFile, &content); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if content["workspacePath"] != "/workspace/sources" {
		t.Errorf("workspacePath = %v, want %q", content["workspacePath"], "/workspace/sources")
	}

	trustedAtStr, ok := content["trustedAt"].(string)
	if !ok {
		t.Fatalf("trustedAt is not a string: %v", content["trustedAt"])
	}
	trustedAt, err := time.Parse(time.RFC3339Nano, trustedAtStr)
	if err != nil {
		t.Fatalf("Failed to parse trustedAt: %v", err)
	}
	if trustedAt.Before(before) || trustedAt.After(after) {
		t.Errorf("trustedAt = %v, expected between %v and %v", trustedAt, before, after)
	}
}

func TestCursor_SkipOnboarding_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

	result, err := agent.SkipOnboarding(nil, "/workspace/sources")
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	expectedPath := ".cursor/projects/workspace-sources/.workspace-trusted"
	if _, exists := result[expectedPath]; !exists {
		t.Errorf("Expected %s to be created", expectedPath)
	}
}

func TestCursor_SkipOnboarding_DifferentWorkspacePaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		workspacePath     string
		expectedCursorDir string
	}{
		{
			name:              "podman path",
			workspacePath:     "/workspace/sources",
			expectedCursorDir: "workspace-sources",
		},
		{
			name:              "fake runtime path",
			workspacePath:     "/project/sources",
			expectedCursorDir: "project-sources",
		},
		{
			name:              "custom path",
			workspacePath:     "/custom/workspace/location",
			expectedCursorDir: "custom-workspace-location",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			agent := NewCursor()
			settings := make(map[string][]byte)

			result, err := agent.SkipOnboarding(settings, tc.workspacePath)
			if err != nil {
				t.Fatalf("SkipOnboarding() error = %v", err)
			}

			expectedPath := ".cursor/projects/" + tc.expectedCursorDir + "/.workspace-trusted"
			trustedFile, exists := result[expectedPath]
			if !exists {
				t.Fatalf("Expected %s to be created", expectedPath)
			}

			var content map[string]interface{}
			if err := json.Unmarshal(trustedFile, &content); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}

			if content["workspacePath"] != tc.workspacePath {
				t.Errorf("workspacePath = %v, want %q", content["workspacePath"], tc.workspacePath)
			}
		})
	}
}

func TestCursor_SkipOnboarding_PreservesExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

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

	expectedPath := ".cursor/projects/workspace-sources/.workspace-trusted"
	if _, exists := result[expectedPath]; !exists {
		t.Errorf("Expected %s to be created", expectedPath)
	}
}

func TestWorkspacePathToCursorDir(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard podman path",
			input:    "/workspace/sources",
			expected: "workspace-sources",
		},
		{
			name:     "project sources path",
			input:    "/project/sources",
			expected: "project-sources",
		},
		{
			name:     "deeply nested path",
			input:    "/a/b/c/d",
			expected: "a-b-c-d",
		},
		{
			name:     "single segment path",
			input:    "/workspace",
			expected: "workspace",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := workspacePathToCursorDir(tc.input)
			if got != tc.expected {
				t.Errorf("workspacePathToCursorDir(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCursor_SetModel_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "model-from-flag")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	cliConfig, exists := result[CursorCLIConfigPath]
	if !exists {
		t.Fatalf("Expected %s to be created", CursorCLIConfigPath)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(cliConfig, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	modelObj, ok := config["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("model is not an object: %v", config["model"])
	}

	if modelObj["modelId"] != "model-from-flag" {
		t.Errorf("modelId = %v, want %q", modelObj["modelId"], "model-from-flag")
	}
	if modelObj["displayModelId"] != "model-from-flag" {
		t.Errorf("displayModelId = %v, want %q", modelObj["displayModelId"], "model-from-flag")
	}
	if modelObj["displayName"] != "model-from-flag" {
		t.Errorf("displayName = %v, want %q", modelObj["displayName"], "model-from-flag")
	}
	if modelObj["displayNameShort"] != "model-from-flag" {
		t.Errorf("displayNameShort = %v, want %q", modelObj["displayNameShort"], "model-from-flag")
	}
	if modelObj["maxMode"] != false {
		t.Errorf("maxMode = %v, want false", modelObj["maxMode"])
	}

	if config["hasChangedDefaultModel"] != true {
		t.Errorf("hasChangedDefaultModel = %v, want true", config["hasChangedDefaultModel"])
	}
}

func TestCursor_SetModel_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

	result, err := agent.SetModel(nil, "some-model-id")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[CursorCLIConfigPath]; !exists {
		t.Errorf("Expected %s to be created", CursorCLIConfigPath)
	}
}

func TestCursor_SetModel_PreservesExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

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

	if _, exists := result[CursorCLIConfigPath]; !exists {
		t.Errorf("Expected %s to be created", CursorCLIConfigPath)
	}
}

func TestCursor_SetModel_PreservesExistingCLIConfig(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

	existingConfig := map[string]interface{}{
		"someOtherField": "some-value",
		"anotherField":   123,
	}
	existingJSON, _ := json.Marshal(existingConfig)

	settings := map[string][]byte{
		CursorCLIConfigPath: existingJSON,
	}

	result, err := agent.SetModel(settings, "new-model-id")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[CursorCLIConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if config["someOtherField"] != "some-value" {
		t.Errorf("someOtherField = %v, want %q", config["someOtherField"], "some-value")
	}
	if config["anotherField"] != float64(123) {
		t.Errorf("anotherField = %v, want 123", config["anotherField"])
	}

	modelObj, ok := config["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("model is not an object: %v", config["model"])
	}
	if modelObj["modelId"] != "new-model-id" {
		t.Errorf("modelId = %v, want %q", modelObj["modelId"], "new-model-id")
	}
}

func TestCursor_SetModel_OverwritesExistingModel(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

	existingConfig := map[string]interface{}{
		"model": map[string]interface{}{
			"modelId":          "old-model",
			"displayModelId":   "old-model",
			"displayName":      "old-model",
			"displayNameShort": "old-model",
			"maxMode":          true,
		},
	}
	existingJSON, _ := json.Marshal(existingConfig)

	settings := map[string][]byte{
		CursorCLIConfigPath: existingJSON,
	}

	result, err := agent.SetModel(settings, "new-model-id")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[CursorCLIConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	modelObj, ok := config["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("model is not an object: %v", config["model"])
	}

	if modelObj["modelId"] != "new-model-id" {
		t.Errorf("modelId = %v, want %q", modelObj["modelId"], "new-model-id")
	}
	if modelObj["maxMode"] != false {
		t.Errorf("maxMode = %v, want false (should be overwritten)", modelObj["maxMode"])
	}
}

func TestCursor_SetModel_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewCursor()

	settings := map[string][]byte{
		CursorCLIConfigPath: []byte("invalid json"),
	}

	_, err := agent.SetModel(settings, "some-model-id")
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestCursor_SetModel_ProviderModelFormat(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "cursor::gemma2:7b")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[CursorCLIConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	modelObj, ok := config["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("model is not an object: %v", config["model"])
	}

	if modelObj["modelId"] != "gemma2:7b" {
		t.Errorf("modelId = %v, want %q", modelObj["modelId"], "gemma2:7b")
	}
}

func TestCursor_SetModel_ProviderModelURLFormat(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	settings := make(map[string][]byte)

	result, err := agent.SetModel(settings, "cursor::gemma2:7b::http://localhost:11434/v1")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[CursorCLIConfigPath], &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	modelObj, ok := config["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("model is not an object: %v", config["model"])
	}

	if modelObj["modelId"] != "gemma2:7b" {
		t.Errorf("modelId = %v, want %q", modelObj["modelId"], "gemma2:7b")
	}
}

func TestCursor_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewCursor()
	if got := agent.SkillsDir(); got != "$HOME/.cursor/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.cursor/skills")
	}
}
