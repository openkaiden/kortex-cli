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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/version"
	"github.com/spf13/cobra"
)

func TestInfoCmd(t *testing.T) {
	t.Parallel()

	cmd := NewInfoCmd()
	if cmd == nil {
		t.Fatal("NewInfoCmd() returned nil")
	}

	if cmd.Use != "info" {
		t.Errorf("Expected Use to be 'info', got '%s'", cmd.Use)
	}
}

func TestInfoCmd_Examples(t *testing.T) {
	t.Parallel()

	cmd := NewInfoCmd()

	if cmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	commands, err := testutil.ParseExampleCommands(cmd.Example)
	if err != nil {
		t.Fatalf("Failed to parse examples: %v", err)
	}

	expectedCount := 3
	if len(commands) != expectedCount {
		t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
	}

	rootCmd := NewRootCmd()
	err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}

func TestInfoCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("accepts empty output flag", func(t *testing.T) {
		t.Parallel()

		c := &infoCmd{}
		cmd := &cobra.Command{}

		err := c.preRun(cmd, []string{})
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}
	})

	t.Run("accepts json output format", func(t *testing.T) {
		t.Parallel()

		c := &infoCmd{output: "json"}
		cmd := &cobra.Command{}

		err := c.preRun(cmd, []string{})
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}
	})

	t.Run("rejects invalid output format", func(t *testing.T) {
		t.Parallel()

		c := &infoCmd{output: "xml"}
		cmd := &cobra.Command{}

		err := c.preRun(cmd, []string{})
		if err == nil {
			t.Fatal("Expected error for invalid output format")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected 'unsupported output format' error, got: %v", err)
		}
	})
}

func TestInfoCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("text output contains version and runtimes", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"info", "--storage", storageDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Version: "+version.Version) {
			t.Errorf("Expected output to contain version, got: %s", output)
		}
		if !strings.Contains(output, "Runtimes:") {
			t.Errorf("Expected output to contain Runtimes, got: %s", output)
		}
		if !strings.Contains(output, "Agents:") {
			t.Errorf("Expected output to contain Agents, got: %s", output)
		}
	})

	t.Run("json output has expected fields", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"info", "--storage", storageDir, "-o", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		var response api.Info
		if err := json.Unmarshal(buf.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if response.Version != version.Version {
			t.Errorf("Expected version %s, got: %s", version.Version, response.Version)
		}

		if response.Runtimes == nil {
			t.Error("Expected runtimes to be present")
		}

		if response.Agents == nil {
			t.Error("Expected agents to be present")
		}
	})

	t.Run("json output with agents from config", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("podman"); err != nil {
			t.Skip("podman not available, skipping agent discovery test")
		}

		storageDir := t.TempDir()

		// Create podman config directory with agent config files
		configDir := filepath.Join(storageDir, "runtimes", "podman", "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		agentConfig := `{"packages": [], "run_commands": [], "terminal_command": []}`
		for _, agent := range []string{"claude", "cursor", "goose"} {
			if err := os.WriteFile(filepath.Join(configDir, agent+".json"), []byte(agentConfig), 0644); err != nil {
				t.Fatalf("Failed to write %s config: %v", agent, err)
			}
		}

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"info", "--storage", storageDir, "-o", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		var response api.Info
		if err := json.Unmarshal(buf.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		expected := []string{"claude", "cursor", "goose", "openclaw", "opencode"}
		if !slices.Equal(response.Agents, expected) {
			t.Errorf("Expected agents %v, got: %v", expected, response.Agents)
		}
	})
}
