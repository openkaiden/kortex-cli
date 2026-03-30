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
	"github.com/kortex-hub/kortex-cli/pkg/logger"
	"github.com/kortex-hub/kortex-cli/pkg/runtimesetup"
	"github.com/kortex-hub/kortex-cli/pkg/steplogger"
	"github.com/spf13/cobra"
)

// workspaceStopCmd contains the configuration for the workspace stop command
type workspaceStopCmd struct {
	manager  instances.Manager
	id       string
	output   string
	showLogs bool
}

// preRun validates the parameters and flags
func (w *workspaceStopCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if w.output != "" && w.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", w.output)
	}

	if w.showLogs && w.output == "json" {
		return fmt.Errorf("--show-logs cannot be used with --output json")
	}

	// Silence Cobra's default error output to stderr when JSON mode is enabled,
	// because we write the error in the JSON response to stdout instead
	if w.output == "json" {
		cmd.SilenceErrors = true
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

// run executes the workspace stop command logic
func (w *workspaceStopCmd) run(cmd *cobra.Command, args []string) error {
	// Create appropriate step logger based on output mode
	var stepLogger steplogger.StepLogger
	if w.output == "json" {
		// No step logging in JSON mode
		stepLogger = steplogger.NewNoOpLogger()
	} else {
		stepLogger = steplogger.NewTextLogger(cmd.ErrOrStderr())
	}
	defer stepLogger.Complete()

	ctx := steplogger.WithLogger(cmd.Context(), stepLogger)

	// Create appropriate logger based on --show-logs flag
	var l logger.Logger
	if w.showLogs {
		l = logger.NewTextLogger(cmd.OutOrStdout(), cmd.ErrOrStderr())
	} else {
		l = logger.NewNoOpLogger()
	}
	ctx = logger.WithLogger(ctx, l)

	// Stop the instance
	err := w.manager.Stop(ctx, w.id)
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
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, w.id)
	return nil
}

// outputJSON outputs the workspace ID as JSON
func (w *workspaceStopCmd) outputJSON(cmd *cobra.Command) error {
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

func NewWorkspaceStopCmd() *cobra.Command {
	c := &workspaceStopCmd{}

	cmd := &cobra.Command{
		Use:   "stop ID",
		Short: "Stop a workspace",
		Long:  "Stop a workspace by its ID",
		Example: `# Stop workspace by ID
kortex-cli workspace stop abc123

# Stop workspace with JSON output
kortex-cli workspace stop abc123 --output json

# Stop workspace and show runtime command output
kortex-cli workspace stop abc123 --show-logs`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRunningWorkspaceID,
		PreRunE:           c.preRun,
		RunE:              c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.Flags().BoolVar(&c.showLogs, "show-logs", false, "Show stdout and stderr from runtime commands")

	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))

	return cmd
}
