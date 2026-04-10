// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

const (
	// AgentsConfigFile is the name of the agents configuration file
	AgentsConfigFile = "agents.json"
)

var (
	// ErrInvalidAgentConfig is returned when the agent configuration is invalid
	ErrInvalidAgentConfig = errors.New("invalid agent configuration")
)

// AgentConfigLoader loads agent-specific workspace configurations
type AgentConfigLoader interface {
	// Load reads and returns the workspace configuration for the specified agent.
	// Returns an empty configuration (not an error) if the agents.json file doesn't exist.
	// Returns an error if the file exists but is invalid JSON or malformed.
	Load(agentName string) (*workspace.WorkspaceConfiguration, error)
}

// agentConfigLoader is the internal implementation
type agentConfigLoader struct {
	storageDir string
}

// Compile-time check to ensure agentConfigLoader implements AgentConfigLoader interface
var _ AgentConfigLoader = (*agentConfigLoader)(nil)

// NewAgentConfigLoader creates a new agent configuration loader
func NewAgentConfigLoader(storageDir string) (AgentConfigLoader, error) {
	if storageDir == "" {
		return nil, ErrInvalidPath
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, err
	}

	return &agentConfigLoader{
		storageDir: absPath,
	}, nil
}

// Load reads and returns the workspace configuration for the specified agent
func (a *agentConfigLoader) Load(agentName string) (*workspace.WorkspaceConfiguration, error) {
	if agentName == "" {
		return nil, fmt.Errorf("%w: agent name cannot be empty", ErrInvalidAgentConfig)
	}

	configPath := filepath.Join(a.storageDir, "config", AgentsConfigFile)

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - return empty config (not an error)
			return &workspace.WorkspaceConfiguration{}, nil
		}
		return nil, err
	}

	// Parse the JSON
	var agentsConfig map[string]workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &agentsConfig); err != nil {
		return nil, fmt.Errorf("%w: failed to parse agents.json: %v", ErrInvalidAgentConfig, err)
	}

	// Get the configuration for the specified agent
	cfg, exists := agentsConfig[agentName]
	if !exists {
		// Agent not found - return empty config (not an error)
		return &workspace.WorkspaceConfiguration{}, nil
	}

	// Validate the configuration
	validator := &config{path: a.storageDir}
	if err := validator.validate(&cfg); err != nil {
		return nil, fmt.Errorf("%w: agent %q configuration validation failed: %v", ErrInvalidAgentConfig, agentName, err)
	}

	return &cfg, nil
}
