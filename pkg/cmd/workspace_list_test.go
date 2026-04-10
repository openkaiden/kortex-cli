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

		c := &workspaceListCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}
	})

	t.Run("accepts no output flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceListCmd{
			output: "",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.output != "" {
			t.Errorf("Expected output to be empty, got %s", c.output)
		}
	})

	t.Run("accepts valid output flag with json", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceListCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{}

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

		c := &workspaceListCmd{
			output: "xml",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid output format")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
	})

	t.Run("rejects invalid output format yaml", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceListCmd{
			output: "yaml",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid output format")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
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

		c := &workspaceListCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.Flags().String("storage", invalidStorage, "test storage flag")

		args := []string{}

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
		// Check for table headers (uppercase)
		if !strings.Contains(result, "NAME") {
			t.Errorf("Expected output to contain table header 'NAME', got: %s", result)
		}
		if !strings.Contains(result, "SHORT ID") {
			t.Errorf("Expected output to contain table header 'SHORT ID', got: %s", result)
		}
		if !strings.Contains(result, "PROJECT") {
			t.Errorf("Expected output to contain table header 'PROJECT', got: %s", result)
		}
		if !strings.Contains(result, "SOURCES") {
			t.Errorf("Expected output to contain table header 'SOURCES', got: %s", result)
		}
		if !strings.Contains(result, "AGENT") {
			t.Errorf("Expected output to contain table header 'AGENT', got: %s", result)
		}
		if !strings.Contains(result, "STATE") {
			t.Errorf("Expected output to contain table header 'STATE', got: %s", result)
		}
		// Check for workspace data in table
		if !strings.Contains(result, addedInstance.GetName()) {
			t.Errorf("Expected output to contain workspace name %q, got: %s", addedInstance.GetName(), result)
		}
		shortID := addedInstance.GetID()[:12]
		if !strings.Contains(result, shortID) {
			t.Errorf("Expected output to contain short ID %q, got: %s", shortID, result)
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

		addedInstance1, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance 1: %v", err)
		}

		addedInstance2, err := manager.Add(context.Background(), instances.AddOptions{Instance: instance2, RuntimeType: "fake"})
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
		// Check for table headers (uppercase)
		if !strings.Contains(result, "NAME") {
			t.Errorf("Expected output to contain table header 'NAME', got: %s", result)
		}
		if !strings.Contains(result, "SHORT ID") {
			t.Errorf("Expected output to contain table header 'SHORT ID', got: %s", result)
		}
		// Check for both workspace names in table
		if !strings.Contains(result, addedInstance1.GetName()) {
			t.Errorf("Expected output to contain workspace name %q, got: %s", addedInstance1.GetName(), result)
		}
		if !strings.Contains(result, addedInstance2.GetName()) {
			t.Errorf("Expected output to contain workspace name %q, got: %s", addedInstance2.GetName(), result)
		}
		// Check for short IDs
		shortID1 := addedInstance1.GetID()[:12]
		if !strings.Contains(result, shortID1) {
			t.Errorf("Expected output to contain short ID %q, got: %s", shortID1, result)
		}
		shortID2 := addedInstance2.GetID()[:12]
		if !strings.Contains(result, shortID2) {
			t.Errorf("Expected output to contain short ID %q, got: %s", shortID2, result)
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
		// Check for table headers (uppercase)
		if !strings.Contains(result, "NAME") {
			t.Errorf("Expected output to contain table header 'NAME', got: %s", result)
		}
		if !strings.Contains(result, "SHORT ID") {
			t.Errorf("Expected output to contain table header 'SHORT ID', got: %s", result)
		}
		// Check for workspace data in table
		if !strings.Contains(result, addedInstance.GetName()) {
			t.Errorf("Expected output to contain workspace name %q, got: %s", addedInstance.GetName(), result)
		}
		shortID := addedInstance.GetID()[:12]
		if !strings.Contains(result, shortID) {
			t.Errorf("Expected output to contain short ID %q, got: %s", shortID, result)
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
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
			Name:      "test-workspace",
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
		expectedProject := addedInstance.GetProject()
		if workspace.Project != expectedProject {
			t.Errorf("Expected Project %s, got %s", expectedProject, workspace.Project)
		}
		if workspace.Paths.Source != addedInstance.GetSourceDir() {
			t.Errorf("Expected Source %s, got %s", addedInstance.GetSourceDir(), workspace.Paths.Source)
		}
		if workspace.Paths.Configuration != addedInstance.GetConfigDir() {
			t.Errorf("Expected Configuration %s, got %s", addedInstance.GetConfigDir(), workspace.Paths.Configuration)
		}
	})
}

func TestWorkspaceListCmd_Model(t *testing.T) {
	t.Parallel()

	t.Run("table header shows AGENT/MODEL", func(t *testing.T) {
		t.Parallel()

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

		_, err = manager.Add(context.Background(), instances.AddOptions{Instance: instance, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, "AGENT/MODEL") {
			t.Errorf("Expected table header 'AGENT/MODEL', got: %s", result)
		}
	})

	t.Run("shows agent/model in table when model is set", func(t *testing.T) {
		t.Parallel()

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

		_, err = manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "claude",
			Model:       "claude-sonnet-4-20250514",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, "claude/claude-sonnet-4-20250514") {
			t.Errorf("Expected 'claude/claude-sonnet-4-20250514' in table, got: %s", result)
		}
	})

	t.Run("shows only agent name in table when model is not set", func(t *testing.T) {
		t.Parallel()

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

		_, err = manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "claude",
			// Model intentionally omitted
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, "claude") {
			t.Errorf("Expected 'claude' in table, got: %s", result)
		}
		if strings.Contains(result, "claude/") {
			t.Errorf("Expected no slash after agent name when model is empty, got: %s", result)
		}
	})

	t.Run("json output includes model field when model is set", func(t *testing.T) {
		t.Parallel()

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

		_, err = manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "claude",
			Model:       "claude-sonnet-4-20250514",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "json"})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		var workspacesList api.WorkspacesList
		if err := json.Unmarshal(output.Bytes(), &workspacesList); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output.String())
		}

		if len(workspacesList.Items) != 1 {
			t.Fatalf("Expected 1 item, got %d", len(workspacesList.Items))
		}

		ws := workspacesList.Items[0]
		if ws.Model == nil {
			t.Fatal("Expected Model to be set in JSON output, got nil")
		}
		if *ws.Model != "claude-sonnet-4-20250514" {
			t.Errorf("Expected Model %q, got %q", "claude-sonnet-4-20250514", *ws.Model)
		}
	})

	t.Run("json output omits model field when model is not set", func(t *testing.T) {
		t.Parallel()

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

		_, err = manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			// Model intentionally omitted
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "json"})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		var workspacesList api.WorkspacesList
		if err := json.Unmarshal(output.Bytes(), &workspacesList); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output.String())
		}

		if len(workspacesList.Items) != 1 {
			t.Fatalf("Expected 1 item, got %d", len(workspacesList.Items))
		}

		if workspacesList.Items[0].Model != nil {
			t.Errorf("Expected Model to be nil in JSON output, got %q", *workspacesList.Items[0].Model)
		}
	})
}

func TestWorkspaceListCmd_Examples(t *testing.T) {
	t.Parallel()

	// Get the workspace list command
	listCmd := NewWorkspaceListCmd()

	// Verify Example field is not empty
	if listCmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	// Parse the examples
	commands, err := testutil.ParseExampleCommands(listCmd.Example)
	if err != nil {
		t.Fatalf("Failed to parse examples: %v", err)
	}

	// Verify we have the expected number of examples
	expectedCount := 3
	if len(commands) != expectedCount {
		t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
	}

	// Validate all examples against the root command
	rootCmd := NewRootCmd()
	err = testutil.ValidateCommandExamples(rootCmd, listCmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}
