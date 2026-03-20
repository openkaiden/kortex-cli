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
	"errors"
	"fmt"
	"path/filepath"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/kortex-hub/kortex-cli/pkg/runtimesetup"
	"github.com/spf13/cobra"
)

// workspaceStartCmd contains the configuration for the workspace start command
type workspaceStartCmd struct {
	manager instances.Manager
	id      string
	output  string
}

// preRun validates the parameters and flags
func (w *workspaceStartCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if w.output != "" && w.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", w.output)
	}

	// Silence Cobra's error and usage output when JSON mode is enabled
	// This prevents "Error: ..." and usage info from being printed
	if w.output == "json" {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
	}

	w.id = args[0]

	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	// Normalize storage path to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to resolve absolute path for storage directory: %w", err))
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to create manager: %w", err))
	}

	// Register all available runtimes
	if err := runtimesetup.RegisterAll(manager); err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to register runtimes: %w", err))
	}

	w.manager = manager

	return nil
}

// run executes the workspace start command logic
func (w *workspaceStartCmd) run(cmd *cobra.Command, args []string) error {
	// Start the instance
	err := w.manager.Start(cmd.Context(), w.id)
	if err != nil {
		if errors.Is(err, instances.ErrInstanceNotFound) {
			if w.output == "json" {
				return outputErrorIfJSON(cmd, w.output, fmt.Errorf("workspace not found: %s", w.id))
			}
			return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", w.id)
		}
		return outputErrorIfJSON(cmd, w.output, err)
	}

	// Handle JSON output
	if w.output == "json" {
		return w.outputJSON(cmd)
	}

	// Output only the ID (text mode)
	cmd.Println(w.id)
	return nil
}

// outputJSON outputs the workspace ID as JSON
func (w *workspaceStartCmd) outputJSON(cmd *cobra.Command) error {
	// Return only the ID (per OpenAPI spec)
	workspaceId := api.WorkspaceId{
		Id: w.id,
	}

	jsonData, err := json.MarshalIndent(workspaceId, "", "  ")
	if err != nil {
		return outputErrorIfJSON(cmd, w.output, fmt.Errorf("failed to marshal to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewWorkspaceStartCmd() *cobra.Command {
	c := &workspaceStartCmd{}

	cmd := &cobra.Command{
		Use:   "start ID",
		Short: "Start a workspace",
		Long:  "Start a workspace by its ID",
		Example: `# Start workspace by ID
kortex-cli workspace start abc123

# Start workspace with JSON output
kortex-cli workspace start abc123 --output json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeNonRunningWorkspaceID,
		PreRunE:           c.preRun,
		RunE:              c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")

	return cmd
}
