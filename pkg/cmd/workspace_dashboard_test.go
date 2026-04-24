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
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/spf13/cobra"
)

func TestWorkspaceDashboardCmd(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceDashboardCmd()
	if cmd == nil {
		t.Fatal("NewWorkspaceDashboardCmd() returned nil")
	}

	if cmd.Use != "dashboard NAME|ID" {
		t.Errorf("Expected Use to be 'dashboard NAME|ID', got '%s'", cmd.Use)
	}
}

func TestWorkspaceDashboardCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("extracts name from args and creates manager", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceDashboardCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		err := c.preRun(cmd, []string{"my-workspace"})
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.nameOrID != "my-workspace" {
			t.Errorf("Expected nameOrID to be 'my-workspace', got %s", c.nameOrID)
		}
	})

	t.Run("returns error when storage flag is not registered", func(t *testing.T) {
		t.Parallel()

		c := &workspaceDashboardCmd{}
		cmd := &cobra.Command{}
		// Intentionally omit registering the "storage" flag

		err := c.preRun(cmd, []string{"my-workspace"})
		if err == nil {
			t.Fatal("Expected error when storage flag is not registered, got nil")
		}

		if !strings.Contains(err.Error(), "failed to read --storage flag") {
			t.Errorf("Expected 'failed to read --storage flag' error, got: %v", err)
		}
	})
}

func TestWorkspaceDashboardCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("fails for nonexistent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "dashboard", "nonexistent-id", "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error for nonexistent workspace")
		}

		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("Expected 'workspace not found' error, got: %v", err)
		}
	})

	t.Run("fails when runtime does not support dashboard", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourceDir := t.TempDir()

		// Create instance using the standard fake runtime (no Dashboard support)
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(sourceDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}
		if err := manager.Start(context.Background(), added.GetID()); err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		// rootCmd creates its own manager via runtimesetup.RegisterAll, which also
		// registers fake.New() (without Dashboard). The command will fail at the Dashboard check.
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "dashboard", added.GetID(), "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err = rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when runtime does not support dashboard")
		}

		if !strings.Contains(err.Error(), "dashboard not supported") {
			t.Errorf("Expected 'dashboard not supported' error, got: %v", err)
		}
	})

	t.Run("outputs dashboard URL when runtime supports dashboard", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourceDir := t.TempDir()

		const dashboardURL = "http://localhost:8888"

		// Build a manager with the dashboard-capable fake runtime and add an instance.
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}
		if err := manager.RegisterRuntime(fake.NewWithDashboard(dashboardURL)); err != nil {
			t.Fatalf("Failed to register fake runtime with dashboard: %v", err)
		}

		inst, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(sourceDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}
		if err := manager.Start(context.Background(), added.GetID()); err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		// Inject the dashboard-capable manager directly into the command struct,
		// bypassing preRun which would register a standard fake without Dashboard support.
		c := &workspaceDashboardCmd{
			manager:  manager,
			nameOrID: added.GetID(),
		}

		var outBuf bytes.Buffer
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&outBuf)

		err = c.run(cobraCmd, nil)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		output := strings.TrimSpace(outBuf.String())
		if output != dashboardURL {
			t.Errorf("Expected output %q, got %q", dashboardURL, output)
		}
	})

	t.Run("requires exactly one argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "dashboard", "--storage", storageDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when no argument provided")
		}
	})
}

func TestWorkspaceDashboardCmd_Examples(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceDashboardCmd()

	if cmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	commands, err := testutil.ParseExampleCommands(cmd.Example)
	if err != nil {
		t.Fatalf("Failed to parse examples: %v", err)
	}

	expectedCount := 2
	if len(commands) != expectedCount {
		t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
	}

	rootCmd := NewRootCmd()
	err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}
