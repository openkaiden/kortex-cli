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
	"path"
	"path/filepath"
	"regexp"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
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
// It manages the structure and contents of a workspace configuration directory (typically .kaiden).
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

// isValidHostPath returns true if the host path is a native OS absolute path or starts with
// one of the allowed variable prefixes. Variables should be provided without the leading "$"
// (e.g. "HOME", "SOURCES"). Uses filepath.IsAbs so that only paths valid on the current OS
// are accepted (e.g. "C:\foo" on Windows, "/foo" on Unix).
func isValidHostPath(p string, variables []string) bool {
	if filepath.IsAbs(p) {
		return true
	}
	for _, v := range variables {
		if strings.HasPrefix(p, "$"+v) {
			return true
		}
	}
	return false
}

// isValidTargetPath returns true if the container target path is a Unix absolute path or starts with $SOURCES or $HOME.
// Uses path.IsAbs (not filepath.IsAbs) because container paths are always Unix-style
// regardless of the host OS.
func isValidTargetPath(p string) bool {
	return path.IsAbs(p) || strings.HasPrefix(p, "$SOURCES") || strings.HasPrefix(p, "$HOME")
}

// mountTargetWithinRoot returns true if resolved is equal to root or is a direct descendant.
func mountTargetWithinRoot(resolved, root string) bool {
	suffix := strings.TrimPrefix(resolved, root)
	return suffix != resolved && (suffix == "" || strings.HasPrefix(suffix, "/"))
}

// checkTargetEscape returns an error if a $SOURCES or $HOME target escapes above its root.
// $SOURCES targets must stay within /workspace; $HOME targets must not escape above $HOME itself.
func checkTargetEscape(target string) error {
	switch {
	case strings.HasPrefix(target, "$SOURCES"):
		resolved := path.Join("/workspace/sources", target[len("$SOURCES"):])
		if !mountTargetWithinRoot(resolved, "/workspace") {
			return fmt.Errorf("mount target %q escapes above $SOURCES", target)
		}
	case strings.HasPrefix(target, "$HOME"):
		// Use a fixed placeholder home; the check is relative so the actual user name does not matter.
		const placeholderHome = "/home/user"
		resolved := path.Join(placeholderHome, target[len("$HOME"):])
		if !mountTargetWithinRoot(resolved, placeholderHome) {
			return fmt.Errorf("mount target %q escapes above $HOME", target)
		}
	}
	return nil
}

// validate checks that the configuration is valid.
// It ensures that environment variables have exactly one of value or secret defined,
// that secret references are not empty, that names are valid Unix environment variable names,
// and that mount paths are non-empty and relative.
func (c *config) validate(cfg *workspace.WorkspaceConfiguration) error {
	if cfg.Environment != nil {
		seen := make(map[string]int)
		for i, env := range *cfg.Environment {
			// Check that name is not empty
			if env.Name == "" {
				return fmt.Errorf("%w: environment variable at index %d has empty name", ErrInvalidConfig, i)
			}

			// Check for duplicate names
			if prevIdx, exists := seen[env.Name]; exists {
				return fmt.Errorf("%w: environment variable %q (index %d) is a duplicate of index %d", ErrInvalidConfig, env.Name, i, prevIdx)
			}
			seen[env.Name] = i

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

	// Validate mounts
	if cfg.Mounts != nil {
		for i, m := range *cfg.Mounts {
			if m.Host == "" {
				return fmt.Errorf("%w: mount at index %d is missing host", ErrInvalidConfig, i)
			}
			if m.Target == "" {
				return fmt.Errorf("%w: mount at index %d is missing target", ErrInvalidConfig, i)
			}
			if !isValidHostPath(m.Host, []string{"SOURCES", "HOME"}) {
				return fmt.Errorf("%w: mount host %q (index %d) must be a native absolute path or start with $SOURCES or $HOME", ErrInvalidConfig, m.Host, i)
			}
			if !isValidTargetPath(m.Target) {
				return fmt.Errorf("%w: mount target %q (index %d) must be a Unix absolute path or start with $SOURCES or $HOME", ErrInvalidConfig, m.Target, i)
			}
			if err := checkTargetEscape(m.Target); err != nil {
				return fmt.Errorf("%w: %s (index %d)", ErrInvalidConfig, err, i)
			}
		}
	}

	// Validate skills
	if cfg.Skills != nil {
		for i, s := range *cfg.Skills {
			if s == "" {
				return fmt.Errorf("%w: skills path at index %d is empty", ErrInvalidConfig, i)
			}
			if !isValidHostPath(s, []string{"HOME"}) {
				return fmt.Errorf("%w: skills path %q (index %d) must be a native absolute path or start with $HOME", ErrInvalidConfig, s, i)
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
