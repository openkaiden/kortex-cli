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

	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/spf13/cobra"
)

// injectForwards writes the given WorkspaceForward list as JSON into the "forwards"
// key of the first instance stored in storageDir.
func injectForwards(t *testing.T, storageDir string, instanceID string, forwards []map[string]any) {
	t.Helper()
	storageFile := filepath.Join(storageDir, instances.DefaultStorageFileName)
	data, err := os.ReadFile(storageFile)
	if err != nil {
		t.Fatalf("injectForwards: read storage: %v", err)
	}

	var all []map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatalf("injectForwards: unmarshal: %v", err)
	}

	forwardsJSON, err := json.Marshal(forwards)
	if err != nil {
		t.Fatalf("injectForwards: marshal forwards: %v", err)
	}

	for i, entry := range all {
		id, _ := entry["id"].(string)
		if id != instanceID {
			continue
		}
		rt, ok := entry["runtime"].(map[string]any)
		if !ok {
			rt = make(map[string]any)
			all[i]["runtime"] = rt
		}
		info, ok := rt["info"].(map[string]any)
		if !ok {
			info = make(map[string]any)
			rt["info"] = info
		}
		info["forwards"] = string(forwardsJSON)
		all[i]["runtime"] = rt
		break
	}

	updated, err := json.Marshal(all)
	if err != nil {
		t.Fatalf("injectForwards: marshal updated: %v", err)
	}
	if err := os.WriteFile(storageFile, updated, 0644); err != nil {
		t.Fatalf("injectForwards: write storage: %v", err)
	}
}

func setupWorkspaceWithForwards(t *testing.T, storageDir string, forwards []map[string]any) string {
	t.Helper()
	sourceDir := t.TempDir()

	manager, err := instances.NewManager(storageDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := manager.RegisterRuntime(fake.New()); err != nil {
		t.Fatalf("RegisterRuntime: %v", err)
	}

	inst, err := instances.NewInstance(instances.NewInstanceParams{
		SourceDir: sourceDir,
		ConfigDir: filepath.Join(sourceDir, ".kaiden"),
	})
	if err != nil {
		t.Fatalf("NewInstance: %v", err)
	}

	added, err := manager.Add(context.Background(), instances.AddOptions{Instance: inst, RuntimeType: "fake"})
	if err != nil {
		t.Fatalf("manager.Add: %v", err)
	}
	if err := manager.Start(context.Background(), added.GetID()); err != nil {
		t.Fatalf("manager.Start: %v", err)
	}

	if len(forwards) > 0 {
		injectForwards(t, storageDir, added.GetID(), forwards)
	}

	return added.GetID()
}

func TestWorkspaceOpenCmd(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceOpenCmd()
	if cmd == nil {
		t.Fatal("NewWorkspaceOpenCmd() returned nil")
	}

	if cmd.Use != "open NAME|ID [PORT]" {
		t.Errorf("Expected Use to be 'open NAME|ID [PORT]', got %q", cmd.Use)
	}
}

func TestWorkspaceOpenCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("extracts name from args and creates manager", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		c := &workspaceOpenCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		if err := c.preRun(cmd, []string{"my-workspace"}); err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}
		if c.nameOrID != "my-workspace" {
			t.Errorf("nameOrID = %q, want %q", c.nameOrID, "my-workspace")
		}
		if c.port != 0 {
			t.Errorf("port = %d, want 0", c.port)
		}
		if c.manager == nil {
			t.Error("Expected manager to be created")
		}
	})

	t.Run("parses optional port argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		c := &workspaceOpenCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		if err := c.preRun(cmd, []string{"my-workspace", "8080"}); err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}
		if c.port != 8080 {
			t.Errorf("port = %d, want 8080", c.port)
		}
	})

	t.Run("returns error for non-numeric port", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		c := &workspaceOpenCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("storage", storageDir, "")

		err := c.preRun(cmd, []string{"my-workspace", "not-a-port"})
		if err == nil {
			t.Fatal("Expected error for non-numeric port, got nil")
		}
		if !strings.Contains(err.Error(), "invalid port") {
			t.Errorf("Expected 'invalid port' error, got: %v", err)
		}
	})

	t.Run("returns error when storage flag is not registered", func(t *testing.T) {
		t.Parallel()

		c := &workspaceOpenCmd{}
		cmd := &cobra.Command{}

		err := c.preRun(cmd, []string{"my-workspace"})
		if err == nil {
			t.Fatal("Expected error when storage flag is not registered, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read --storage flag") {
			t.Errorf("Expected 'failed to read --storage flag' error, got: %v", err)
		}
	})
}

func TestWorkspaceOpenCmd_Run(t *testing.T) {
	t.Parallel()

	t.Run("fails for nonexistent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		c := &workspaceOpenCmd{manager: manager, nameOrID: "nonexistent"}
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&bytes.Buffer{})

		err := c.run(cobraCmd, nil)
		if err == nil {
			t.Fatal("Expected error for nonexistent workspace")
		}
		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("Expected 'workspace not found', got: %v", err)
		}
	})

	t.Run("fails when no ports are configured", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		id := setupWorkspaceWithForwards(t, storageDir, nil)

		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		c := &workspaceOpenCmd{manager: manager, nameOrID: id}
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&bytes.Buffer{})

		err := c.run(cobraCmd, nil)
		if err == nil {
			t.Fatal("Expected error when no forwards configured")
		}
		if !strings.Contains(err.Error(), "no port forwards configured") {
			t.Errorf("Expected 'no port forwards configured', got: %v", err)
		}
	})

	t.Run("opens single port without port argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		var openedURL string
		c := &workspaceOpenCmd{
			manager:  manager,
			nameOrID: id,
			openBrowser: func(_ context.Context, url string) error {
				openedURL = url
				return nil
			},
		}

		var outBuf bytes.Buffer
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&outBuf)

		if err := c.run(cobraCmd, nil); err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		expected := "http://127.0.0.1:45678"
		if output := strings.TrimSpace(outBuf.String()); output != expected {
			t.Errorf("stdout = %q, want %q", output, expected)
		}
		if openedURL != expected {
			t.Errorf("openBrowser url = %q, want %q", openedURL, expected)
		}
	})

	t.Run("fails when multiple ports and no port argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
			{"bind": "127.0.0.1", "port": 45679, "target": 9090},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		c := &workspaceOpenCmd{manager: manager, nameOrID: id}
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&bytes.Buffer{})

		err := c.run(cobraCmd, nil)
		if err == nil {
			t.Fatal("Expected error when multiple ports and no port argument")
		}
		if !strings.Contains(err.Error(), "multiple port forwards") {
			t.Errorf("Expected 'multiple port forwards' error, got: %v", err)
		}
	})

	t.Run("opens specific port when multiple ports configured", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
			{"bind": "127.0.0.1", "port": 45679, "target": 9090},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		c := &workspaceOpenCmd{manager: manager, nameOrID: id, port: 9090}
		var outBuf bytes.Buffer
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&outBuf)

		if err := c.run(cobraCmd, nil); err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		expected := "http://127.0.0.1:45679"
		if output := strings.TrimSpace(outBuf.String()); output != expected {
			t.Errorf("stdout = %q, want %q", output, expected)
		}
	})

	t.Run("fails when specified port not found", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		forwards := []map[string]any{
			{"bind": "127.0.0.1", "port": 45678, "target": 8080},
		}
		id := setupWorkspaceWithForwards(t, storageDir, forwards)

		manager, _ := instances.NewManager(storageDir)
		_ = manager.RegisterRuntime(fake.New())

		c := &workspaceOpenCmd{manager: manager, nameOrID: id, port: 9999}
		cobraCmd := &cobra.Command{}
		cobraCmd.SetOut(&bytes.Buffer{})

		err := c.run(cobraCmd, nil)
		if err == nil {
			t.Fatal("Expected error for unknown port")
		}
		if !strings.Contains(err.Error(), "no port forward found for port 9999") {
			t.Errorf("Expected 'no port forward found for port 9999', got: %v", err)
		}
	})
}

func TestWorkspaceOpenCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("fails for nonexistent workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "open", "nonexistent", "--storage", storageDir})

		var outBuf bytes.Buffer
		rootCmd.SetOut(&outBuf)
		rootCmd.SetErr(&outBuf)

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error for nonexistent workspace")
		}
		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("Expected 'workspace not found', got: %v", err)
		}
	})

	t.Run("requires at least one argument", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "open", "--storage", storageDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error when no argument provided")
		}
	})
}

func TestWorkspaceOpenCmd_Examples(t *testing.T) {
	t.Parallel()

	cmd := NewWorkspaceOpenCmd()

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
	if err := testutil.ValidateCommandExamples(rootCmd, cmd.Example); err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}
