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

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	kdnconfig "github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/containerurl"
)

const (
	// ClawConfigPath is the relative path to the OpenClaw configuration file.
	ClawConfigPath = ".openclaw/openclaw.json"
)

// clawAgent is the implementation of Agent for OpenClaw.
type clawAgent struct{}

// Compile-time check to ensure clawAgent implements Agent interface
var _ Agent = (*clawAgent)(nil)

// NewClaw creates a new OpenClaw agent implementation.
func NewClaw() Agent {
	return &clawAgent{}
}

// Name returns the agent name.
func (c *clawAgent) Name() string {
	return "openclaw"
}

// SkipOnboarding modifies OpenClaw settings to disable gateway auth and enable
// the control UI. All other fields in the settings file are preserved.
func (c *clawAgent) SkipOnboarding(settings map[string]SettingsFile, _ string, _ []string) (map[string]SettingsFile, error) {
	settings = EnsureSettings(settings)

	existingContent := GetContent(settings, ClawConfigPath, []byte("{}"))

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClawConfigPath, err)
	}

	// Get or create the gateway map
	gateway, _ := config["gateway"].(map[string]interface{})
	if gateway == nil {
		gateway = make(map[string]interface{})
	}

	// Set auth mode to "token" with a default token (gateway.auth)
	gateway["auth"] = map[string]interface{}{
		"mode":  "token",
		"token": "openclaw123",
	}

	// Enable the control UI (gateway.controlUi.enabled)
	controlUi, _ := gateway["controlUi"].(map[string]interface{})
	if controlUi == nil {
		controlUi = make(map[string]interface{})
	}
	controlUi["enabled"] = true
	gateway["controlUi"] = controlUi

	gateway["bind"] = "lan"

	config["gateway"] = gateway

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClawConfigPath, err)
	}

	SetContent(settings, ClawConfigPath, modifiedContent)
	return settings, nil
}

// SkillsDir returns the container path under which skill directories are mounted for OpenClaw.
func (c *clawAgent) SkillsDir() string {
	return "$HOME/.openclaw/skills"
}

// SetMCPServers configures MCP servers in OpenClaw settings.
// It writes MCP server entries into openclaw.json under the "mcp.servers" key.
// Command-based servers use {command, args, env} with no transport field
// (OpenClaw infers stdio from the presence of a command key).
// URL-based servers use transport "streamable-http" with {url, headers}.
// All other fields in the settings file are preserved.
// If mcp is nil, settings are returned unchanged.
func (c *clawAgent) SetMCPServers(settings map[string]SettingsFile, mcp *workspace.McpConfiguration) (map[string]SettingsFile, error) {
	if mcp == nil {
		return settings, nil
	}
	settings = EnsureSettings(settings)

	existingContent := GetContent(settings, ClawConfigPath, []byte("{}"))

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClawConfigPath, err)
	}

	// Get or create the mcp map
	mcpConfig, _ := config["mcp"].(map[string]interface{})
	if mcpConfig == nil {
		mcpConfig = make(map[string]interface{})
	}

	// Get or create the servers map
	servers, _ := mcpConfig["servers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}

	if mcp.Commands != nil {
		for _, cmd := range *mcp.Commands {
			entry := map[string]interface{}{
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
			servers[cmd.Name] = entry
		}
	}

	if mcp.Servers != nil {
		for _, srv := range *mcp.Servers {
			entry := map[string]interface{}{
				"transport": "streamable-http",
				"url":       srv.Url,
			}
			if srv.Headers != nil {
				entry["headers"] = *srv.Headers
			}
			servers[srv.Name] = entry
		}
	}

	if len(servers) > 0 {
		mcpConfig["servers"] = servers
		config["mcp"] = mcpConfig
	}

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClawConfigPath, err)
	}

	SetContent(settings, ClawConfigPath, modifiedContent)
	return settings, nil
}

// SetModel configures the model ID in OpenClaw settings.
// It sets agents.defaults.model in openclaw.json. OpenClaw uses provider/model
// format (e.g. "google/gemini-2.5-pro"). When the kdn provider::model format is
// used, it is converted to provider/model. Plain model IDs without a provider
// are passed through as-is.
// All other fields in the settings file are preserved.
func (c *clawAgent) SetModel(settings map[string]SettingsFile, modelID string) (map[string]SettingsFile, error) {
	settings = EnsureSettings(settings)

	existingContent := GetContent(settings, ClawConfigPath, []byte("{}"))

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", ClawConfigPath, err)
	}

	// Get or create the agents map
	agents, _ := config["agents"].(map[string]interface{})
	if agents == nil {
		agents = make(map[string]interface{})
	}

	// Get or create the defaults map
	defaults, _ := agents["defaults"].(map[string]interface{})
	if defaults == nil {
		defaults = make(map[string]interface{})
	}

	provider, modelName, baseURL := kdnconfig.ParseModelID(modelID)
	if provider != "" || baseURL != "" {
		if provider == "" {
			provider = "local"
		}
		defaults["model"] = provider + "/" + modelName
		if baseURL != "" {
			configureClawProvider(config, provider, modelName, containerurl.RewriteURL(baseURL))
		}
	} else {
		defaults["model"] = modelID
	}
	agents["defaults"] = defaults
	config["agents"] = agents

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", ClawConfigPath, err)
	}

	SetContent(settings, ClawConfigPath, modifiedContent)
	return settings, nil
}

func configureClawProvider(config map[string]interface{}, provider, modelName, baseURL string) {
	models, _ := config["models"].(map[string]interface{})
	if models == nil {
		models = make(map[string]interface{})
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}

	providers[provider] = map[string]interface{}{
		"baseUrl": baseURL,
		"apiKey":  "local-no-auth",
		"api":     "openai-completions",
		"models":  []map[string]interface{}{{"id": modelName, "name": modelName}},
	}
	models["providers"] = providers
	config["models"] = models
}
