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
