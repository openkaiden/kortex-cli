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
	"fmt"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

// workspaceListCmd contains the configuration for the workspace list command
type workspaceListCmd struct {
	manager instances.Manager
}

// preRun validates the parameters and flags
func (w *workspaceListCmd) preRun(cmd *cobra.Command, args []string) error {
	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return fmt.Errorf("failed to read --storage flag: %w", err)
	}

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

	// Display the instances
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

func NewWorkspaceListCmd() *cobra.Command {
	c := &workspaceListCmd{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all registered workspaces",
		Long:    "List all workspaces registered with kortex-cli init",
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	// TODO: Add flags as needed

	return cmd
}
