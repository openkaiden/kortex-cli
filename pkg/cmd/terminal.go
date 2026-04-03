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

func NewTerminalCmd() *cobra.Command {
	// Create the workspace terminal command
	workspaceTerminalCmd := NewWorkspaceTerminalCmd()

	// Create an alias command that delegates to workspace terminal
	cmd := &cobra.Command{
		Use:               "terminal NAME|ID [COMMAND...]",
		Short:             workspaceTerminalCmd.Short + " (alias for 'workspace terminal')",
		Long:              workspaceTerminalCmd.Long,
		Example:           AdaptExampleForAlias(workspaceTerminalCmd.Example, "workspace terminal", "terminal"),
		Args:              workspaceTerminalCmd.Args,
		ValidArgsFunction: workspaceTerminalCmd.ValidArgsFunction,
		PreRunE:           workspaceTerminalCmd.PreRunE,
		RunE:              workspaceTerminalCmd.RunE,
	}

	// No flags to copy - terminal command uses arguments only

	return cmd
}
