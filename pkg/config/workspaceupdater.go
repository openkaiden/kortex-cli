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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// WorkspaceConfigUpdater manages the local workspace configuration file.
type WorkspaceConfigUpdater interface {
	// AddSecret appends secretName to the Secrets list of the workspace config,
	// creating the file and directory if they do not yet exist.
	// The call is idempotent: if the secret is already present it is not duplicated.
	AddSecret(secretName string) error
}

type workspaceConfigUpdater struct {
	configDir string // absolute path to the .kaiden/ directory
}

var _ WorkspaceConfigUpdater = (*workspaceConfigUpdater)(nil)

// NewWorkspaceConfigUpdater returns a WorkspaceConfigUpdater backed by
// <configDir>/workspace.json.
func NewWorkspaceConfigUpdater(configDir string) (WorkspaceConfigUpdater, error) {
	if configDir == "" {
		return nil, ErrInvalidPath
	}
	absPath, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config directory path: %w", err)
	}
	return &workspaceConfigUpdater{configDir: absPath}, nil
}

func (w *workspaceConfigUpdater) AddSecret(secretName string) error {
	configPath := filepath.Join(w.configDir, WorkspaceConfigFile)

	cfg, err := w.readConfig(configPath)
	if err != nil {
		return err
	}

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

	return w.writeConfig(configPath, cfg)
}

func (w *workspaceConfigUpdater) readConfig(configPath string) (*workspace.WorkspaceConfiguration, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &workspace.WorkspaceConfiguration{}, nil
		}
		return nil, fmt.Errorf("failed to read workspace config: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return &workspace.WorkspaceConfiguration{}, nil
	}

	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse workspace config: %w", err)
	}
	return &cfg, nil
}

func (w *workspaceConfigUpdater) writeConfig(configPath string, cfg *workspace.WorkspaceConfiguration) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write workspace config: %w", err)
	}
	return nil
}
