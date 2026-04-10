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
	// ProjectsConfigFile is the name of the projects configuration file
	ProjectsConfigFile = "projects.json"
)

var (
	// ErrInvalidProjectConfig is returned when the project configuration is invalid
	ErrInvalidProjectConfig = errors.New("invalid project configuration")
)

// ProjectConfigLoader loads project-specific workspace configurations
type ProjectConfigLoader interface {
	// Load reads and returns the workspace configuration for the specified project.
	// It first loads the global configuration (empty string "" key), then loads
	// the project-specific configuration, and merges them with project-specific
	// taking precedence.
	// Returns an empty configuration (not an error) if the projects.json file doesn't exist.
	// Returns an error if the file exists but is invalid JSON or malformed.
	Load(projectID string) (*workspace.WorkspaceConfiguration, error)
}

// projectConfigLoader is the internal implementation
type projectConfigLoader struct {
	storageDir string
	merger     Merger
}

// Compile-time check to ensure projectConfigLoader implements ProjectConfigLoader interface
var _ ProjectConfigLoader = (*projectConfigLoader)(nil)

// NewProjectConfigLoader creates a new project configuration loader
func NewProjectConfigLoader(storageDir string) (ProjectConfigLoader, error) {
	if storageDir == "" {
		return nil, ErrInvalidPath
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, err
	}

	return &projectConfigLoader{
		storageDir: absPath,
		merger:     NewMerger(),
	}, nil
}

// Load reads and returns the workspace configuration for the specified project
func (p *projectConfigLoader) Load(projectID string) (*workspace.WorkspaceConfiguration, error) {
	configPath := filepath.Join(p.storageDir, "config", ProjectsConfigFile)

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
	var projectsConfig map[string]workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &projectsConfig); err != nil {
		return nil, fmt.Errorf("%w: failed to parse projects.json: %v", ErrInvalidProjectConfig, err)
	}

	// Load global configuration (empty string key)
	globalCfg, globalExists := projectsConfig[""]
	if globalExists {
		// Validate global configuration
		validator := &config{path: p.storageDir}
		if err := validator.validate(&globalCfg); err != nil {
			return nil, fmt.Errorf("%w: global configuration validation failed: %v", ErrInvalidProjectConfig, err)
		}
	}

	// Load project-specific configuration
	projectCfg, projectExists := projectsConfig[projectID]
	if projectExists {
		// Validate project configuration
		validator := &config{path: p.storageDir}
		if err := validator.validate(&projectCfg); err != nil {
			return nil, fmt.Errorf("%w: project %q configuration validation failed: %v", ErrInvalidProjectConfig, projectID, err)
		}
	}

	// Merge configurations: global -> project-specific
	if !globalExists && !projectExists {
		// Neither global nor project-specific config exists
		return &workspace.WorkspaceConfiguration{}, nil
	}

	if !globalExists {
		// Only project-specific config exists
		return &projectCfg, nil
	}

	if !projectExists {
		// Only global config exists
		return &globalCfg, nil
	}

	// Both exist - merge with project-specific taking precedence
	merged := p.merger.Merge(&globalCfg, &projectCfg)
	return merged, nil
}
