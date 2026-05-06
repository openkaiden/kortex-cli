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
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// Agent is an interface for agent-specific configuration and setup operations.
type Agent interface {
	// Name returns the agent name (e.g., "claude", "goose").
	Name() string
	// SkipOnboarding modifies agent settings to skip onboarding prompts.
	// It takes the current agent settings map (path -> content) and the workspace
	// sources path inside the container, and returns the modified settings with
	// onboarding flags set appropriately.
	// Returns the modified settings map, or an error if modification fails.
	SkipOnboarding(settings map[string][]byte, workspaceSourcesPath string) (map[string][]byte, error)
	// SetModel configures the model ID in the agent settings.
	// It takes the current agent settings map (path -> content) and the model ID,
	// and returns the modified settings with the model configured.
	// If the agent does not support model configuration, settings are returned unchanged.
	// Returns the modified settings map, or an error if modification fails.
	SetModel(settings map[string][]byte, modelID string) (map[string][]byte, error)
	// SkillsDir returns the container path (using $HOME variable) under which skill
	// directories should be mounted (e.g., "$HOME/.claude/skills" for Claude Code).
	// Returns "" if the agent does not support skills mounting.
	SkillsDir() string
	// SetMCPServers configures MCP servers in the agent settings.
	// It takes the current agent settings map (path -> content) and the MCP configuration,
	// and returns the modified settings with MCP servers configured.
	// If the agent does not support MCP configuration, settings are returned unchanged.
	// If mcp is nil, settings are returned unchanged.
	// Returns the modified settings map, or an error if modification fails.
	SetMCPServers(settings map[string][]byte, mcp *workspace.McpConfiguration) (map[string][]byte, error)
	// ApprovePresetKey adds the given key values to the agent's approved preset API keys list.
	// It takes the current agent settings map (path -> content) and a list of key values to approve,
	// and returns the modified settings with the keys added to the approved list.
	// If the agent does not support preset key approval, settings are returned unchanged.
	// If approvedKeys is empty, settings are returned unchanged.
	// Returns the modified settings map, or an error if modification fails.
	ApprovePresetKey(settings map[string][]byte, approvedKeys []string) (map[string][]byte, error)
}
