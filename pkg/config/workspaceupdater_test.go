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

func readWorkspaceSecrets(t *testing.T, configDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, WorkspaceConfigFile))
	if err != nil {
		t.Fatalf("failed to read workspace.json: %v", err)
	}
	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse workspace.json: %v", err)
	}
	if cfg.Secrets == nil {
		return nil
	}
	return *cfg.Secrets
}

func TestWorkspaceUpdater_FileMissing_CreatesIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	if err := u.AddSecret("github"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 1 || secrets[0] != "github" {
		t.Errorf("expected [github], got %v", secrets)
	}
}

func TestWorkspaceUpdater_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	u, _ := NewWorkspaceConfigUpdater(dir)
	_ = u.AddSecret("github")
	_ = u.AddSecret("github")

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(secrets))
	}
}

func TestWorkspaceUpdater_MultipleCalls_Accumulate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	u, _ := NewWorkspaceConfigUpdater(dir)
	_ = u.AddSecret("github")
	_ = u.AddSecret("anthropic")

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d: %v", len(secrets), secrets)
	}
}

func TestWorkspaceUpdater_ExistingFile_Preserved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	existing := workspace.WorkspaceConfiguration{}
	s := []string{"existing-secret"}
	existing.Secrets = &s
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFile), data, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	u, _ := NewWorkspaceConfigUpdater(dir)
	if err := u.AddSecret("new-secret"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 2 {
		t.Errorf("expected 2 secrets, got %v", secrets)
	}
}

func TestWorkspaceUpdater_EmptyConfigDir_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := NewWorkspaceConfigUpdater("")
	if err == nil {
		t.Error("expected error for empty configDir")
	}
}

// TestWorkspaceUpdater_ReadError covers the error propagation path in AddSecret
// when readConfig fails with a non-NotExist error (workspace.json is a directory).
func TestWorkspaceUpdater_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Place a directory at the workspace.json path so ReadFile returns "is a directory".
	if err := os.MkdirAll(filepath.Join(dir, WorkspaceConfigFile), 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}
	if err := u.AddSecret("github"); err == nil {
		t.Error("expected error when workspace.json is a directory, got nil")
	}
}

// TestWorkspaceUpdater_ReadConfig_NonExistError covers the non-NotExist branch
// in readConfig when the file path is occupied by a directory.
func TestWorkspaceUpdater_ReadConfig_NonExistError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, WorkspaceConfigFile)
	if err := os.MkdirAll(configPath, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := &workspaceConfigUpdater{configDir: dir}
	if _, err := w.readConfig(configPath); err == nil {
		t.Error("expected error for directory-as-file, got nil")
	}
}

// TestWorkspaceUpdater_ReadConfig_InvalidJSON covers the JSON unmarshal error
// branch in readConfig.
func TestWorkspaceUpdater_ReadConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, WorkspaceConfigFile)
	if err := os.WriteFile(configPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := &workspaceConfigUpdater{configDir: dir}
	if _, err := w.readConfig(configPath); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestWorkspaceUpdater_WriteConfig_MkdirAllFails covers the MkdirAll error
// branch in writeConfig by placing a regular file where the config dir is expected.
func TestWorkspaceUpdater_WriteConfig_MkdirAllFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a regular file at the path where the configDir would go.
	if err := os.WriteFile(filepath.Join(dir, "kaiden"), []byte("file"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := &workspaceConfigUpdater{configDir: filepath.Join(dir, "kaiden")}
	configPath := filepath.Join(dir, "kaiden", WorkspaceConfigFile)
	if err := w.writeConfig(configPath, &workspace.WorkspaceConfiguration{}); err == nil {
		t.Error("expected error when config path is a file, got nil")
	}
}

// TestWorkspaceUpdater_WriteConfig_WriteFileFails covers the WriteFile error
// branch in writeConfig by making the config directory read-only.
func TestWorkspaceUpdater_WriteConfig_WriteFileFails(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission tests do not apply on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("chmod restrictions do not apply to root")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("setup chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	w := &workspaceConfigUpdater{configDir: dir}
	configPath := filepath.Join(dir, WorkspaceConfigFile)
	if err := w.writeConfig(configPath, &workspace.WorkspaceConfiguration{}); err == nil {
		t.Error("expected error writing to read-only directory, got nil")
	}
}

// TestWorkspaceUpdater_EmptyFile_TreatedAsMissing verifies that a zero-byte
// workspace.json is treated as an empty config rather than returning a JSON error.
func TestWorkspaceUpdater_EmptyFile_TreatedAsMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFile), []byte{}, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	if err := u.AddSecret("github"); err != nil {
		t.Fatalf("AddSecret on empty file: %v", err)
	}

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 1 || secrets[0] != "github" {
		t.Errorf("expected [github] after adding to empty file, got %v", secrets)
	}
}
