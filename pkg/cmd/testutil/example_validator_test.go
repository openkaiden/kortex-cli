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
			example:   `kortex-cli init`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Binary != "kortex-cli" {
					t.Errorf("Expected binary 'kortex-cli', got '%s'", commands[0].Binary)
				}
				if len(commands[0].Args) != 1 || commands[0].Args[0] != "init" {
					t.Errorf("Expected args [init], got %v", commands[0].Args)
				}
			},
		},
		{
			name: "multiple commands with comments",
			example: `# Initialize workspace
kortex-cli init

# List workspaces
kortex-cli workspace list`,
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
			example:   `kortex-cli workspace list --output=json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["output"] != "json" {
					t.Errorf("Expected flag output=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with long flag using space",
			example:   `kortex-cli workspace list --output json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["output"] != "json" {
					t.Errorf("Expected flag output=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with short flag",
			example:   `kortex-cli workspace list -o json`,
			wantCount: 1,
			checkCommands: func(t *testing.T, commands []ExampleCommand) {
				if commands[0].Flags["o"] != "json" {
					t.Errorf("Expected flag o=json, got %v", commands[0].Flags)
				}
			},
		},
		{
			name:      "command with positional argument",
			example:   `kortex-cli workspace remove abc123`,
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
			example:   `kortex-cli init /path/to/project`,
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
			example:   `kortex-cli init --verbose`,
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
			name:      "command with multiple flags",
			example:   `kortex-cli init --name my-project --verbose`,
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
			Use: "kortex-cli",
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
				Raw:         "kortex-cli init",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid command - workspace list",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli workspace list",
				Binary:      "kortex-cli",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid command with flag",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli workspace list --output json",
				Binary:      "kortex-cli",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{"output": true},
				FlagValues:  map[string]string{"output": "json"},
			},
			wantErr: false,
		},
		{
			name: "valid command with short flag",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli workspace list -f json",
				Binary:      "kortex-cli",
				Args:        []string{"workspace", "list"},
				FlagPresent: map[string]bool{"f": true},
				FlagValues:  map[string]string{"f": "json"},
			},
			wantErr: false,
		},
		{
			name: "valid command with persistent flag",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli init --storage /tmp/storage",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"storage": true},
				FlagValues:  map[string]string{"storage": "/tmp/storage"},
			},
			wantErr: false,
		},
		{
			name: "invalid command - non-existent",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli nonexistent",
				Binary:      "kortex-cli",
				Args:        []string{"nonexistent"},
				FlagPresent: map[string]bool{},
				FlagValues:  map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "invalid flag",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli init --nonexistent-flag value",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"nonexistent-flag": true},
				FlagValues:  map[string]string{"nonexistent-flag": "value"},
			},
			wantErr: true,
		},
		{
			name: "valid command with multiple flags",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli init --name test --verbose",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"name": true, "verbose": true},
				FlagValues:  map[string]string{"name": "test", "verbose": ""},
			},
			wantErr: false,
		},
		{
			name: "string flag missing value",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli init --name",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"name": true},
				FlagValues:  map[string]string{"name": ""},
			},
			wantErr: true,
		},
		{
			name: "boolean flag without value is valid",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli init --verbose",
				Binary:      "kortex-cli",
				Args:        []string{"init"},
				FlagPresent: map[string]bool{"verbose": true},
				FlagValues:  map[string]string{"verbose": ""},
			},
			wantErr: false,
		},
		{
			name: "extra positional to leaf command",
			exampleCmd: ExampleCommand{
				Raw:         "kortex-cli workspace list extra",
				Binary:      "kortex-cli",
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
			Use: "kortex-cli",
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
kortex-cli init

# Initialize with name
kortex-cli init --name my-project

# Verbose output
kortex-cli init --verbose`,
			wantErr: false,
		},
		{
			name: "invalid command in examples",
			example: `# Valid command
kortex-cli init

# Invalid command
kortex-cli nonexistent`,
			wantErr: true,
		},
		{
			name: "invalid flag in examples",
			example: `# Invalid flag
kortex-cli init --invalid-flag value`,
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

