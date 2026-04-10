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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/spf13/cobra"
)

func TestWorkspaceRemoveCmd(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceRemoveCmd()
	if cmd == nil {
		t.Fatal("NewWorkspaceRemoveCmd() returned nil")
	}

	if cmd.Use != "remove NAME|ID" {
		t.Errorf("Expected Use to be 'remove NAME|ID', got '%s'", cmd.Use)
	}
}

func TestWorkspaceRemoveCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("extracts id from args and creates manager", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceRemoveCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{"test-workspace-id"}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.nameOrID != "test-workspace-id" {
			t.Errorf("Expected id to be 'test-workspace-id', got %s", c.nameOrID)
		}
	})

	t.Run("accepts empty output flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceRemoveCmd{
			output: "",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{"test-id"}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.output != "" {
			t.Errorf("Expected output to be empty, got %s", c.output)
		}
	})

	t.Run("accepts json output format", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceRemoveCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{"test-id"}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.output != "json" {
			t.Errorf("Expected output to be 'json', got %s", c.output)
		}
	})

	t.Run("rejects invalid output format", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceRemoveCmd{
			output: "yaml",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{"test-id"}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid output format")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
		if !strings.Contains(err.Error(), "yaml") {
			t.Errorf("Expected error to mention 'yaml', got: %v", err)
		}
	})

	t.Run("rejects --show-logs with --output json", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceRemoveCmd{
			output:   "json",
			showLogs: true,
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		err := c.preRun(cmd, []string{"test-id"})
		if err == nil {
			t.Fatal("Expected preRun() to fail when --show-logs used with --output json")
		}

		if !strings.Contains(err.Error(), "--show-logs") {
			t.Errorf("Expected error to mention '--show-logs', got: %v", err)
		}
	})

	t.Run("outputs JSON error when manager creation fails with json output", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		// Create a file and try to use it as a parent directory - will fail cross-platform
		notADir := filepath.Join(tempDir, "file")
		if err := os.WriteFile(notADir, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		invalidStorage := filepath.Join(notADir, "subdir")

		c := &workspaceRemoveCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.Flags().String("storage", invalidStorage, "test storage flag")

		args := []string{"test-id"}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid storage path")
		}

		// Verify JSON error was output
		var errorResponse api.Error
		if jsonErr := json.Unmarshal(buf.Bytes(), &errorResponse); jsonErr != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v\nOutput was: %s", jsonErr, buf.String())
		}

		if !strings.Contains(errorResponse.Error, "failed to create manager") {
			t.Errorf("Expected error to contain 'failed to create manager', got: %s", errorResponse.Error)
		}
	})
}

func TestWorkspaceRemoveCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("requires ID argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", "--storage", storageDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when ID argument is missing, got nil")
		}

		if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
			t.Errorf("Expected error to contain 'accepts 1 arg(s), received 0', got: %v", err)
		}
	})

	t.Run("rejects multiple arguments", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", "id1", "id2", "--storage", storageDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when multiple arguments provided, got nil")
		}

		if !strings.Contains(err.Error(), "accepts 1 arg(s), received 2") {
			t.Errorf("Expected error to contain 'accepts 1 arg(s), received 2', got: %v", err)
		}
	})

	t.Run("creates manager from storage flag", func(t *testing.T) {
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
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now remove it
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", addedInstance.GetID(), "--storage", storageDir})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("removes existing workspace by ID", func(t *testing.T) {
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
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		instanceID := addedInstance.GetID()

		// Remove the workspace
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", instanceID, "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify output is only the ID
		result := strings.TrimSpace(output.String())
		if result != instanceID {
			t.Errorf("Expected output to be '%s', got: '%s'", instanceID, result)
		}

		// Verify workspace is removed from storage
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances after removal, got %d", len(instancesList))
		}

		// Verify Get returns not found
		_, err = manager.Get(instanceID)
		if err == nil {
			t.Error("Expected error when getting removed instance, got nil")
		}
		if err != instances.ErrInstanceNotFound {
			t.Errorf("Expected ErrInstanceNotFound, got: %v", err)
		}
	})

	t.Run("returns error for non-existent workspace ID", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", "nonexistent-id", "--storage", storageDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error for non-existent ID, got nil")
		}

		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("Expected error to contain 'workspace not found', got: %v", err)
		}
		if !strings.Contains(err.Error(), "workspace list") {
			t.Errorf("Expected error to contain 'workspace list', got: %v", err)
		}
	})

	t.Run("removes only specified workspace when multiple exist", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
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
			ConfigDir: filepath.Join(sourcesDir1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 1: %v", err)
		}

		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir2,
			ConfigDir: filepath.Join(sourcesDir2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 2: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance1, err := manager.Add(ctx, instances.AddOptions{Instance: instance1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance 1: %v", err)
		}

		addedInstance2, err := manager.Add(ctx, instances.AddOptions{Instance: instance2, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance 2: %v", err)
		}

		// Remove the first workspace
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", addedInstance1.GetID(), "--storage", storageDir})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify only one workspace remains
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance after removal, got %d", len(instancesList))
		}

		// Verify the remaining workspace is instance2
		if instancesList[0].GetID() != addedInstance2.GetID() {
			t.Errorf("Expected remaining instance ID %s, got %s", addedInstance2.GetID(), instancesList[0].GetID())
		}

		// Verify instance1 is removed
		_, err = manager.Get(addedInstance1.GetID())
		if err != instances.ErrInstanceNotFound {
			t.Errorf("Expected ErrInstanceNotFound for removed instance, got: %v", err)
		}

		// Verify instance2 still exists
		retrievedInstance, err := manager.Get(addedInstance2.GetID())
		if err != nil {
			t.Fatalf("Expected no error getting instance 2, got %v", err)
		}
		if retrievedInstance.GetID() != addedInstance2.GetID() {
			t.Errorf("Expected ID %s, got %s", addedInstance2.GetID(), retrievedInstance.GetID())
		}
	})

	t.Run("remove command alias works", func(t *testing.T) {
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
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		instanceID := addedInstance.GetID()

		// Use the alias command 'remove' instead of 'workspace remove'
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"remove", instanceID, "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify output is only the ID
		result := strings.TrimSpace(output.String())
		if result != instanceID {
			t.Errorf("Expected output to be '%s', got: '%s'", instanceID, result)
		}

		// Verify workspace is removed
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances after removal, got %d", len(instancesList))
		}
	})

	t.Run("json output returns workspace ID", func(t *testing.T) {
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
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		instanceID := addedInstance.GetID()

		// Remove with JSON output
		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"workspace", "remove", instanceID, "--storage", storageDir, "--output", "json"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Parse JSON output
		var workspaceId api.WorkspaceId
		if err := json.Unmarshal(buf.Bytes(), &workspaceId); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify ID matches
		if workspaceId.Id != instanceID {
			t.Errorf("Expected ID %s in JSON output, got %s", instanceID, workspaceId.Id)
		}

		// Verify only ID field exists
		var parsed map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal to map: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field in JSON, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["id"]; !exists {
			t.Error("Expected 'id' field in JSON")
		}

		// Verify workspace is actually removed
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances after removal, got %d", len(instancesList))
		}
	})

	t.Run("json output error for non-existent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"workspace", "remove", "invalid-id", "--storage", storageDir, "--output", "json"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected Execute() to fail with non-existent workspace")
		}

		// Parse JSON error output
		var errorResponse api.Error
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v", err)
		}

		// Verify error message
		if !strings.Contains(errorResponse.Error, "workspace not found") {
			t.Errorf("Expected error to contain 'workspace not found', got: %s", errorResponse.Error)
		}

		if !strings.Contains(errorResponse.Error, "invalid-id") {
			t.Errorf("Expected error to contain 'invalid-id', got: %s", errorResponse.Error)
		}

		// Verify only error field exists
		var parsed map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal to map: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field in error JSON, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["error"]; !exists {
			t.Error("Expected 'error' field in JSON")
		}
	})

	t.Run("json output removes correct workspace when multiple exist", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
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
			ConfigDir: filepath.Join(sourcesDir1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 1: %v", err)
		}

		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir2,
			ConfigDir: filepath.Join(sourcesDir2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 2: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance1, err := manager.Add(ctx, instances.AddOptions{Instance: instance1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance 1: %v", err)
		}

		addedInstance2, err := manager.Add(ctx, instances.AddOptions{Instance: instance2, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance 2: %v", err)
		}

		// Remove first workspace with JSON output
		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"workspace", "remove", addedInstance1.GetID(), "--storage", storageDir, "--output", "json"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Parse JSON output
		var workspaceId api.WorkspaceId
		if err := json.Unmarshal(buf.Bytes(), &workspaceId); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify correct ID was removed
		if workspaceId.Id != addedInstance1.GetID() {
			t.Errorf("Expected ID %s in JSON output, got %s", addedInstance1.GetID(), workspaceId.Id)
		}

		// Verify only instance2 remains
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance after removal, got %d", len(instancesList))
		}

		if instancesList[0].GetID() != addedInstance2.GetID() {
			t.Errorf("Expected remaining instance ID %s, got %s", addedInstance2.GetID(), instancesList[0].GetID())
		}
	})
}

func TestWorkspaceRemoveCmd_Force(t *testing.T) {
	t.Parallel()

	t.Run("removes running workspace with --force flag", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(ctx, instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Start the instance so it is running
		if err := manager.Start(ctx, addedInstance.GetID()); err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		// Remove with --force
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", addedInstance.GetID(), "--storage", storageDir, "--force"})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error with --force on running workspace, got %v", err)
		}

		// Verify output is the instance ID
		result := strings.TrimSpace(output.String())
		if result != addedInstance.GetID() {
			t.Errorf("Expected output to be '%s', got: '%s'", addedInstance.GetID(), result)
		}

		// Verify workspace is removed
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}
		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances after removal, got %d", len(instancesList))
		}
	})

	t.Run("fails to remove running workspace without --force flag", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		addedInstance, err := manager.Add(ctx, instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Start the instance so it is running
		if err := manager.Start(ctx, addedInstance.GetID()); err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		// Try to remove without --force
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "remove", addedInstance.GetID(), "--storage", storageDir})

		err = rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when removing running workspace without --force, got nil")
		}

		// Verify workspace is still present
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}
		if len(instancesList) != 1 {
			t.Errorf("Expected 1 instance (workspace should not be removed), got %d", len(instancesList))
		}
	})
}

func TestWorkspaceRemoveCmd_Examples(t *testing.T) {
	t.Parallel()

	// Get the workspace remove command
	removeCmd := NewWorkspaceRemoveCmd()

	// Verify Example field is not empty
	if removeCmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	// Parse the examples
	commands, err := testutil.ParseExampleCommands(removeCmd.Example)
	if err != nil {
		t.Fatalf("Failed to parse examples: %v", err)
	}

	// Verify we have the expected number of examples
	expectedCount := 4
	if len(commands) != expectedCount {
		t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
	}

	// Validate all examples against the root command
	rootCmd := NewRootCmd()
	err = testutil.ValidateCommandExamples(rootCmd, removeCmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}
