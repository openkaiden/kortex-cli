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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/secretservice"
	"github.com/openkaiden/kdn/pkg/secretservicesetup"
	"github.com/spf13/cobra"
)

func TestWorkspaceTerminalCmd(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceTerminalCmd()
	if cmd == nil {
		t.Fatal("NewWorkspaceTerminalCmd() returned nil")
	}

	if cmd.Use != "terminal NAME|ID [COMMAND...]" {
		t.Errorf("Expected Use to be 'terminal NAME|ID [COMMAND...]', got '%s'", cmd.Use)
	}
}

func TestWorkspaceTerminalCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("extracts id from args and creates manager", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceTerminalCmd{}
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

		// Verify command is empty when no command args provided
		// The runtime will choose the agent's terminal command
		if len(c.command) != 0 {
			t.Errorf("Expected empty command [], got %v", c.command)
		}
	})

	t.Run("registers all secret services so known types resolve on auto-start", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		c := &workspaceTerminalCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		if err := c.preRun(cmd, []string{"test-id"}); err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		for _, name := range secretservicesetup.ListAvailable() {
			svc := secretservice.NewSecretService(name, nil, "", nil, "", "", "")
			err := c.manager.RegisterSecretService(svc)
			if err == nil {
				t.Errorf("secret service %q was not registered by preRun (re-registration succeeded)", name)
			} else if !strings.Contains(err.Error(), "already registered") {
				t.Errorf("secret service %q: unexpected error: %v", name, err)
			}
		}
	})

	t.Run("handles id with command args", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		c := &workspaceTerminalCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "test storage flag")

		// args contains ID and command
		args := []string{"test-id", "bash", "-l"}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.nameOrID != "test-id" {
			t.Errorf("Expected id to be 'test-id', got %s", c.nameOrID)
		}

		// Verify command was extracted in preRun
		if len(c.command) != 2 {
			t.Errorf("Expected command length 2, got %d", len(c.command))
		}
		if len(c.command) >= 2 && (c.command[0] != "bash" || c.command[1] != "-l") {
			t.Errorf("Expected command ['bash', '-l'], got %v", c.command)
		}
	})
}

func TestWorkspaceTerminalCmd_Examples(t *testing.T) {
	t.Parallel()

	// Get the command
	cmd := NewWorkspaceTerminalCmd()

	// Verify Example field is not empty
	if cmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	// Parse the examples
	commands, err := testutil.ParseExampleCommands(cmd.Example)
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
	err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}

func TestWorkspaceTerminalCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("fails for nonexistent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "terminal", "nonexistent-id", "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error for nonexistent workspace")
		}

		output := outBuf.String()
		if !strings.Contains(output, "workspace not found") && !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("Expected 'workspace not found' error, got: %v (output: %s)", err, output)
		}
	})

	t.Run("auto-starts stopped workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourceDir := t.TempDir()
		configDir := filepath.Join(sourceDir, ".kaiden")
		os.MkdirAll(configDir, 0755)

		// Initialize a workspace
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"init", sourceDir, "--storage", storageDir, "--runtime", "fake", "--agent", "test-agent"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Failed to init workspace: %v", err)
		}

		// Get the workspace ID
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}
		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}
		if len(instancesList) == 0 {
			t.Fatal("No instances found after init")
		}
		workspaceID := instancesList[0].GetID()

		// Try to connect to terminal (workspace is not started, should auto-start)
		rootCmd = NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "terminal", workspaceID, "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err = rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error because fake runtime does not support terminal")
		}

		// Should auto-start successfully, then fail because fake runtime doesn't implement Terminal
		if !strings.Contains(err.Error(), "does not support terminal sessions") {
			t.Errorf("Expected 'does not support terminal sessions' error, got: %v", err)
		}
	})
}
