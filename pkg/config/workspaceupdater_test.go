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

	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}
	if err := u.AddSecret("github"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}
	if err := u.AddSecret("github"); err != nil {
		t.Fatalf("AddSecret (duplicate): %v", err)
	}

	secrets := readWorkspaceSecrets(t, dir)
	if len(secrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(secrets))
	}
}

func TestWorkspaceUpdater_MultipleCalls_Accumulate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}
	if err := u.AddSecret("github"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}
	if err := u.AddSecret("anthropic"); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

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

func TestWorkspaceUpdater_AddEnvVar_CreatesFile(t *testing.T) {
	t.Parallel()

	u, _ := NewWorkspaceConfigUpdater(t.TempDir())
	if err := u.AddEnvVar("MY_VAR", "hello"); err != nil {
		t.Fatalf("AddEnvVar: %v", err)
	}

	dir := u.(*workspaceConfigUpdater).configDir
	data, err := os.ReadFile(filepath.Join(dir, WorkspaceConfigFile))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Environment == nil || len(*cfg.Environment) != 1 {
		t.Fatalf("expected 1 env var, got %v", cfg.Environment)
	}
	if (*cfg.Environment)[0].Name != "MY_VAR" || *(*cfg.Environment)[0].Value != "hello" {
		t.Errorf("unexpected env var: %+v", (*cfg.Environment)[0])
	}
}

func TestWorkspaceUpdater_AddEnvVar_UpdatesExisting(t *testing.T) {
	t.Parallel()

	u, _ := NewWorkspaceConfigUpdater(t.TempDir())
	if err := u.AddEnvVar("MY_VAR", "old"); err != nil {
		t.Fatalf("first AddEnvVar: %v", err)
	}
	if err := u.AddEnvVar("MY_VAR", "new"); err != nil {
		t.Fatalf("second AddEnvVar: %v", err)
	}

	dir := u.(*workspaceConfigUpdater).configDir
	data, _ := os.ReadFile(filepath.Join(dir, WorkspaceConfigFile))
	var cfg workspace.WorkspaceConfiguration
	_ = json.Unmarshal(data, &cfg)
	if len(*cfg.Environment) != 1 {
		t.Errorf("expected 1 env var (no duplicate), got %d", len(*cfg.Environment))
	}
	if *(*cfg.Environment)[0].Value != "new" {
		t.Errorf("expected value=new, got %q", *(*cfg.Environment)[0].Value)
	}
}

func TestWorkspaceUpdater_AddMount_CreatesFile(t *testing.T) {
	t.Parallel()

	u, _ := NewWorkspaceConfigUpdater(t.TempDir())
	if err := u.AddMount("$HOME/.foo", "$HOME/.foo", true); err != nil {
		t.Fatalf("AddMount: %v", err)
	}

	dir := u.(*workspaceConfigUpdater).configDir
	data, _ := os.ReadFile(filepath.Join(dir, WorkspaceConfigFile))
	var cfg workspace.WorkspaceConfiguration
	_ = json.Unmarshal(data, &cfg)
	if cfg.Mounts == nil || len(*cfg.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %v", cfg.Mounts)
	}
	m := (*cfg.Mounts)[0]
	if m.Host != "$HOME/.foo" || m.Target != "$HOME/.foo" {
		t.Errorf("unexpected mount: %+v", m)
	}
	if m.Ro == nil || !*m.Ro {
		t.Error("expected read-only mount")
	}
}

func TestWorkspaceUpdater_AddMount_Idempotent(t *testing.T) {
	t.Parallel()

	u, _ := NewWorkspaceConfigUpdater(t.TempDir())
	if err := u.AddMount("$HOME/.foo", "$HOME/.foo", true); err != nil {
		t.Fatalf("first AddMount: %v", err)
	}
	if err := u.AddMount("$HOME/.foo", "$HOME/.foo", true); err != nil {
		t.Fatalf("second AddMount: %v", err)
	}

	dir := u.(*workspaceConfigUpdater).configDir
	data, _ := os.ReadFile(filepath.Join(dir, WorkspaceConfigFile))
	var cfg workspace.WorkspaceConfiguration
	_ = json.Unmarshal(data, &cfg)
	if len(*cfg.Mounts) != 1 {
		t.Errorf("expected 1 mount after duplicate, got %d", len(*cfg.Mounts))
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

func readWorkspacePorts(t *testing.T, configDir string) []int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, WorkspaceConfigFile))
	if err != nil {
		t.Fatalf("failed to read workspace.json: %v", err)
	}
	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse workspace.json: %v", err)
	}
	if cfg.Ports == nil {
		return nil
	}
	return *cfg.Ports
}

func readWorkspaceFeatures(t *testing.T, configDir string) map[string]map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, WorkspaceConfigFile))
	if err != nil {
		t.Fatalf("failed to read workspace.json: %v", err)
	}
	var cfg workspace.WorkspaceConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse workspace.json: %v", err)
	}
	if cfg.Features == nil {
		return nil
	}
	return *cfg.Features
}

func TestWorkspaceUpdater_AddPort_CreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	if err := u.AddPort(8080); err != nil {
		t.Fatalf("AddPort: %v", err)
	}

	ports := readWorkspacePorts(t, dir)
	if len(ports) != 1 || ports[0] != 8080 {
		t.Errorf("expected [8080], got %v", ports)
	}
}

func TestWorkspaceUpdater_AddPort_Idempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	if err := u.AddPort(3000); err != nil {
		t.Fatalf("first AddPort: %v", err)
	}
	if err := u.AddPort(3000); err != nil {
		t.Fatalf("second AddPort: %v", err)
	}

	ports := readWorkspacePorts(t, dir)
	if len(ports) != 1 {
		t.Errorf("expected exactly 1 port after duplicate AddPort, got %v", ports)
	}
}

func TestWorkspaceUpdater_AddPort_Accumulates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	for _, p := range []int{8080, 3000, 5000} {
		if err := u.AddPort(p); err != nil {
			t.Fatalf("AddPort(%d): %v", p, err)
		}
	}

	ports := readWorkspacePorts(t, dir)
	if len(ports) != 3 {
		t.Errorf("expected 3 ports, got %v", ports)
	}
}

func TestWorkspaceUpdater_AddFeature_CreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	if err := u.AddFeature("ghcr.io/devcontainers/features/go:1", map[string]interface{}{}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}

	features := readWorkspaceFeatures(t, dir)
	if _, ok := features["ghcr.io/devcontainers/features/go:1"]; !ok {
		t.Errorf("expected go feature in map, got %v", features)
	}
}

func TestWorkspaceUpdater_AddFeature_Idempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	opts := map[string]interface{}{"version": "1.21"}
	if err := u.AddFeature("ghcr.io/devcontainers/features/go:1", opts); err != nil {
		t.Fatalf("first AddFeature: %v", err)
	}
	// Second call with different options — must not overwrite (idempotent).
	if err := u.AddFeature("ghcr.io/devcontainers/features/go:1", map[string]interface{}{}); err != nil {
		t.Fatalf("second AddFeature: %v", err)
	}

	features := readWorkspaceFeatures(t, dir)
	if len(features) != 1 {
		t.Errorf("expected exactly 1 feature after duplicate AddFeature, got %v", features)
	}
}

func TestWorkspaceUpdater_AddFeature_Accumulates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	u, err := NewWorkspaceConfigUpdater(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceConfigUpdater: %v", err)
	}

	featureIDs := []string{
		"ghcr.io/devcontainers/features/go:1",
		"ghcr.io/devcontainers/features/python:1",
	}
	for _, id := range featureIDs {
		if err := u.AddFeature(id, map[string]interface{}{}); err != nil {
			t.Fatalf("AddFeature(%s): %v", id, err)
		}
	}

	features := readWorkspaceFeatures(t, dir)
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %v", features)
	}
	for _, id := range featureIDs {
		if _, ok := features[id]; !ok {
			t.Errorf("feature %q not found in %v", id, features)
		}
	}
}
