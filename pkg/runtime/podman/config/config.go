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
	"regexp"
	"sort"
	"strings"
)

var (
	// ErrInvalidPath is returned when a configuration path is invalid or empty
	ErrInvalidPath = errors.New("invalid configuration path")
	// ErrConfigNotFound is returned when a configuration file is not found
	ErrConfigNotFound = errors.New("configuration file not found for podman runtime and agent")
	// ErrInvalidConfig is returned when configuration validation fails
	ErrInvalidConfig = errors.New("invalid configuration")

	// agentNamePattern matches valid agent names (alphanumeric and underscore only).
	agentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

// Config represents a Podman runtime configuration manager.
// It manages the structure and contents of the Podman runtime configuration directory.
type Config interface {
	// LoadImage reads and parses the base image configuration from image.json.
	// Returns ErrConfigNotFound if the image.json file doesn't exist.
	// Returns ErrInvalidConfig if the configuration is invalid.
	LoadImage() (*ImageConfig, error)

	// LoadAgent reads and parses the agent-specific configuration.
	// Returns ErrConfigNotFound if the agent configuration file doesn't exist.
	// Returns ErrInvalidConfig if the configuration is invalid.
	LoadAgent(agentName string) (*AgentConfig, error)

	// ListAgents returns the names of all configured agents.
	// It scans the configuration directory for *.json files, excluding image.json.
	// Returns an empty slice if the directory does not exist.
	ListAgents() ([]string, error)

	// GenerateDefaults creates default configuration files if they don't exist.
	// Creates the configuration directory if it doesn't exist.
	// Does not overwrite existing configuration files.
	GenerateDefaults() error
}

// config is the internal implementation of Config
type config struct {
	// path is the absolute path to the configuration directory
	path string
}

// Compile-time check to ensure config implements Config interface
var _ Config = (*config)(nil)

// LoadImage reads and parses the base image configuration from image.json
func (c *config) LoadImage() (*ImageConfig, error) {
	configPath := filepath.Join(c.path, ImageConfigFileName)

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	// Parse the JSON
	var cfg ImageConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// Validate the configuration
	if err := validateImageConfig(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	return &cfg, nil
}

// LoadAgent reads and parses the agent-specific configuration
func (c *config) LoadAgent(agentName string) (*AgentConfig, error) {
	if agentName == "" {
		return nil, fmt.Errorf("%w: agent name cannot be empty", ErrInvalidConfig)
	}

	// Validate agent name to prevent path traversal attacks
	if !agentNamePattern.MatchString(agentName) {
		return nil, fmt.Errorf("%w: agent name must contain only alphanumeric characters or underscores: %s", ErrInvalidConfig, agentName)
	}

	configPath := filepath.Join(c.path, agentName+".json")

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	// Parse the JSON
	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// Validate the configuration
	if err := validateAgentConfig(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	return &cfg, nil
}

// generateConfigFile generates a configuration file if it doesn't exist.
// Does not overwrite existing files.
func (c *config) generateConfigFile(filename string, config interface{}) error {
	configPath := filepath.Join(c.path, filename)
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat %s: %w", configPath, err)
		}
		// File doesn't exist, generate it
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", filename, err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	} else {
		// File exists, check if it's a directory
		if fileInfo.IsDir() {
			return fmt.Errorf("expected file but found directory: %s", configPath)
		}
	}
	return nil
}

// ListAgents returns the names of all configured agents by scanning for *.json files
// in the config directory, excluding image.json.
func (c *config) ListAgents() ([]string, error) {
	entries, err := os.ReadDir(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var agents []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if name == ImageConfigFileName {
			continue
		}
		agents = append(agents, strings.TrimSuffix(name, ".json"))
	}

	sort.Strings(agents)
	return agents, nil
}

// GenerateDefaults creates default configuration files if they don't exist
func (c *config) GenerateDefaults() error {
	// Create the configuration directory if it doesn't exist
	if err := os.MkdirAll(c.path, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate default configurations
	configs := map[string]interface{}{
		ImageConfigFileName:  defaultImageConfig(),
		ClaudeConfigFileName: defaultClaudeConfig(),
		GooseConfigFileName:  defaultGooseConfig(),
		CursorConfigFileName: defaultCursorConfig(),
	}

	for filename, config := range configs {
		if err := c.generateConfigFile(filename, config); err != nil {
			return err
		}
	}

	return nil
}

// NewConfig creates a new Config for the specified configuration directory.
// The configDir is converted to an absolute path.
func NewConfig(configDir string) (Config, error) {
	if configDir == "" {
		return nil, ErrInvalidPath
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(configDir)
	if err != nil {
		return nil, err
	}

	return &config{
		path: absPath,
	}, nil
}
