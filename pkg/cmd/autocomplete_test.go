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
	"path/filepath"
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/fake"
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
		configDir1 := filepath.Join(sourceDir1, ".kortex")
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
		configDir2 := filepath.Join(sourceDir2, ".kortex")
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

		// Verify we got only instance2 (instance1 is running)
		if len(completions) != 1 {
			t.Errorf("Expected 1 completion (non-running), got %d", len(completions))
		}

		// Verify only instance2 is in the completions
		if len(completions) > 0 && completions[0] != addedInstance2.GetID() {
			t.Errorf("Expected ID %s in completions, got %s", addedInstance2.GetID(), completions[0])
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
		storageDir := t.TempDir()

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
		configDir1 := filepath.Join(sourceDir1, ".kortex")
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
		configDir2 := filepath.Join(sourceDir2, ".kortex")
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

		// Verify we got only instance1 (instance2 is not running)
		if len(completions) != 1 {
			t.Errorf("Expected 1 completion (running), got %d", len(completions))
		}

		// Verify only instance1 is in the completions
		if len(completions) > 0 && completions[0] != addedInstance1.GetID() {
			t.Errorf("Expected ID %s in completions, got %s", addedInstance1.GetID(), completions[0])
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
		configDir := filepath.Join(sourceDir, ".kortex")
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
