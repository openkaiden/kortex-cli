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
	"path/filepath"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

// stateFilter is a function that determines if an instance with the given state should be included
type stateFilter func(state string) bool

// getFilteredWorkspaceIDs retrieves workspace IDs, optionally filtered by state
func getFilteredWorkspaceIDs(cmd *cobra.Command, filter stateFilter) ([]string, cobra.ShellCompDirective) {
	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Normalize storage path to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// List all instances
	instancesList, err := manager.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Extract IDs with optional filtering
	var ids []string
	for _, instance := range instancesList {
		state := instance.GetRuntimeData().State
		// Apply filter if provided, otherwise include all
		if filter == nil || filter(state) {
			ids = append(ids, instance.GetID())
		}
	}

	return ids, cobra.ShellCompDirectiveNoFileComp
}

// completeNonRunningWorkspaceID provides completion for non-running workspaces (for start and remove)
// The args and toComplete parameters are part of Cobra's ValidArgsFunction signature but are unused
// because Cobra's shell completion framework automatically filters results based on user input.
func completeNonRunningWorkspaceID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return getFilteredWorkspaceIDs(cmd, func(state string) bool {
		return state != "running"
	})
}

// completeRunningWorkspaceID provides completion for running workspaces (for stop)
// The args and toComplete parameters are part of Cobra's ValidArgsFunction signature but are unused
// because Cobra's shell completion framework automatically filters results based on user input.
func completeRunningWorkspaceID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return getFilteredWorkspaceIDs(cmd, func(state string) bool {
		return state == "running"
	})
}
