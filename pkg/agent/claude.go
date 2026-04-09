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
	"fmt"
)

const (
	// ClaudeJSONPath is the relative path to the claude.json file.
	ClaudeJSONPath = ".claude.json"
	// ClaudeSettingsPath is the relative path to the Claude settings file.
	ClaudeSettingsPath = ".claude/settings.json"
)

// claudeAgent is the implementation of Agent for Claude Code.
type claudeAgent struct{}

// Compile-time check to ensure claudeAgent implements Agent interface
var _ Agent = (*claudeAgent)(nil)

// NewClaude creates a new Claude agent implementation.
func NewClaude() Agent {
	return &claudeAgent{}
}

// Name returns the agent name.
func (c *claudeAgent) Name() string {
	return "claude"
}

// SkipOnboarding modifies Claude settings to skip onboarding prompts.
// It sets hasCompletedOnboarding to true and marks the workspace sources
// directory as trusted. All other fields in the settings file are preserved.
func (c *claudeAgent) SkipOnboarding(settings map[string][]byte, workspaceSourcesPath string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var existingContent []byte
	var exists bool
	if existingContent, exists = settings[ClaudeJSONPath]; !exists {
		existingContent = []byte("{}")
	}

	// Parse into map to preserve all unknown fields
	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClaudeJSONPath, err)
	}

	// Set hasCompletedOnboarding
	config["hasCompletedOnboarding"] = true

	// Get or create projects map
	var projects map[string]interface{}
	if projectsRaw, ok := config["projects"]; ok {
		if projectsMap, ok := projectsRaw.(map[string]interface{}); ok {
			projects = projectsMap
		} else {
			projects = make(map[string]interface{})
		}
	} else {
		projects = make(map[string]interface{})
	}
	config["projects"] = projects

	// Get or create the specific project settings
	var projectSettings map[string]interface{}
	if projectRaw, ok := projects[workspaceSourcesPath]; ok {
		if projectMap, ok := projectRaw.(map[string]interface{}); ok {
			projectSettings = projectMap
		} else {
			projectSettings = make(map[string]interface{})
		}
	} else {
		projectSettings = make(map[string]interface{})
	}

	// Set hasTrustDialogAccepted while preserving other fields
	projectSettings["hasTrustDialogAccepted"] = true
	projects[workspaceSourcesPath] = projectSettings

	// Marshal final result
	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClaudeJSONPath, err)
	}

	settings[ClaudeJSONPath] = modifiedContent
	return settings, nil
}

// SkillsDir returns the container path under which skill directories are mounted for Claude Code.
func (c *claudeAgent) SkillsDir() string {
	return "$HOME/.claude/skills"
}

// SetModel configures the model ID in Claude settings.
// It sets the model field in .claude/settings.json.
// All other fields in the settings file are preserved.
func (c *claudeAgent) SetModel(settings map[string][]byte, modelID string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var existingContent []byte
	var exists bool
	if existingContent, exists = settings[ClaudeSettingsPath]; !exists {
		existingContent = []byte("{}")
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClaudeSettingsPath, err)
	}

	config["model"] = modelID

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClaudeSettingsPath, err)
	}

	settings[ClaudeSettingsPath] = modifiedContent
	return settings, nil
}
