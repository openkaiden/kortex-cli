// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParseExampleCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		example       string
		wantCount     int
		wantErr       bool
		checkCommands func(t *testing.T, commands []ExampleCommand)
	}{
		{
			name:      "single command",
			example:   `kdn init`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Binary != "kdn" {
					t.Errorf("Expected binary 'kdn', got '%s'", commands[0].Binary)
				}
				if len(commands[0].Args) != 1 || commands[0].Args[0] != "init" {
					t.Errorf("Expected args [init], got %v", commands[0].Args)
				}
			},
		},
		{
			name: "multiple commands with comments",
			example: `# Initialize workspace
kdn init

# List workspaces
kdn workspace list`,
			wantCount: 2,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if len(commands[0].Args) != 1 || commands[0].Args[0] != "init" {
					t.Errorf("Expected first command args [init], got %v", commands[0].Args)
				}
				if len(commands[1].Args) != 2 || commands[1].Args[0] != "workspace" || commands[1].Args[1] != "list" {
					t.Errorf("Expected second command args [workspace list], got %v", commands[1].Args)
				}
			},
		},
		{
			name:      "command with long flag using equals",
			example:   `kdn workspace list --output=json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["output"] != "json" {
					t.Errorf("Expected flag output=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with long flag using space",
			example:   `kdn workspace list --output json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["output"] != "json" {
					t.Errorf("Expected flag output=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with short flag",
			example:   `kdn workspace list -o json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["o"] != "json" {
					t.Errorf("Expected flag o=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with positional argument",
			example:   `kdn workspace remove abc123`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if len(commands[0].Args) != 3 {
					t.Errorf("Expected 3 args, got %d", len(commands[0].Args))
				}
				if commands[0].Args[2] != "abc123" {
					t.Errorf("Expected arg 'abc123', got '%s'", commands[0].Args[2])
				}
			},
		},
		{
			name:      "command with path argument",
			example:   `kdn init /path/to/project`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if len(commands[0].Args) != 2 {
					t.Errorf("Expected 2 args, got %d", len(commands[0].Args))
				}
				if commands[0].Args[1] != "/path/to/project" {
					t.Errorf("Expected arg '/path/to/project', got '%s'", commands[0].Args[1])
				}
			},
		},
		{
			name:      "command with boolean flag",
			example:   `kdn init --verbose`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if _, exists := commands[0].Flags["verbose"]; !exists {
					t.Errorf("Expected flag 'verbose' to exist")
				}
			},
		},
		{
			name:      "empty example",
			example:   ``,
			wantCount: 0,
		},
		{
			name: "only comments and empty lines",
			example: `# This is a comment

# Another comment
`,
			wantCount: 0,
		},
		{
			name:    "invalid command - wrong binary",
			example: `other-cli init`,
			wantErr: true,
		},
		{
			name:      "command with env var prefix",
			example:   `GH_TOKEN=ghp_abc kdn autoconf`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Binary != "kdn" {
					t.Errorf("Expected binary 'kdn', got '%s'", commands[0].Binary)
				}
				if commands[0].EnvVars["GH_TOKEN"] != "ghp_abc" {
					t.Errorf("Expected EnvVars[GH_TOKEN]=ghp_abc, got %v", commands[0].EnvVars)
				}
				if len(commands[0].Args) != 1 || commands[0].Args[0] != "autoconf" {
					t.Errorf("Expected args [autoconf], got %v", commands[0].Args)
				}
			},
		},
		{
			name:      "command with multiple env var prefixes",
			example:   `ANTHROPIC_API_KEY=sk-ant-... GH_TOKEN=ghp_... kdn autoconf --yes`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].EnvVars["ANTHROPIC_API_KEY"] != "sk-ant-..." {
					t.Errorf("Expected ANTHROPIC_API_KEY in EnvVars, got %v", commands[0].EnvVars)
				}
				if commands[0].EnvVars["GH_TOKEN"] != "ghp_..." {
					t.Errorf("Expected GH_TOKEN in EnvVars, got %v", commands[0].EnvVars)
				}
				if !commands[0].FlagPresent["yes"] {
					t.Errorf("Expected --yes flag to be present")
				}
			},
		},
		{
			name:    "env vars only, no kdn command",
			example: `FOO=bar BAZ=qux`,
			wantErr: true,
		},
		{
			name:      "command with multiple flags",
			example:   `kdn init --name my-project --verbose`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["name"] != "my-project" {
					t.Errorf("Expected flag name=my-project, got %v", commands[0].Flags)
				}
				if _, exists := commands[0].Flags["verbose"]; !exists {
					t.Errorf("Expected flag 'verbose' to exist")
				}
			},
		},
		{
			name:      "command with -- separator",
			example:   `kdn terminal abc123 -- bash -c 'echo hello'`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Binary != "kdn" {
					t.Errorf("Expected binary 'kdn', got '%s'", commands[0].Binary)
				}
				// Should have: terminal, abc123, bash, -c, echo hello
				expectedArgs := []string{"terminal", "abc123", "bash", "-c", "echo hello"}
				if len(commands[0].Args) != len(expectedArgs) {
					t.Errorf("Expected %d args, got %d: %v", len(expectedArgs), len(commands[0].Args), commands[0].Args)
				}
				for i, arg := range expectedArgs {
					if i >= len(commands[0].Args) || commands[0].Args[i] != arg {
						t.Errorf("Expected args[%d]=%s, got %v", i, arg, commands[0].Args)
					}
				}
				// Should have no flags (-- stops flag parsing)
				if len(commands[0].FlagPresent) != 0 {
					t.Errorf("Expected no flags after --, got %v", commands[0].FlagPresent)
				}
			},
		},
		{
			name:      "command with flags before -- separator",
			example:   `kdn terminal --storage /tmp abc123 -- bash -l`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				// Should have flag before --
				if !commands[0].FlagPresent["storage"] {
					t.Error("Expected 'storage' flag to be present")
				}
				if commands[0].FlagValues["storage"] != "/tmp" {
					t.Errorf("Expected storage=/tmp, got %s", commands[0].FlagValues["storage"])
				}
				// Should have: terminal, abc123, bash, -l (after --)
				expectedArgs := []string{"terminal", "abc123", "bash", "-l"}
				if len(commands[0].Args) != len(expectedArgs) {
					t.Errorf("Expected %d args, got %d: %v", len(expectedArgs), len(commands[0].Args), commands[0].Args)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			commands, err := ParseExampleCommands(tt.example)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(commands) != tt.wantCount {
				t.Errorf("Expected %d commands, got %d", tt.wantCount, len(commands))
				return
			}

			if tt.checkCommands != nil {
				tt.checkCommands(t, commands)
			}
		})
	}
}

func TestValidateExampleCommand(t *testing.T) {
	t.Parallel()

	// Helper function to create a fresh command tree for each test
	createTestCommandTree := func() *cobra.Command {
		rootCmd := &cobra.Command{
			Use: "kdn",
		}

		workspaceCmd := &cobra.Command{
			Use: "workspace",
		}

		listCmd := &cobra.Command{
			Use:  "list",
			Args: cobra.NoArgs,
		}
		listCmd.Flags().String("output", "text", "output format")
		listCmd.Flags().StringP("format", "f", "text", "alias for output")

		removeCmd := &cobra.Command{
			Use: "remove",
		}

		initCmd := &cobra.Command{
			Use: "init",
		}
		initCmd.Flags().String("name", "", "workspace name")
		initCmd.Flags().Bool("verbose", false, "verbose output")

		workspaceCmd.AddCommand(listCmd)
		workspaceCmd.AddCommand(removeCmd)
		rootCmd.AddCommand(workspaceCmd)
		rootCmd.AddCommand(initCmd)

		// Add a global flag
		rootCmd.PersistentFlags().String("storage", "", "storage directory")

		return rootCmd
	}

	tests := []struct {
		name       string
		exampleCmd ExampleCommand
		wantErr    bool
	}{
		{
			name: "valid command - init",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid command - workspace list",
			exampleCmd: ExampleCommand{
				Raw:         "kdn workspace list",
				Binary:      "kdn",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid command with flag",
			exampleCmd: ExampleCommand{
				Raw:         "kdn workspace list --output json",
				Binary:      "kdn",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{"output": true},
				FlagValues:  map[string]string{"output": "json"},
			},
			wantErr: false,
		},
		{
			name: "valid command with short flag",
			exampleCmd: ExampleCommand{
				Raw:         "kdn workspace list -f json",
				Binary:      "kdn",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{"f": true},
				FlagValues:  map[string]string{"f": "json"},
			},
			wantErr: false,
		},
		{
			name: "valid command with persistent flag",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init --storage /tmp/storage",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"storage": true},
				FlagValues:  map[string]string{"storage": "/tmp/storage"},
			},
			wantErr: false,
		},
		{
			name: "invalid command - non-existent",
			exampleCmd: ExampleCommand{
				Raw:         "kdn nonexistent",
				Binary:      "kdn",
				Args:        []string{"nonexistent"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "invalid flag",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init --nonexistent-flag value",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"nonexistent-flag": true},
				FlagValues:  map[string]string{"nonexistent-flag": "value"},
			},
			wantErr: true,
		},
		{
			name: "valid command with multiple flags",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init --name test --verbose",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"name": true, "verbose": true},
				FlagValues:  map[string]string{"name": "test", "verbose": ""},
			},
			wantErr: false,
		},
		{
			name: "string flag missing value",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init --name",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"name": true},
				FlagValues:  map[string]string{"name": ""},
			},
			wantErr: true,
		},
		{
			name: "boolean flag without value is valid",
			exampleCmd: ExampleCommand{
				Raw:         "kdn init --verbose",
				Binary:      "kdn",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"verbose": true},
				FlagValues:  map[string]string{"verbose": ""},
			},
			wantErr: false,
		},
		{
			name: "extra positional to leaf command",
			exampleCmd: ExampleCommand{
				Raw:         "kdn workspace list extra",
				Binary:      "kdn",
				Args:        []string{"workspace", "list", "extra"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh command tree for this test to avoid data races
			rootCmd := createTestCommandTree()

			err := ValidateExampleCommand(rootCmd, tt.exampleCmd)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateCommandExamples(t *testing.T) {
	t.Parallel()

	// Helper function to create a fresh command tree for each test
	createTestCommandTree := func() *cobra.Command {
		rootCmd := &cobra.Command{
			Use: "kdn",
		}

		initCmd := &cobra.Command{
			Use: "init",
		}
		initCmd.Flags().String("name", "", "workspace name")
		initCmd.Flags().Bool("verbose", false, "verbose output")

		rootCmd.AddCommand(initCmd)
		rootCmd.PersistentFlags().String("storage", "", "storage directory")

		return rootCmd
	}

	tests := []struct {
		name    string
		example string
		wantErr bool
	}{
		{
			name: "valid examples",
			example: `# Initialize workspace
kdn init

# Initialize with name
kdn init --name my-project

# Verbose output
kdn init --verbose`,
			wantErr: false,
		},
		{
			name: "invalid command in examples",
			example: `# Valid command
kdn init

# Invalid command
kdn nonexistent`,
			wantErr: true,
		},
		{
			name: "invalid flag in examples",
			example: `# Invalid flag
kdn init --invalid-flag value`,
			wantErr: true,
		},
		{
			name:    "empty examples",
			example: ``,
			wantErr: false,
		},
		{
			name: "only comments",
			example: `# This is just a comment
# Another comment`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh command tree for this test to avoid data races
			rootCmd := createTestCommandTree()

			err := ValidateCommandExamples(rootCmd, tt.example)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
