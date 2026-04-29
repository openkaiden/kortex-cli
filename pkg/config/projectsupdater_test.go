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
	"os"
	"path/filepath"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestNewProjectConfigUpdater_EmptyStorageDir(t *testing.T) {
	t.Parallel()
	_, err := NewProjectConfigUpdater("")
	if err == nil {
		t.Error("expected error for empty storage dir")
	}
}

func TestAddSecret_CreatesFileWhenMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	if err := updater.AddSecret("", "anthropic"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

	cfg := readProjectsFile(t, dir)
	global := cfg[""]
	if global.Secrets == nil || len(*global.Secrets) != 1 || (*global.Secrets)[0] != "anthropic" {
		t.Errorf("expected [anthropic] in global secrets, got %v", global.Secrets)
	}
}

func TestAddSecret_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	for range 3 {
		if err := updater.AddSecret("", "github"); err != nil {
			t.Fatalf("AddSecret: %v", err)
		}
	}

	cfg := readProjectsFile(t, dir)
	secrets := *cfg[""].Secrets
	if len(secrets) != 1 {
		t.Errorf("expected exactly 1 secret, got %v", secrets)
	}
}

func TestAddSecret_AccumulatesMultiple(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	for _, name := range []string{"anthropic", "github", "gemini"} {
		if err := updater.AddSecret("", name); err != nil {
			t.Fatalf("AddSecret(%s): %v", name, err)
		}
	}

	cfg := readProjectsFile(t, dir)
	secrets := *cfg[""].Secrets
	if len(secrets) != 3 {
		t.Errorf("expected 3 secrets, got %v", secrets)
	}
}

func TestAddSecret_ProjectSpecificKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	if err := updater.AddSecret("my-project", "github"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

	cfg := readProjectsFile(t, dir)
	if _, ok := cfg[""]; ok {
		t.Error("expected no global key")
	}
	projectCfg := cfg["my-project"]
	if projectCfg.Secrets == nil || len(*projectCfg.Secrets) != 1 || (*projectCfg.Secrets)[0] != "github" {
		t.Errorf("expected [github] in project secrets, got %v", projectCfg.Secrets)
	}
}

func TestAddSecret_GlobalAndProjectIndependent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	if err := updater.AddSecret("", "anthropic"); err != nil {
		t.Fatalf("AddSecret global: %v", err)
	}
	if err := updater.AddSecret("proj-a", "github"); err != nil {
		t.Fatalf("AddSecret project: %v", err)
	}

	cfg := readProjectsFile(t, dir)
	if s := *cfg[""].Secrets; len(s) != 1 || s[0] != "anthropic" {
		t.Errorf("global secrets unexpected: %v", s)
	}
	if s := *cfg["proj-a"].Secrets; len(s) != 1 || s[0] != "github" {
		t.Errorf("project secrets unexpected: %v", s)
	}
}

// readProjectsFile is a test helper that reads and parses the projects.json file.
func readProjectsFile(t *testing.T, storageDir string) map[string]workspace.WorkspaceConfiguration {
	t.Helper()
	path := filepath.Join(storageDir, "config", ProjectsConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read projects file: %v", err)
	}
	var cfg map[string]workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse projects file: %v", err)
	}
	return cfg
}
