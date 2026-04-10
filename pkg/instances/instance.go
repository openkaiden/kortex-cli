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

package instances

import (
	"errors"
	"maps"
	"os"
	"path/filepath"

	api "github.com/openkaiden/kdn-api/cli/go"
)

var (
	// ErrInstanceNotFound is returned when an instance is not found
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrInstanceExists is returned when trying to add a duplicate instance
	ErrInstanceExists = errors.New("instance already exists")
	// ErrInvalidPath is returned when a path is invalid or empty
	ErrInvalidPath = errors.New("invalid path")
)

// InstancePaths represents the paths in an instance
type InstancePaths struct {
	// Source is the directory containing source files (absolute path)
	Source string `json:"source"`
	// Configuration is the directory containing workspace configuration (absolute path)
	Configuration string `json:"configuration"`
}

// RuntimeData represents runtime-specific information for an instance
type RuntimeData struct {
	// Type is the runtime type (e.g., "fake", "podman", "docker")
	Type string `json:"type"`
	// InstanceID is the runtime-assigned instance identifier
	InstanceID string `json:"instance_id"`
	// State is the current runtime state (e.g., "running", "stopped")
	State api.WorkspaceState `json:"state"`
	// Info contains runtime-specific metadata
	Info map[string]string `json:"info"`
}

// InstanceData represents the serializable data of an instance
type InstanceData struct {
	// ID is the unique identifier for the instance
	ID string `json:"id"`
	// Name is the human-readable name for the instance
	Name string `json:"name"`
	// Paths contains the source and configuration directories
	Paths InstancePaths `json:"paths"`
	// Runtime contains runtime-specific information
	Runtime RuntimeData `json:"runtime"`
	// Project is the project identifier for grouping instances.
	// Format depends on workspace type:
	// - Git repository with remote: normalized remote repository identifier (remote URL with .git suffix
	//   and credentials stripped, e.g., "https://github.com/user/repo")
	// - Git repository without remote: repository root directory as absolute path (e.g., "/home/user/my-local-repo")
	// - Non-git directory: workspace source directory as absolute path (e.g., "/home/user/my-workspace")
	// When the workspace is in a subdirectory of the repository, a relative-path suffix is appended
	// (e.g., "https://github.com/user/repo/pkg/subdir" or "/home/user/my-local-repo/pkg/subdir")
	Project string `json:"project"`
	// Agent is the agent name for this instance (e.g., "claude", "goose").
	// This is used to load agent-specific configuration and determine which agent command to run.
	Agent string `json:"agent"`
	// Model is the model ID configured for the agent (e.g., "claude-sonnet-4-20250514").
	// Empty string means no model was explicitly specified.
	Model string `json:"model,omitempty"`
}

// Instance represents a workspace instance with source and configuration directories.
// Each instance is uniquely identified by its ID and has a human-readable name.
// Instances must be created using NewInstance to ensure proper validation and path normalization.
type Instance interface {
	// GetID returns the unique identifier for the instance.
	GetID() string
	// GetName returns the human-readable name for the instance.
	GetName() string
	// GetSourceDir returns the source directory.
	// The path is always absolute.
	GetSourceDir() string
	// GetConfigDir returns the configuration directory.
	// The path is always absolute.
	GetConfigDir() string
	// IsAccessible checks if both source and config directories are accessible
	IsAccessible() bool
	// GetRuntimeType returns the runtime type for this instance
	GetRuntimeType() string
	// GetRuntimeData returns the complete runtime data for this instance
	GetRuntimeData() RuntimeData
	// GetProject returns the project identifier for this instance
	GetProject() string
	// GetAgent returns the agent name for this instance
	GetAgent() string
	// GetModel returns the model ID for this instance (empty if not set)
	GetModel() string
	// Dump returns the serializable data of the instance
	Dump() InstanceData
}

// instance is the internal implementation of Instance
type instance struct {
	// ID is the unique identifier for the instance
	ID string
	// Name is the human-readable name for the instance
	Name string
	// SourceDir is the directory containing source files.
	// This is always stored as an absolute path.
	SourceDir string
	// ConfigDir is the directory containing workspace configuration.
	// This is always stored as an absolute path.
	ConfigDir string
	// Runtime contains runtime-specific information
	Runtime RuntimeData
	// Project is the project identifier for grouping instances
	Project string
	// Agent is the agent name for this instance
	Agent string
	// Model is the model ID configured for the agent (empty if not set)
	Model string
}

// Compile-time check to ensure instance implements Instance interface
var _ Instance = (*instance)(nil)

// GetID returns the unique identifier for the instance
func (i *instance) GetID() string {
	return i.ID
}

// GetName returns the human-readable name for the instance
func (i *instance) GetName() string {
	return i.Name
}

// GetSourceDir returns the source directory
func (i *instance) GetSourceDir() string {
	return i.SourceDir
}

// GetConfigDir returns the configuration directory
func (i *instance) GetConfigDir() string {
	return i.ConfigDir
}

// IsAccessible checks if both source and config directories are accessible
func (i *instance) IsAccessible() bool {
	if !isDirAccessible(i.SourceDir) {
		return false
	}
	if !isDirAccessible(i.ConfigDir) {
		return false
	}
	return true
}

// GetRuntimeType returns the runtime type for this instance
func (i *instance) GetRuntimeType() string {
	return i.Runtime.Type
}

// GetRuntimeData returns the complete runtime data for this instance
func (i *instance) GetRuntimeData() RuntimeData {
	infoCopy := make(map[string]string, len(i.Runtime.Info))
	maps.Copy(infoCopy, i.Runtime.Info)
	return RuntimeData{
		Type:       i.Runtime.Type,
		InstanceID: i.Runtime.InstanceID,
		State:      i.Runtime.State,
		Info:       infoCopy,
	}
}

// GetProject returns the project identifier for this instance
func (i *instance) GetProject() string {
	return i.Project
}

// GetAgent returns the agent name for this instance
func (i *instance) GetAgent() string {
	return i.Agent
}

// GetModel returns the model ID for this instance (empty if not set)
func (i *instance) GetModel() string {
	return i.Model
}

// Dump returns the serializable data of the instance
func (i *instance) Dump() InstanceData {
	return InstanceData{
		ID:   i.ID,
		Name: i.Name,
		Paths: InstancePaths{
			Source:        i.SourceDir,
			Configuration: i.ConfigDir,
		},
		Runtime: i.Runtime,
		Project: i.Project,
		Agent:   i.Agent,
		Model:   i.Model,
	}
}

// NewInstanceParams contains the parameters for creating a new Instance
type NewInstanceParams struct {
	// SourceDir is the directory containing source files
	SourceDir string
	// ConfigDir is the directory containing workspace configuration
	ConfigDir string
	// Name is the human-readable name for the instance (optional, will be generated if empty)
	Name string
}

// NewInstance creates a new Instance with validated and normalized paths.
// Both sourceDir and configDir are converted to absolute paths.
// The ID and Name (if empty) will be generated by the Manager when the instance is added to storage.
func NewInstance(params NewInstanceParams) (Instance, error) {
	if params.SourceDir == "" {
		return nil, ErrInvalidPath
	}
	if params.ConfigDir == "" {
		return nil, ErrInvalidPath
	}

	// Convert to absolute paths
	absSourceDir, err := filepath.Abs(params.SourceDir)
	if err != nil {
		return nil, err
	}

	absConfigDir, err := filepath.Abs(params.ConfigDir)
	if err != nil {
		return nil, err
	}

	return &instance{
		ID:        "", // ID will be set by Manager when added
		Name:      params.Name,
		SourceDir: absSourceDir,
		ConfigDir: absConfigDir,
	}, nil
}

// NewInstanceFromData creates a new Instance from InstanceData.
// The paths in InstanceData are assumed to be already validated and absolute.
func NewInstanceFromData(data InstanceData) (Instance, error) {
	if data.ID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	if data.Name == "" {
		return nil, errors.New("instance name cannot be empty")
	}
	if data.Paths.Source == "" {
		return nil, ErrInvalidPath
	}
	if data.Paths.Configuration == "" {
		return nil, ErrInvalidPath
	}

	return &instance{
		ID:        data.ID,
		Name:      data.Name,
		SourceDir: data.Paths.Source,
		ConfigDir: data.Paths.Configuration,
		Runtime:   data.Runtime,
		Project:   data.Project,
		Agent:     data.Agent,
		Model:     data.Model,
	}, nil
}

// isDirAccessible checks if a directory exists and is accessible
func isDirAccessible(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
