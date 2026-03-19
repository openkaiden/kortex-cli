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

	workspace "github.com/kortex-hub/kortex-cli-api/workspace-configuration/go"
)

const (
	// WorkspaceConfigFile is the name of the workspace configuration file
	WorkspaceConfigFile = "workspace.json"
)

var (
	// envVarNamePattern matches valid Unix environment variable names.
	// Names must start with a letter or underscore, followed by letters, digits, or underscores.
	envVarNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

var (
	// ErrInvalidPath is returned when a configuration path is invalid or empty
	ErrInvalidPath = errors.New("invalid configuration path")
	// ErrConfigNotFound is returned when the workspace.json file is not found
	ErrConfigNotFound = errors.New("workspace configuration file not found")
	// ErrInvalidConfig is returned when the configuration validation fails
	ErrInvalidConfig = errors.New("invalid workspace configuration")
)

// Config represents a workspace configuration manager.
// It manages the structure and contents of a workspace configuration directory (typically .kortex).
// If the configuration directory does not exist, the config is considered empty.
type Config interface {
	// Load reads and parses the workspace configuration from workspace.json.
	// Returns ErrConfigNotFound if the workspace.json file doesn't exist.
	// Returns an error if the JSON is malformed or cannot be read.
	Load() (*workspace.WorkspaceConfiguration, error)
}

// config is the internal implementation of Config
type config struct {
	// path is the absolute path to the configuration directory
	path string
}

// Compile-time check to ensure config implements Config interface
var _ Config = (*config)(nil)

// validate checks that the configuration is valid.
// It ensures that environment variables have exactly one of value or secret defined,
// that secret references are not empty, that names are valid Unix environment variable names,
// and that mount paths are non-empty and relative.
func (c *config) validate(cfg *workspace.WorkspaceConfiguration) error {
	if cfg.Environment != nil {
		for i, env := range *cfg.Environment {
			// Check that name is not empty
			if env.Name == "" {
				return fmt.Errorf("%w: environment variable at index %d has empty name", ErrInvalidConfig, i)
			}

			// Check that name is a valid Unix environment variable name
			if !envVarNamePattern.MatchString(env.Name) {
				return fmt.Errorf("%w: environment variable %q (index %d) has invalid name (must start with letter or underscore, followed by letters, digits, or underscores)", ErrInvalidConfig, env.Name, i)
			}

			// Check that secret is not empty if set
			if env.Secret != nil && *env.Secret == "" {
				return fmt.Errorf("%w: environment variable %q (index %d) has empty secret reference", ErrInvalidConfig, env.Name, i)
			}

			// Check that exactly one of value or secret is defined
			// Note: empty string values are allowed, but empty string secrets are not
			hasValue := env.Value != nil
			hasSecret := env.Secret != nil && *env.Secret != ""

			if hasValue && hasSecret {
				return fmt.Errorf("%w: environment variable %q (index %d) has both value and secret set", ErrInvalidConfig, env.Name, i)
			}

			if !hasValue && !hasSecret {
				return fmt.Errorf("%w: environment variable %q (index %d) must have either value or secret set", ErrInvalidConfig, env.Name, i)
			}
		}
	}

	// Validate mount paths
	if cfg.Mounts != nil {
		if cfg.Mounts.Dependencies != nil {
			for i, dep := range *cfg.Mounts.Dependencies {
				if dep == "" {
					return fmt.Errorf("%w: dependency mount at index %d is empty", ErrInvalidConfig, i)
				}
				if filepath.IsAbs(dep) {
					return fmt.Errorf("%w: dependency mount %q (index %d) must be a relative path", ErrInvalidConfig, dep, i)
				}
			}
		}

		if cfg.Mounts.Configs != nil {
			for i, conf := range *cfg.Mounts.Configs {
				if conf == "" {
					return fmt.Errorf("%w: config mount at index %d is empty", ErrInvalidConfig, i)
				}
				if filepath.IsAbs(conf) {
					return fmt.Errorf("%w: config mount %q (index %d) must be a relative path", ErrInvalidConfig, conf, i)
				}
			}
		}
	}

	return nil
}

// Load reads and parses the workspace configuration from workspace.json
func (c *config) Load() (*workspace.WorkspaceConfiguration, error) {
	configPath := filepath.Join(c.path, WorkspaceConfigFile)

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	// Parse the JSON
	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Validate the configuration
	if err := c.validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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
