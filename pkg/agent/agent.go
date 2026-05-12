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

// SettingsFile represents an agent settings file with its content and metadata.
type SettingsFile struct {
	Content    []byte
	Executable bool
}

// GetContent returns the content bytes for a settings file path, or defaultContent
// if the path does not exist.
func GetContent(settings map[string]SettingsFile, path string, defaultContent []byte) []byte {
	if sf, ok := settings[path]; ok {
		return sf.Content
	}
	return defaultContent
}

// SetContent updates or creates a settings file entry, preserving the Executable
// flag if the entry already exists. Returns the (possibly initialized) settings map.
func SetContent(settings map[string]SettingsFile, path string, content []byte) map[string]SettingsFile {
	if settings == nil {
		settings = make(map[string]SettingsFile)
	}
	if sf, ok := settings[path]; ok {
		sf.Content = content
		settings[path] = sf
	} else {
		settings[path] = SettingsFile{Content: content}
	}
	return settings
}

// EnsureSettings returns the settings map, creating one if nil.
func EnsureSettings(settings map[string]SettingsFile) map[string]SettingsFile {
	if settings == nil {
		return make(map[string]SettingsFile)
	}
	return settings
}

// Agent is an interface for agent-specific configuration and setup operations.
type Agent interface {
	// Name returns the agent name (e.g., "claude", "goose").
	Name() string
	// SkipOnboarding modifies agent settings to skip onboarding prompts.
	// It takes the current agent settings map (path -> SettingsFile), the workspace
	// sources path inside the container, and an optional list of API key values
	// to pre-approve so the agent does not prompt the user about them.
	// Returns the modified settings map, or an error if modification fails.
	SkipOnboarding(settings map[string]SettingsFile, workspaceSourcesPath string, approvedKeys []string) (map[string]SettingsFile, error)
	// SetModel configures the model ID in the agent settings.
	// It takes the current agent settings map (path -> SettingsFile), the model ID,
	// and the hostname used to reach the host machine from inside the runtime
	// environment (e.g. "host.containers.internal" for Podman, "host.openshell.internal"
	// for OpenShell). Implementations should use containerHost when rewriting localhost
	// URLs in model base URLs.
	// If the agent does not support model configuration, settings are returned unchanged.
	// Returns the modified settings map, or an error if modification fails.
	SetModel(settings map[string]SettingsFile, modelID string, containerHost string) (map[string]SettingsFile, error)
	// SkillsDir returns the container path (using $HOME variable) under which skill
	// directories should be mounted (e.g., "$HOME/.claude/skills" for Claude Code).
	// Returns "" if the agent does not support skills mounting.
	SkillsDir() string
	// SetMCPServers configures MCP servers in the agent settings.
	// It takes the current agent settings map (path -> SettingsFile) and the MCP configuration,
	// and returns the modified settings with MCP servers configured.
	// If the agent does not support MCP configuration, settings are returned unchanged.
	// If mcp is nil, settings are returned unchanged.
	// Returns the modified settings map, or an error if modification fails.
	SetMCPServers(settings map[string]SettingsFile, mcp *workspace.McpConfiguration) (map[string]SettingsFile, error)
}

// PortProvider is an optional interface that agents can implement to declare
// container ports that should be automatically forwarded. Agents without port
// requirements do not need to implement this interface.
type PortProvider interface {
	DefaultPorts() []int
}
