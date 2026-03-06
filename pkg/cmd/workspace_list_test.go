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
	"path/filepath"
	"strings"
	"testing"

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

		instance, err := instances.NewInstance(sourcesDir, filepath.Join(sourcesDir, ".kortex"))
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

		instance1, err := instances.NewInstance(sourcesDir1, filepath.Join(sourcesDir1, ".kortex"))
		if err != nil {
			t.Fatalf("Failed to create instance 1: %v", err)
		}

		instance2, err := instances.NewInstance(sourcesDir2, filepath.Join(sourcesDir2, ".kortex"))
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

		instance, err := instances.NewInstance(sourcesDir, filepath.Join(sourcesDir, ".kortex"))
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
}
