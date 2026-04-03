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

func NewStopCmd() *cobra.Command {
	// Create the workspace stop command
	workspaceStopCmd := NewWorkspaceStopCmd()

	// Create an alias command that delegates to workspace stop
	cmd := &cobra.Command{
		Use:               "stop NAME|ID",
		Short:             workspaceStopCmd.Short + " (alias for 'workspace stop')",
		Long:              workspaceStopCmd.Long,
		Example:           AdaptExampleForAlias(workspaceStopCmd.Example, "workspace stop", "stop"),
		Args:              workspaceStopCmd.Args,
		ValidArgsFunction: workspaceStopCmd.ValidArgsFunction,
		PreRunE:           workspaceStopCmd.PreRunE,
		RunE:              workspaceStopCmd.RunE,
	}

	// Copy flags from workspace stop command
	cmd.Flags().AddFlagSet(workspaceStopCmd.Flags())

	return cmd
}
