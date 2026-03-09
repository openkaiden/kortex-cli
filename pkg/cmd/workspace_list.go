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
	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return fmt.Errorf("failed to read --storage flag: %w", err)
	}

	// Get output format flag
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("failed to read --output flag: %w", err)
	}

	// Validate output format if specified
	if output != "" && output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", output)
	}
	w.output = output

	// Create manager
	manager, err := instances.NewManager(storageDir)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}
	w.manager = manager

	return nil
}

// run executes the workspace list command logic
func (w *workspaceListCmd) run(cmd *cobra.Command, args []string) error {
	// Get all instances
	instancesList, err := w.manager.List()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// Handle JSON output format
	if w.output == "json" {
		return w.outputJSON(cmd, instancesList)
	}

	// Display the instances in text format
	if len(instancesList) == 0 {
		cmd.Println("No workspaces registered")
		return nil
	}

	for _, instance := range instancesList {
		cmd.Printf("ID: %s\n", instance.GetID())
		cmd.Printf("  Sources: %s\n", instance.GetSourceDir())
		cmd.Printf("  Configuration: %s\n", instance.GetConfigDir())
		cmd.Println()
	}

	return nil
}

// outputJSON converts instances to Workspace format and outputs as JSON
func (w *workspaceListCmd) outputJSON(cmd *cobra.Command, instancesList []instances.Instance) error {
	// Convert instances to API Workspace format
	workspaces := make([]api.Workspace, 0, len(instancesList))
	for _, instance := range instancesList {
		workspace := api.Workspace{
			Id:   instance.GetID(),
			Name: instance.GetName(),
			Paths: api.WorkspacePaths{
				Configuration: instance.GetConfigDir(),
				Source:        instance.GetSourceDir(),
			},
		}
		workspaces = append(workspaces, workspace)
	}

	// Create WorkspacesList wrapper
	workspacesList := api.WorkspacesList{
		Items: workspaces,
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(workspacesList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspaces to JSON: %w", err)
	}

	// Output the JSON to stdout
	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewWorkspaceListCmd() *cobra.Command {
	c := &workspaceListCmd{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all registered workspaces",
		Long:    "List all workspaces registered with kortex-cli init",
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().StringP("output", "o", "", "Output format (supported: json)")

	return cmd
}
