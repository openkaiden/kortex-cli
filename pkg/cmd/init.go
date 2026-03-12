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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

// initCmd contains the configuration for the init command
type initCmd struct {
	sourcesDir         string
	workspaceConfigDir string
	name               string
	absSourcesDir      string
	absConfigDir       string
	manager            instances.Manager
	verbose            bool
	output             string
}

// preRun validates the parameters and flags
func (i *initCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if i.output != "" && i.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", i.output)
	}

	// Silence Cobra's error and usage output when JSON mode is enabled
	// This prevents "Error: ..." and usage info from being printed
	if i.output == "json" {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
	}

	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	// Convert to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve storage directory path: %w", err))
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to create manager: %w", err))
	}
	i.manager = manager

	// Get sources directory (default to current directory)
	i.sourcesDir = "."
	if len(args) > 0 {
		i.sourcesDir = args[0]
	}

	// Convert to absolute path for clarity
	absSourcesDir, err := filepath.Abs(i.sourcesDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve sources directory path: %w", err))
	}
	i.absSourcesDir = absSourcesDir

	// Verify that the sources directory exists and is a directory
	fileInfo, err := os.Stat(i.absSourcesDir)
	if os.IsNotExist(err) {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("sources directory does not exist: %s", i.absSourcesDir))
	} else if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to check sources directory: %w", err))
	}
	if !fileInfo.IsDir() {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("sources path is not a directory: %s", i.absSourcesDir))
	}

	// If workspace-configuration flag was not explicitly set, default to .kortex/ inside sources directory
	if !cmd.Flags().Changed("workspace-configuration") {
		i.workspaceConfigDir = filepath.Join(i.sourcesDir, ".kortex")
	}

	// Convert workspace config to absolute path
	absConfigDir, err := filepath.Abs(i.workspaceConfigDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve workspace configuration directory path: %w", err))
	}
	i.absConfigDir = absConfigDir

	return nil
}

// run executes the init command logic
func (i *initCmd) run(cmd *cobra.Command, args []string) error {
	// Create a new instance
	instance, err := instances.NewInstance(instances.NewInstanceParams{
		SourceDir: i.absSourcesDir,
		ConfigDir: i.absConfigDir,
		Name:      i.name,
	})
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, err)
	}

	// Add the instance to the manager
	addedInstance, err := i.manager.Add(instance)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, err)
	}

	// Handle JSON output
	if i.output == "json" {
		return i.outputJSON(cmd, addedInstance)
	}

	// Handle text output
	if i.verbose {
		cmd.Printf("Registered workspace:\n")
		cmd.Printf("  ID: %s\n", addedInstance.GetID())
		cmd.Printf("  Name: %s\n", addedInstance.GetName())
		cmd.Printf("  Sources directory: %s\n", addedInstance.GetSourceDir())
		cmd.Printf("  Configuration directory: %s\n", addedInstance.GetConfigDir())
	} else {
		cmd.Println(addedInstance.GetID())
	}

	return nil
}

// outputJSON outputs the instance as JSON based on verbose flag
func (i *initCmd) outputJSON(cmd *cobra.Command, instance instances.Instance) error {
	var jsonData []byte
	var err error

	if i.verbose {
		// Verbose mode: return full Workspace
		workspace := instanceToWorkspace(instance)
		jsonData, err = json.MarshalIndent(workspace, "", "  ")
	} else {
		// Default mode: return only WorkspaceId
		workspaceId := instanceToWorkspaceId(instance)
		jsonData, err = json.MarshalIndent(workspaceId, "", "  ")
	}

	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to marshal to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewInitCmd() *cobra.Command {
	c := &initCmd{}

	cmd := &cobra.Command{
		Use:   "init [sources-directory]",
		Short: "Register a new workspace",
		Long: `Register a new workspace with the specified sources directory and configuration location.

The sources directory defaults to the current directory (.) if not specified.
The workspace configuration directory defaults to .kortex/ inside the sources directory if not specified.`,
		Example: `# Register current directory as workspace
kortex-cli init

# Register specific directory as workspace
kortex-cli init /path/to/project

# Register with custom workspace name
kortex-cli init --name my-project

# Show detailed output
kortex-cli init --verbose`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	// Add workspace-configuration flag
	cmd.Flags().StringVar(&c.workspaceConfigDir, "workspace-configuration", "", "Directory for workspace configuration (default: <sources-directory>/.kortex)")

	// Add name flag
	cmd.Flags().StringVarP(&c.name, "name", "n", "", "Name for the workspace (default: generated from sources directory)")

	// Add verbose flag
	cmd.Flags().BoolVarP(&c.verbose, "verbose", "v", false, "Show detailed output")

	// Add output flag
	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")

	return cmd
}
