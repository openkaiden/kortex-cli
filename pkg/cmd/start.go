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
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	// Create the workspace start command
	workspaceStartCmd := NewWorkspaceStartCmd()

	// Create an alias command that delegates to workspace start
	cmd := &cobra.Command{
		Use:               "start NAME|ID",
		Short:             workspaceStartCmd.Short + " (alias for 'workspace start')",
		Long:              workspaceStartCmd.Long,
		Example:           AdaptExampleForAlias(workspaceStartCmd.Example, "workspace start", "start"),
		Args:              workspaceStartCmd.Args,
		ValidArgsFunction: workspaceStartCmd.ValidArgsFunction,
		PreRunE:           workspaceStartCmd.PreRunE,
		RunE:              workspaceStartCmd.RunE,
	}

	// Copy flags from workspace start command
	cmd.Flags().AddFlagSet(workspaceStartCmd.Flags())

	return cmd
}
