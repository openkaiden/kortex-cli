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

package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
)

func TestWorkspaceListCmd(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceListCmd()
	if cmd == nil {
		t.Fatal("NewWorkspaceListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Expected Use to be 'list', got '%s'", cmd.Use)
	}
}

func TestWorkspaceListCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("creates manager from storage flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		// Execute to trigger preRun
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("accepts no output flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error with no output flag, got %v", err)
		}
	})

	t.Run("accepts valid output flag with json", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "--output", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error with --output json, got %v", err)
		}
	})

	t.Run("accepts valid output flag with -o json", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error with -o json, got %v", err)
		}
	})

	t.Run("rejects invalid output format", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "--output", "xml"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error with invalid output format, got nil")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
	})

	t.Run("rejects invalid output format with short flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "yaml"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error with invalid output format, got nil")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
	})
}

func TestWorkspaceListCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("shows no workspaces message when empty", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, "No workspaces registered") {
			t.Errorf("Expected 'No workspaces registered' message, got: %s", result)
		}
	})

	t.Run("lists single workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Create a workspace first
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		addedInstance, err := manager.Add(instance)
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now list workspaces
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, addedInstance.GetID()) {
			t.Errorf("Expected output to contain ID %s, got: %s", addedInstance.GetID(), result)
		}
		if !strings.Contains(result, sourcesDir) {
			t.Errorf("Expected output to contain sources dir %s, got: %s", sourcesDir, result)
		}
	})

	t.Run("lists multiple workspaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir1 := t.TempDir()
		sourcesDir2 := t.TempDir()

		// Create two workspaces
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir1,
			ConfigDir: filepath.Join(sourcesDir1, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 1: %v", err)
		}

		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir2,
			ConfigDir: filepath.Join(sourcesDir2, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 2: %v", err)
		}

		addedInstance1, err := manager.Add(instance1)
		if err != nil {
			t.Fatalf("Failed to add instance 1: %v", err)
		}

		addedInstance2, err := manager.Add(instance2)
		if err != nil {
			t.Fatalf("Failed to add instance 2: %v", err)
		}

		// Now list workspaces
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, addedInstance1.GetID()) {
			t.Errorf("Expected output to contain ID %s, got: %s", addedInstance1.GetID(), result)
		}
		if !strings.Contains(result, addedInstance2.GetID()) {
			t.Errorf("Expected output to contain ID %s, got: %s", addedInstance2.GetID(), result)
		}
		if !strings.Contains(result, sourcesDir1) {
			t.Errorf("Expected output to contain sources dir %s, got: %s", sourcesDir1, result)
		}
		if !strings.Contains(result, sourcesDir2) {
			t.Errorf("Expected output to contain sources dir %s, got: %s", sourcesDir2, result)
		}
	})

	t.Run("list command alias works", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Create a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		addedInstance, err := manager.Add(instance)
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Use the alias command 'list' instead of 'workspace list'
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, addedInstance.GetID()) {
			t.Errorf("Expected output to contain ID %s, got: %s", addedInstance.GetID(), result)
		}
	})

	t.Run("outputs JSON with empty list", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "json"})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Parse JSON output
		var workspacesList api.WorkspacesList
		err = json.Unmarshal(output.Bytes(), &workspacesList)
		if err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output.String())
		}

		// Verify empty items array
		if workspacesList.Items == nil {
			t.Error("Expected Items to be non-nil")
		}
		if len(workspacesList.Items) != 0 {
			t.Errorf("Expected 0 items, got %d", len(workspacesList.Items))
		}
	})

	t.Run("outputs JSON with single workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Create a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kortex"),
			Name:      "test-workspace",
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		addedInstance, err := manager.Add(instance)
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// List workspaces with JSON output
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "json"})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Parse JSON output
		var workspacesList api.WorkspacesList
		err = json.Unmarshal(output.Bytes(), &workspacesList)
		if err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output.String())
		}

		// Verify structure
		if len(workspacesList.Items) != 1 {
			t.Fatalf("Expected 1 item, got %d", len(workspacesList.Items))
		}

		workspace := workspacesList.Items[0]

		// Verify all fields
		if workspace.Id != addedInstance.GetID() {
			t.Errorf("Expected ID %s, got %s", addedInstance.GetID(), workspace.Id)
		}
		if workspace.Name != addedInstance.GetName() {
			t.Errorf("Expected Name %s, got %s", addedInstance.GetName(), workspace.Name)
		}
		if workspace.Paths.Source != addedInstance.GetSourceDir() {
			t.Errorf("Expected Source %s, got %s", addedInstance.GetSourceDir(), workspace.Paths.Source)
		}
		if workspace.Paths.Configuration != addedInstance.GetConfigDir() {
			t.Errorf("Expected Configuration %s, got %s", addedInstance.GetConfigDir(), workspace.Paths.Configuration)
		}
	})

}
