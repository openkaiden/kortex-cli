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
	"path/filepath"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

// workspaceListCmd contains the configuration for the workspace list command
type workspaceListCmd struct {
	manager instances.Manager
	output  string
}

// preRun validates the parameters and flags
func (w *workspaceListCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if w.output != "" && w.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", w.output)
	}

	// Silence Cobra's default error output to stderr when JSON mode is enabled,
	// because we write the error in the JSON response to stdout instead
	if w.output == "json" {
		cmd.SilenceErrors = true
	}

	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	// Convert to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to resolve storage directory path: %w", err))
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to create manager: %w", err))
	}
	w.manager = manager

	return nil
}

// run executes the workspace list command logic
func (w *workspaceListCmd) run(cmd *cobra.Command, args []string) error {
	// Get all instances
	instancesList, err := w.manager.List()
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to list instances: %w", err))
	}

	// Handle JSON output format
	if w.output == "json" {
		return w.outputJSON(cmd, instancesList)
	}

	// Display the instances in text format
	out := cmd.OutOrStdout()
	if len(instancesList) == 0 {
		fmt.Fprintln(out, "No workspaces registered")
		return nil
	}

	for _, instance := range instancesList {
		fmt.Fprintf(out, "ID: %s\n", instance.GetID())
		fmt.Fprintf(out, "  Name: %s\n", instance.GetName())
		fmt.Fprintf(out, "  Project: %s\n", instance.GetProject())
		fmt.Fprintf(out, "  Agent: %s\n", instance.GetAgent())
		fmt.Fprintf(out, "  Sources: %s\n", instance.GetSourceDir())
		fmt.Fprintf(out, "  Configuration: %s\n", instance.GetConfigDir())
		fmt.Fprintf(out, "  State: %s\n", instance.GetRuntimeData().State)
		fmt.Fprintln(out)
	}

	return nil
}

// outputJSON converts instances to Workspace format and outputs as JSON
func (w *workspaceListCmd) outputJSON(cmd *cobra.Command, instancesList []instances.Instance) error {
	// Convert instances to API Workspace format
	workspaces := make([]api.Workspace, 0, len(instancesList))
	for _, instance := range instancesList {
		workspace := instanceToWorkspace(instance)
		workspaces = append(workspaces, workspace)
	}

	// Create WorkspacesList wrapper
	workspacesList := api.WorkspacesList{
		Items: workspaces,
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(workspacesList, "", "  ")
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to marshal workspaces to JSON: %w", err))
	}

	// Output the JSON to stdout
	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewWorkspaceListCmd() *cobra.Command {
	c := &workspaceListCmd{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered workspaces",
		Long:  "List all workspaces registered with kortex-cli init",
		Example: `# List all workspaces
kortex-cli workspace list

# List workspaces in JSON format
kortex-cli workspace list --output json

# List using short flag
kortex-cli workspace list -o json`,
		Args:    cobra.NoArgs,
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))

	return cmd
}
