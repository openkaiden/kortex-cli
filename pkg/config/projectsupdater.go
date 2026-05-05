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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// ProjectConfigUpdater adds entries to the per-project configuration file.
type ProjectConfigUpdater interface {
	// AddSecret appends secretName to the Secrets list of the given project.
	// projectID is "" for the global configuration.
	// The call is idempotent: if the secret is already present it is not duplicated.
	AddSecret(projectID string, secretName string) error
}

// projectConfigUpdater is the unexported implementation.
type projectConfigUpdater struct {
	storageDir string
}

var _ ProjectConfigUpdater = (*projectConfigUpdater)(nil)

// NewProjectConfigUpdater returns a ProjectConfigUpdater backed by
// <storageDir>/config/projects.json.
func NewProjectConfigUpdater(storageDir string) (ProjectConfigUpdater, error) {
	if storageDir == "" {
		return nil, ErrInvalidPath
	}
	absPath, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve storage directory path: %w", err)
	}
	return &projectConfigUpdater{storageDir: absPath}, nil
}

// AddSecret reads projects.json, adds secretName to the Secrets list for projectID,
// and writes the file back. The operation is idempotent.
func (p *projectConfigUpdater) AddSecret(projectID string, secretName string) error {
	configPath := filepath.Join(p.storageDir, "config", ProjectsConfigFile)

	projectsConfig, err := p.readProjectsFile(configPath)
	if err != nil {
		return err
	}

	cfg := projectsConfig[projectID]

	if cfg.Secrets == nil {
		secrets := []string{secretName}
		cfg.Secrets = &secrets
	} else {
		for _, s := range *cfg.Secrets {
			if s == secretName {
				return nil
			}
		}
		*cfg.Secrets = append(*cfg.Secrets, secretName)
	}

	projectsConfig[projectID] = cfg

	return p.writeProjectsFile(configPath, projectsConfig)
}

func (p *projectConfigUpdater) readProjectsFile(configPath string) (map[string]workspace.WorkspaceConfiguration, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]workspace.WorkspaceConfiguration), nil
		}
		return nil, fmt.Errorf("failed to read projects config: %w", err)
	}

	var projectsConfig map[string]workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &projectsConfig); err != nil {
		return nil, fmt.Errorf("failed to parse projects config: %w", err)
	}
	return projectsConfig, nil
}

func (p *projectConfigUpdater) writeProjectsFile(configPath string, projectsConfig map[string]workspace.WorkspaceConfiguration) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(projectsConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal projects config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write projects config: %w", err)
	}
	return nil
}
