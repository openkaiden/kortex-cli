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
	"sort"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	kdnconfig "github.com/openkaiden/kdn/pkg/config"
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

// SetMCPServers configures MCP servers in Claude settings.
// It writes MCP server entries into .claude.json under the top-level "mcpServers" key.
// Command-based servers use type "stdio" with {command, args, env}.
// URL-based servers use type "sse" with {url, headers}.
// All other fields in the settings file are preserved.
// If mcp is nil, settings are returned unchanged.
func (c *claudeAgent) SetMCPServers(settings map[string][]byte, mcp *workspace.McpConfiguration) (map[string][]byte, error) {
	if mcp == nil {
		return settings, nil
	}
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var existingContent []byte
	var exists bool
	if existingContent, exists = settings[ClaudeJSONPath]; !exists {
		existingContent = []byte("{}")
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClaudeJSONPath, err)
	}

	// Get or create the mcpServers map
	mcpServers := make(map[string]interface{})
	if raw, ok := config["mcpServers"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			mcpServers = m
		}
	}

	// Add command-based MCP servers (stdio type)
	if mcp.Commands != nil {
		for _, cmd := range *mcp.Commands {
			entry := map[string]interface{}{
				"type":    "stdio",
				"command": cmd.Command,
				"args":    []string{},
				"env":     map[string]string{},
			}
			if cmd.Args != nil {
				entry["args"] = *cmd.Args
			}
			if cmd.Env != nil {
				entry["env"] = *cmd.Env
			}
			mcpServers[cmd.Name] = entry
		}
	}

	// Add URL-based MCP servers (sse type)
	if mcp.Servers != nil {
		for _, srv := range *mcp.Servers {
			entry := map[string]interface{}{
				"type": "sse",
				"url":  srv.Url,
			}
			if srv.Headers != nil {
				entry["headers"] = *srv.Headers
			}
			mcpServers[srv.Name] = entry
		}
	}

	if len(mcpServers) > 0 {
		config["mcpServers"] = mcpServers
	}

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClaudeJSONPath, err)
	}

	settings[ClaudeJSONPath] = modifiedContent
	return settings, nil
}

// ApprovePresetKey adds the given key values to customApiKeyResponses.approved in .claude.json.
// Existing approved entries are preserved; duplicates are removed.
func (c *claudeAgent) ApprovePresetKey(settings map[string][]byte, approvedKeys []string) (map[string][]byte, error) {
	if len(approvedKeys) == 0 {
		return settings, nil
	}
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var existingContent []byte
	var exists bool
	if existingContent, exists = settings[ClaudeJSONPath]; !exists {
		existingContent = []byte("{}")
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClaudeJSONPath, err)
	}

	customApiKeyResponses := make(map[string]interface{})
	if raw, ok := config["customApiKeyResponses"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			customApiKeyResponses = m
		}
	}

	seen := make(map[string]struct{})
	if raw, ok := customApiKeyResponses["approved"]; ok {
		if slice, ok := raw.([]interface{}); ok {
			for _, v := range slice {
				if s, ok := v.(string); ok {
					seen[s] = struct{}{}
				}
			}
		}
	}
	for _, k := range approvedKeys {
		seen[k] = struct{}{}
	}

	approved := make([]string, 0, len(seen))
	for k := range seen {
		approved = append(approved, k)
	}
	sort.Strings(approved)

	customApiKeyResponses["approved"] = approved
	config["customApiKeyResponses"] = customApiKeyResponses

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClaudeJSONPath, err)
	}

	settings[ClaudeJSONPath] = modifiedContent
	return settings, nil
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

	_, modelName, _ := kdnconfig.ParseModelID(modelID)
	config["model"] = modelName

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClaudeSettingsPath, err)
	}

	settings[ClaudeSettingsPath] = modifiedContent
	return settings, nil
}
