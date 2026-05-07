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
	"runtime"
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

// TestAddSecret_ReadError covers the error propagation path in AddSecret when
// readProjectsFile fails with a non-NotExist error (projects.json is a directory).
func TestAddSecret_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Make projects.json a directory so ReadFile returns "is a directory".
	configPath := filepath.Join(dir, "config", ProjectsConfigFile)
	if err := os.MkdirAll(configPath, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}
	if err := updater.AddSecret("", "anthropic"); err == nil {
		t.Error("expected error when projects.json is a directory, got nil")
	}
}

// TestReadProjectsFile_NonExistError covers the non-NotExist error branch
// in readProjectsFile (projects.json exists but is unreadable as a file).
func TestReadProjectsFile_NonExistError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a directory at the file path — ReadFile returns "is a directory".
	configPath := filepath.Join(dir, "config", ProjectsConfigFile)
	if err := os.MkdirAll(configPath, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	p := &projectConfigUpdater{storageDir: dir}
	_, err := p.readProjectsFile(configPath)
	if err == nil {
		t.Error("expected error for directory-as-file, got nil")
	}
}

// TestReadProjectsFile_InvalidJSON covers the JSON unmarshal error branch.
func TestReadProjectsFile_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, ProjectsConfigFile)
	if err := os.WriteFile(configPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	p := &projectConfigUpdater{storageDir: dir}
	_, err := p.readProjectsFile(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestWriteProjectsFile_MkdirAllFails covers the MkdirAll error branch by
// placing a regular file where the config directory is expected.
func TestWriteProjectsFile_MkdirAllFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a regular file at the path where the config directory would go.
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("file"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	p := &projectConfigUpdater{storageDir: dir}
	configPath := filepath.Join(dir, "config", ProjectsConfigFile)
	err := p.writeProjectsFile(configPath, map[string]workspace.WorkspaceConfiguration{})
	if err == nil {
		t.Error("expected error when config path is a file, got nil")
	}
}

// TestWriteProjectsFile_WriteFileFails covers the WriteFile error branch by
// making the config directory read-only.
func TestWriteProjectsFile_WriteFileFails(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission tests do not apply on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("chmod restrictions do not apply to root")
	}

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	// Remove write permission so WriteFile fails.
	if err := os.Chmod(configDir, 0500); err != nil {
		t.Fatalf("setup chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configDir, 0700) })

	p := &projectConfigUpdater{storageDir: dir}
	configPath := filepath.Join(configDir, ProjectsConfigFile)
	err := p.writeProjectsFile(configPath, map[string]workspace.WorkspaceConfiguration{})
	if err == nil {
		t.Error("expected error writing to read-only directory, got nil")
	}
}

func TestAddMount_CreatesFileWhenMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	if err := updater.AddMount("", "$HOME/.gitconfig", "$HOME/.gitconfig", true); err != nil {
		t.Fatalf("AddMount: %v", err)
	}

	cfg := readProjectsFile(t, dir)
	global := cfg[""]
	if global.Mounts == nil || len(*global.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %v", global.Mounts)
	}
	m := (*global.Mounts)[0]
	if m.Host != "$HOME/.gitconfig" || m.Target != "$HOME/.gitconfig" || m.Ro == nil || !*m.Ro {
		t.Errorf("unexpected mount: %+v", m)
	}
}

func TestAddMount_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	for range 3 {
		if err := updater.AddMount("", "$HOME/.gitconfig", "$HOME/.gitconfig", true); err != nil {
			t.Fatalf("AddMount: %v", err)
		}
	}

	cfg := readProjectsFile(t, dir)
	if n := len(*cfg[""].Mounts); n != 1 {
		t.Errorf("expected exactly 1 mount, got %d", n)
	}
}

func TestAddMount_ProjectSpecificKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	if err := updater.AddMount("my-project", "$HOME/.gitconfig", "$HOME/.gitconfig", true); err != nil {
		t.Fatalf("AddMount: %v", err)
	}

	cfg := readProjectsFile(t, dir)
	if _, ok := cfg[""]; ok {
		t.Error("expected no global key")
	}
	mounts := *cfg["my-project"].Mounts
	if len(mounts) != 1 || mounts[0].Target != "$HOME/.gitconfig" {
		t.Errorf("unexpected mounts: %v", mounts)
	}
}

func TestAddMount_AccumulatesMultiple(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}

	mounts := [][2]string{
		{"$HOME/.gitconfig", "$HOME/.gitconfig"},
		{"$HOME/.npmrc", "$HOME/.npmrc"},
	}
	for _, m := range mounts {
		if err := updater.AddMount("", m[0], m[1], true); err != nil {
			t.Fatalf("AddMount(%s): %v", m[0], err)
		}
	}

	cfg := readProjectsFile(t, dir)
	if n := len(*cfg[""].Mounts); n != 2 {
		t.Errorf("expected 2 mounts, got %d", n)
	}
}

func TestAddMount_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config", ProjectsConfigFile)
	if err := os.MkdirAll(configPath, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	updater, err := NewProjectConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewProjectConfigUpdater: %v", err)
	}
	if err := updater.AddMount("", "$HOME/.gitconfig", "$HOME/.gitconfig", true); err == nil {
		t.Error("expected error when projects.json is a directory, got nil")
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
