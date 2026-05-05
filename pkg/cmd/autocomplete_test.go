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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/spf13/cobra"
)

func TestCompleteNonRunningWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("returns only non-running workspace IDs", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		// Create manager and add some instances
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		// Add first instance
		sourceDir1 := t.TempDir()
		configDir1 := filepath.Join(sourceDir1, ".kaiden")
		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir1,
			ConfigDir: configDir1,
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		addedInstance1, err := manager.Add(ctx, instances.AddOptions{
			Instance:    instance1,
			RuntimeType: "fake",
		})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}

		// Add second instance
		sourceDir2 := t.TempDir()
		configDir2 := filepath.Join(sourceDir2, ".kaiden")
		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir2,
			ConfigDir: configDir2,
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		addedInstance2, err := manager.Add(ctx, instances.AddOptions{
			Instance:    instance2,
			RuntimeType: "fake",
		})
		if err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		// Start instance1 to make it "running"
		err = manager.Start(ctx, addedInstance1.GetID())
		if err != nil {
			t.Fatalf("failed to start instance1: %v", err)
		}

		// Create a command with the storage flag
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// Call completion function - should only return non-running instances
		completions, directive := completeNonRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got instance2's ID and name (instance1 is running, so not included)
		// Should return both ID and name for better discoverability
		if len(completions) != 2 {
			t.Errorf("Expected 2 completions (ID and name for non-running), got %d", len(completions))
		}

		// Verify instance2 ID and name are in the completions
		expectedID := addedInstance2.GetID()
		expectedName := addedInstance2.GetName()
		foundID := false
		foundName := false
		for _, completion := range completions {
			if completion == expectedID {
				foundID = true
			}
			if completion == expectedName {
				foundName = true
			}
		}
		if !foundID {
			t.Errorf("Expected ID %s in completions, got %v", expectedID, completions)
		}
		if !foundName {
			t.Errorf("Expected name %s in completions, got %v", expectedName, completions)
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})

	t.Run("returns error directive when storage flag is missing", func(t *testing.T) {
		t.Parallel()

		// Create a command without the storage flag
		cmd := &cobra.Command{}

		// Call completion function
		completions, directive := completeNonRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got an error directive
		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("Expected ShellCompDirectiveError, got %v", directive)
		}

		// Verify we got no completions
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d", len(completions))
		}
	})

	t.Run("returns empty list when list fails", func(t *testing.T) {
		t.Parallel()

		// Use a non-existent storage directory
		// Manager creation will succeed but List() may fail on corrupted data
		tmpDir := t.TempDir()
		storageDir := filepath.Join(tmpDir, "not-found")

		// Create a command with the storage flag
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// Call completion function - should handle gracefully even if list fails
		completions, directive := completeNonRunningWorkspaceID(cmd, []string{}, "")

		// Should return empty list with no file completion
		// (If list fails, we return error directive; if list succeeds with no instances, we return empty)
		if len(completions) != 0 && directive != cobra.ShellCompDirectiveError {
			t.Errorf("Expected empty completions or error directive")
		}
	})

	t.Run("returns empty list when no instances exist", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// Create manager without adding any instances
		_, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Create a command with the storage flag
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// Call completion function
		completions, directive := completeNonRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got no completions
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d", len(completions))
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})
}

func TestCompleteRunningWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("returns only running workspace IDs", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		// Create manager and add some instances
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		// Add first instance
		sourceDir1 := t.TempDir()
		configDir1 := filepath.Join(sourceDir1, ".kaiden")
		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir1,
			ConfigDir: configDir1,
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		addedInstance1, err := manager.Add(ctx, instances.AddOptions{
			Instance:    instance1,
			RuntimeType: "fake",
		})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}

		// Add second instance (will remain non-running)
		sourceDir2 := t.TempDir()
		configDir2 := filepath.Join(sourceDir2, ".kaiden")
		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir2,
			ConfigDir: configDir2,
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		_, err = manager.Add(ctx, instances.AddOptions{
			Instance:    instance2,
			RuntimeType: "fake",
		})
		if err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		// Start instance1 to make it "running"
		err = manager.Start(ctx, addedInstance1.GetID())
		if err != nil {
			t.Fatalf("failed to start instance1: %v", err)
		}

		// Create a command with the storage flag
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// Call completion function - should only return running instances
		completions, directive := completeRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got instance1's ID and name (instance2 is not running, so not included)
		// Should return both ID and name for better discoverability
		if len(completions) != 2 {
			t.Errorf("Expected 2 completions (ID and name for running), got %d", len(completions))
		}

		// Verify instance1 ID and name are in the completions
		expectedID := addedInstance1.GetID()
		expectedName := addedInstance1.GetName()
		foundID := false
		foundName := false
		for _, completion := range completions {
			if completion == expectedID {
				foundID = true
			}
			if completion == expectedName {
				foundName = true
			}
		}
		if !foundID {
			t.Errorf("Expected ID %s in completions, got %v", expectedID, completions)
		}
		if !foundName {
			t.Errorf("Expected name %s in completions, got %v", expectedName, completions)
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})

	t.Run("returns error directive when storage flag is missing", func(t *testing.T) {
		t.Parallel()

		// Create a command without the storage flag
		cmd := &cobra.Command{}

		// Call completion function
		completions, directive := completeRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got an error directive
		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("Expected ShellCompDirectiveError, got %v", directive)
		}

		// Verify we got no completions
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d", len(completions))
		}
	})

	t.Run("returns empty list when no running instances exist", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		// Create manager and add a non-running instance
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		// Add instance but don't start it
		sourceDir := t.TempDir()
		configDir := filepath.Join(sourceDir, ".kaiden")
		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		_, err = manager.Add(ctx, instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
		})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}

		// Create a command with the storage flag
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// Call completion function
		completions, directive := completeRunningWorkspaceID(cmd, []string{}, "")

		// Verify we got no completions (instance is not running)
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d", len(completions))
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})
}

func TestNewOutputFlagCompletion(t *testing.T) {
	t.Parallel()

	t.Run("returns configured output formats", func(t *testing.T) {
		t.Parallel()

		// Create completion function with multiple formats
		completionFunc := newOutputFlagCompletion([]string{"json", "yaml", "text"})

		cmd := &cobra.Command{}
		completions, directive := completionFunc(cmd, []string{}, "")

		// Verify we got all formats
		if len(completions) != 3 {
			t.Errorf("Expected 3 completions, got %d", len(completions))
		}

		expectedFormats := map[string]bool{"json": true, "yaml": true, "text": true}
		for _, completion := range completions {
			if !expectedFormats[completion] {
				t.Errorf("Unexpected completion: %s", completion)
			}
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})

	t.Run("returns only json for current commands", func(t *testing.T) {
		t.Parallel()

		// Create completion function with only json (current state)
		completionFunc := newOutputFlagCompletion([]string{"json"})

		cmd := &cobra.Command{}
		completions, directive := completionFunc(cmd, []string{}, "")

		// Verify we got only json
		if len(completions) != 1 {
			t.Errorf("Expected 1 completion, got %d", len(completions))
		}

		if len(completions) > 0 && completions[0] != "json" {
			t.Errorf("Expected 'json', got %s", completions[0])
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})

	t.Run("returns empty list when no formats configured", func(t *testing.T) {
		t.Parallel()

		// Create completion function with empty list
		completionFunc := newOutputFlagCompletion([]string{})

		cmd := &cobra.Command{}
		completions, directive := completionFunc(cmd, []string{}, "")

		// Verify we got no completions
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d", len(completions))
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})
}

func TestCompleteRemoveWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("without --force returns only non-running workspace IDs", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		sourceDir1 := t.TempDir()
		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir1,
			ConfigDir: filepath.Join(sourceDir1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		addedInstance1, err := manager.Add(ctx, instances.AddOptions{Instance: instance1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}

		sourceDir2 := t.TempDir()
		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir2,
			ConfigDir: filepath.Join(sourceDir2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		addedInstance2, err := manager.Add(ctx, instances.AddOptions{Instance: instance2, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		// Start instance1 so it is running
		if err := manager.Start(ctx, addedInstance1.GetID()); err != nil {
			t.Fatalf("failed to start instance1: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")
		cmd.Flags().Bool("force", false, "")

		completions, directive := completeRemoveWorkspaceID(cmd, []string{}, "")

		// Only instance2 (stopped) should appear
		if len(completions) != 2 {
			t.Errorf("Expected 2 completions (ID and name for non-running), got %d: %v", len(completions), completions)
		}

		for _, completion := range completions {
			if completion == addedInstance1.GetID() || completion == addedInstance1.GetName() {
				t.Errorf("Running instance should not appear in completions without --force, got %s", completion)
			}
		}

		foundID := false
		foundName := false
		for _, completion := range completions {
			if completion == addedInstance2.GetID() {
				foundID = true
			}
			if completion == addedInstance2.GetName() {
				foundName = true
			}
		}
		if !foundID {
			t.Errorf("Expected stopped instance ID %s in completions, got %v", addedInstance2.GetID(), completions)
		}
		if !foundName {
			t.Errorf("Expected stopped instance name %s in completions, got %v", addedInstance2.GetName(), completions)
		}

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})

	t.Run("with --force returns all workspace IDs including running", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		sourceDir1 := t.TempDir()
		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir1,
			ConfigDir: filepath.Join(sourceDir1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		addedInstance1, err := manager.Add(ctx, instances.AddOptions{Instance: instance1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}

		sourceDir2 := t.TempDir()
		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir2,
			ConfigDir: filepath.Join(sourceDir2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		addedInstance2, err := manager.Add(ctx, instances.AddOptions{Instance: instance2, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		// Start instance1 so it is running
		if err := manager.Start(ctx, addedInstance1.GetID()); err != nil {
			t.Fatalf("failed to start instance1: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")
		cmd.Flags().Bool("force", false, "")
		if err := cmd.Flags().Set("force", "true"); err != nil {
			t.Fatalf("failed to set --force flag: %v", err)
		}

		completions, directive := completeRemoveWorkspaceID(cmd, []string{}, "")

		// Both instances (ID + name each) should appear
		if len(completions) != 4 {
			t.Errorf("Expected 4 completions (ID and name for each instance), got %d: %v", len(completions), completions)
		}

		expected := []string{
			addedInstance1.GetID(), addedInstance1.GetName(),
			addedInstance2.GetID(), addedInstance2.GetName(),
		}
		completionSet := make(map[string]bool, len(completions))
		for _, c := range completions {
			completionSet[c] = true
		}
		for _, e := range expected {
			if !completionSet[e] {
				t.Errorf("Expected %s in completions, got %v", e, completions)
			}
		}

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})
}

func TestCompleteSecretName(t *testing.T) {
	t.Parallel()

	// writeSecretsJSON writes a minimal secrets.json so List() returns known names without
	// touching the keychain (only List() is called during completion — it reads metadata only).
	writeSecretsJSON := func(t *testing.T, dir string, names []string) {
		t.Helper()
		type record struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		type file struct {
			Secrets []record `json:"secrets"`
		}
		records := make([]record, 0, len(names))
		for _, n := range names {
			records = append(records, record{Name: n, Type: "github"})
		}
		data, err := json.Marshal(file{Secrets: records})
		if err != nil {
			t.Fatalf("failed to marshal secrets: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "secrets.json"), data, 0600); err != nil {
			t.Fatalf("failed to write secrets.json: %v", err)
		}
	}

	t.Run("returns names of stored secrets", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		writeSecretsJSON(t, storageDir, []string{"github-token", "api-key"})

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeSecretName(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 2 {
			t.Fatalf("expected 2 completions, got %d: %v", len(completions), completions)
		}
		found := map[string]bool{}
		for _, c := range completions {
			found[c] = true
		}
		for _, name := range []string{"github-token", "api-key"} {
			if !found[name] {
				t.Errorf("expected %q in completions, got %v", name, completions)
			}
		}
	})

	t.Run("returns no completions when storage directory does not exist", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", filepath.Join(t.TempDir(), "nonexistent"), "")

		completions, directive := completeSecretName(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 0 {
			t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns empty list when no secrets exist", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeSecretName(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 0 {
			t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns error directive when storage flag is not registered", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		// Intentionally omit registering the "storage" flag so GetString returns an error.

		_, directive := completeSecretName(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("expected ShellCompDirectiveError, got %v", directive)
		}
	})

	t.Run("returns error directive when secrets.json is corrupt", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(storageDir, "secrets.json"), []byte("not-json"), 0600); err != nil {
			t.Fatalf("failed to write corrupt secrets.json: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		_, directive := completeSecretName(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("expected ShellCompDirectiveError, got %v", directive)
		}
	})
}

func TestCompleteDashboardWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("returns running instances whose runtime supports Dashboard", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		sourceDir := t.TempDir()
		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(sourceDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		added, err := manager.Add(ctx, instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}
		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("failed to start instance: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 2 {
			t.Fatalf("Expected 2 completions (ID and name), got %d: %v", len(completions), completions)
		}
		completionSet := make(map[string]bool)
		for _, c := range completions {
			completionSet[c] = true
		}
		if !completionSet[added.GetID()] {
			t.Errorf("Expected ID %q in completions, got %v", added.GetID(), completions)
		}
		if !completionSet[added.GetName()] {
			t.Errorf("Expected name %q in completions, got %v", added.GetName(), completions)
		}
	})

	t.Run("excludes running instances whose runtime does not support Dashboard", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		sourceDir := t.TempDir()
		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(sourceDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		added, err := manager.Add(ctx, instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}
		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("failed to start instance: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return nil, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions (runtime not Dashboard-capable), got %d: %v", len(completions), completions)
		}
	})

	t.Run("excludes stopped instances even if runtime supports Dashboard", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		sourceDir := t.TempDir()
		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(sourceDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		_, err = manager.Add(ctx, instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions (instance is stopped), got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns error directive when storage flag is missing", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}

		_, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("Expected ShellCompDirectiveError, got %v", directive)
		}
	})

	t.Run("returns no suggestions when storage does not exist", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", filepath.Join(t.TempDir(), "nonexistent"), "")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 0 {
			t.Errorf("Expected 0 completions, got %d: %v", len(completions), completions)
		}
	})
}

func TestCompleteWorkspaceIDIgnoreIDs(t *testing.T) {
	// Cannot use t.Parallel() on the parent because subtests use t.Setenv.

	ctx := context.Background()

	setup := func(t *testing.T) (storageDir string, id1, name1, id2, name2 string) {
		t.Helper()
		storageDir = t.TempDir()
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		src1 := t.TempDir()
		inst1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src1,
			ConfigDir: filepath.Join(src1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		added1, err := manager.Add(ctx, instances.AddOptions{Instance: inst1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}

		src2 := t.TempDir()
		inst2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src2,
			ConfigDir: filepath.Join(src2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		added2, err := manager.Add(ctx, instances.AddOptions{Instance: inst2, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		return storageDir, added1.GetID(), added1.GetName(), added2.GetID(), added2.GetName()
	}

	t.Run("returns IDs and names when KDN_AUTOCOMPLETE_IGNORE_IDS is not set", func(t *testing.T) {
		t.Setenv("KDN_AUTOCOMPLETE_IGNORE_IDS", "")

		storageDir, id1, name1, id2, name2 := setup(t)
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeWorkspaceID(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 4 {
			t.Errorf("Expected 4 completions (ID+name for each), got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		for _, v := range []string{id1, name1, id2, name2} {
			if !set[v] {
				t.Errorf("Expected %q in completions, got %v", v, completions)
			}
		}
	})

	t.Run("returns only names when KDN_AUTOCOMPLETE_IGNORE_IDS is truthy", func(t *testing.T) {
		t.Setenv("KDN_AUTOCOMPLETE_IGNORE_IDS", "1")

		storageDir, id1, name1, id2, name2 := setup(t)
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeWorkspaceID(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 2 {
			t.Errorf("Expected 2 completions (names only), got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		for _, name := range []string{name1, name2} {
			if !set[name] {
				t.Errorf("Expected name %q in completions, got %v", name, completions)
			}
		}
		for _, id := range []string{id1, id2} {
			if set[id] {
				t.Errorf("Did not expect ID %q in completions when KDN_AUTOCOMPLETE_IGNORE_IDS=1", id)
			}
		}
	})
}

func TestCompleteDashboardWorkspaceIDIgnoreIDs(t *testing.T) {
	// Cannot use t.Parallel() on the parent because subtests use t.Setenv.

	ctx := context.Background()

	t.Run("returns ID and name when KDN_AUTOCOMPLETE_IGNORE_IDS is not set", func(t *testing.T) {
		t.Setenv("KDN_AUTOCOMPLETE_IGNORE_IDS", "")

		storageDir := t.TempDir()
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}
		src := t.TempDir()
		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src,
			ConfigDir: filepath.Join(src, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		added, err := manager.Add(ctx, instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}
		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("failed to start instance: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 2 {
			t.Errorf("Expected 2 completions (ID and name), got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		if !set[added.GetID()] {
			t.Errorf("Expected ID %q in completions, got %v", added.GetID(), completions)
		}
		if !set[added.GetName()] {
			t.Errorf("Expected name %q in completions, got %v", added.GetName(), completions)
		}
	})

	t.Run("returns only name when KDN_AUTOCOMPLETE_IGNORE_IDS is truthy", func(t *testing.T) {
		t.Setenv("KDN_AUTOCOMPLETE_IGNORE_IDS", "true")

		storageDir := t.TempDir()
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}
		src := t.TempDir()
		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src,
			ConfigDir: filepath.Join(src, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		added, err := manager.Add(ctx, instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance: %v", err)
		}
		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("failed to start instance: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return []string{"fake"}, nil
		})

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
		if len(completions) != 1 {
			t.Errorf("Expected 1 completion (name only), got %d: %v", len(completions), completions)
		}
		if len(completions) > 0 && completions[0] != added.GetName() {
			t.Errorf("Expected name %q, got %q", added.GetName(), completions[0])
		}
		for _, c := range completions {
			if c == added.GetID() {
				t.Errorf("Did not expect ID %q in completions when KDN_AUTOCOMPLETE_IGNORE_IDS=true", added.GetID())
			}
		}
	})
}

func TestCompleteRuntimeFlag(t *testing.T) {
	t.Parallel()

	t.Run("returns available runtimes excluding fake", func(t *testing.T) {
		t.Parallel()

		// Create a command (no storage flag needed since it uses runtimesetup.ListAvailable)
		cmd := &cobra.Command{}

		// Call completion function
		completions, directive := completeRuntimeFlag(cmd, []string{}, "")

		// Verify "fake" is not in the completions
		for _, completion := range completions {
			if completion == "fake" {
				t.Errorf("Expected 'fake' runtime to be filtered out, but it was included")
			}
		}

		// Verify directive
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
		}
	})
}

func TestCompleteOpenArgs(t *testing.T) {
	t.Parallel()

	t.Run("first arg completes running workspace IDs and names", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		storageDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}

		// running instance
		src1 := t.TempDir()
		inst1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src1,
			ConfigDir: filepath.Join(src1, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance1: %v", err)
		}
		added1, err := manager.Add(ctx, instances.AddOptions{Instance: inst1, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("failed to add instance1: %v", err)
		}
		if err := manager.Start(ctx, added1.GetID()); err != nil {
			t.Fatalf("failed to start instance1: %v", err)
		}

		// stopped instance — must not appear
		src2 := t.TempDir()
		inst2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: src2,
			ConfigDir: filepath.Join(src2, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("failed to create instance2: %v", err)
		}
		if _, err := manager.Add(ctx, instances.AddOptions{Instance: inst2, RuntimeType: "fake"}); err != nil {
			t.Fatalf("failed to add instance2: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenArgs(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		// Only running instance should appear (ID + name)
		if len(completions) != 2 {
			t.Fatalf("expected 2 completions (ID and name of running workspace), got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		if !set[added1.GetID()] {
			t.Errorf("expected running workspace ID %q in completions, got %v", added1.GetID(), completions)
		}
		if !set[added1.GetName()] {
			t.Errorf("expected running workspace name %q in completions, got %v", added1.GetName(), completions)
		}
	})

	t.Run("second arg completes port numbers for the given workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
			{"bind": "127.0.0.1", "port": 45679, "target": 9090},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenArgs(cmd, []string{id}, "")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 2 {
			t.Fatalf("expected 2 port completions, got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		if !set["8080"] {
			t.Errorf("expected %q in port completions, got %v", "8080", completions)
		}
		if !set["9090"] {
			t.Errorf("expected %q in port completions, got %v", "9090", completions)
		}
	})

	t.Run("third arg and beyond returns no completions", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", t.TempDir(), "")

		completions, directive := completeOpenArgs(cmd, []string{"my-workspace", "8080"}, "")

		if completions != nil {
			t.Errorf("expected nil completions for third arg, got %v", completions)
		}
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
	})

	t.Run("first arg returns error directive when storage flag is missing", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}

		_, directive := completeOpenArgs(cmd, []string{}, "")

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("directive = %v, want ShellCompDirectiveError", directive)
		}
	})
}

func TestCompleteOpenPort(t *testing.T) {
	t.Parallel()

	t.Run("returns error directive when storage flag is missing", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}

		_, directive := completeOpenPort(cmd, "my-workspace")

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("directive = %v, want ShellCompDirectiveError", directive)
		}
	})

	t.Run("returns no completions when storage directory does not exist", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", filepath.Join(t.TempDir(), "nonexistent"), "")

		completions, directive := completeOpenPort(cmd, "my-workspace")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 0 {
			t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns no completions when workspace is not found", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		// Create an empty manager (no instances)
		if _, err := instances.NewManager(storageDir); err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenPort(cmd, "nonexistent")

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 0 {
			t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns no completions when workspace has no forwards", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		id := setupWorkspaceWithForwards(t, storageDir, nil)

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenPort(cmd, id)

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 0 {
			t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
		}
	})

	t.Run("returns target port numbers for a workspace with forwards", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
			{"bind": "127.0.0.1", "port": 45679, "target": 3000},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenPort(cmd, id)

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 2 {
			t.Fatalf("expected 2 port completions, got %d: %v", len(completions), completions)
		}
		set := make(map[string]bool)
		for _, c := range completions {
			set[c] = true
		}
		if !set["8080"] {
			t.Errorf("expected %q in completions, got %v", "8080", completions)
		}
		if !set["3000"] {
			t.Errorf("expected %q in completions, got %v", "3000", completions)
		}
	})

	t.Run("returns a single port number for a single-forward workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenPort(cmd, id)

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 1 {
			t.Fatalf("expected 1 port completion, got %d: %v", len(completions), completions)
		}
		if completions[0] != "8080" {
			t.Errorf("expected %q, got %q", "8080", completions[0])
		}
	})

	t.Run("lookup by name as well as ID", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
		}
		setupWorkspaceWithForwards(t, storageDir, forwards)

		// Get the name from the stored instance.
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("failed to register fake runtime: %v", err)
		}
		list, err := manager.List()
		if err != nil || len(list) == 0 {
			t.Fatalf("failed to list instances: %v", err)
		}
		name := list[0].GetName()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		completions, directive := completeOpenPort(cmd, name)

		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(completions) != 1 || completions[0] != "8080" {
			t.Errorf("expected [\"8080\"], got %v", completions)
		}
	})
}

func TestCompleteDashboardWorkspaceIDWith_ListError(t *testing.T) {
	t.Parallel()

	t.Run("returns error directive when listDashboardTypes fails", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		_, directive := completeDashboardWorkspaceIDWith(cmd, func(_ string) ([]string, error) {
			return nil, filepath.ErrBadPattern
		})

		if directive != cobra.ShellCompDirectiveError {
			t.Errorf("directive = %v, want ShellCompDirectiveError", directive)
		}
	})
}
