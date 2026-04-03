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

	"github.com/kortex-hub/kortex-cli/pkg/instances"
)

func TestTerminalCmd(t *testing.T) {
	t.Parallel()

	cmd := NewTerminalCmd()
	if cmd == nil {
		t.Fatal("NewTerminalCmd() returned nil")
	}

	if cmd.Use != "terminal NAME|ID [COMMAND...]" {
		t.Errorf("Expected Use to be 'terminal NAME|ID [COMMAND...]', got '%s'", cmd.Use)
	}
}

func TestTerminalCmd_DelegatesToWorkspaceTerminal(t *testing.T) {
	t.Parallel()

	terminalCmd := NewTerminalCmd()
	workspaceTerminalCmd := NewWorkspaceTerminalCmd()

	// Verify it includes the original workspace terminal Short description
	if !strings.Contains(terminalCmd.Short, workspaceTerminalCmd.Short) {
		t.Errorf("Expected Short to contain workspace terminal Short '%s', got '%s'", workspaceTerminalCmd.Short, terminalCmd.Short)
	}

	// Verify it includes the alias indicator
	if !strings.Contains(terminalCmd.Short, "(alias for 'workspace terminal')") {
		t.Errorf("Expected Short to contain alias indicator, got '%s'", terminalCmd.Short)
	}

	if terminalCmd.Long != workspaceTerminalCmd.Long {
		t.Errorf("Long mismatch")
	}
}

func TestTerminalCmd_ExamplesAdapted(t *testing.T) {
	t.Parallel()

	terminalCmd := NewTerminalCmd()

	// Verify examples are adapted
	if !strings.Contains(terminalCmd.Example, "kortex-cli terminal") {
		t.Error("Expected examples to contain 'kortex-cli terminal'")
	}

	if strings.Contains(terminalCmd.Example, "kortex-cli workspace terminal") {
		t.Error("Examples should not contain 'kortex-cli workspace terminal'")
	}
}

func TestTerminalCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("fails for nonexistent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"terminal", "nonexistent-id", "--storage", storageDir})

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

	t.Run("fails for stopped workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourceDir := t.TempDir()
		configDir := filepath.Join(sourceDir, ".kortex")
		os.MkdirAll(configDir, 0755)

		// Initialize a workspace
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"init", sourceDir, "--storage", storageDir, "--runtime", "fake", "--agent", "test-agent"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Failed to init workspace: %v", err)
		}

		// Get the workspace ID using the instances manager
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

		// Try to connect to terminal (workspace is not started)
		rootCmd = NewRootCmd()
		rootCmd.SetArgs([]string{"terminal", workspaceID, "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err = rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error for stopped workspace")
		}

		// Should fail because workspace is not running
		if !strings.Contains(err.Error(), "not running") {
			t.Errorf("Expected 'not running' error, got: %v", err)
		}
	})
}
