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
	"errors"
	"fmt"
	"path/filepath"

	"github.com/openkaiden/kdn/pkg/agentsetup"
	"github.com/openkaiden/kdn/pkg/credentialsetup"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtimesetup"
	"github.com/openkaiden/kdn/pkg/secretservicesetup"
	"github.com/spf13/cobra"
)

// workspaceTerminalCmd contains the configuration for the workspace terminal command
type workspaceTerminalCmd struct {
	manager  instances.Manager
	nameOrID string
	command  []string
}

// preRun validates the parameters and flags
func (w *workspaceTerminalCmd) preRun(cmd *cobra.Command, args []string) error {
	w.nameOrID = args[0]

	// Extract command from args[1:] if provided
	// If no command is provided, w.command will be empty and the runtime
	// will use the agent's configured terminal command
	if len(args) > 1 {
		w.command = args[1:]
	}

	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return fmt.Errorf("failed to read --storage flag: %w", err)
	}

	// Normalize storage path to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for storage directory: %w", err)
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Register all available runtimes
	if err := runtimesetup.RegisterAll(manager); err != nil {
		return fmt.Errorf("failed to register runtimes: %w", err)
	}

	if err := agentsetup.RegisterAll(manager); err != nil {
		return fmt.Errorf("failed to register agents: %w", err)
	}

	if err := secretservicesetup.RegisterAll(manager); err != nil {
		return fmt.Errorf("failed to register secret services: %w", err)
	}

	if err := credentialsetup.RegisterAll(manager); err != nil {
		return fmt.Errorf("failed to register credentials: %w", err)
	}

	w.manager = manager

	return nil
}

// run executes the workspace terminal command logic
func (w *workspaceTerminalCmd) run(cmd *cobra.Command, args []string) error {
	// Resolve name or ID to get the instance
	instance, err := w.manager.Get(w.nameOrID)
	if err != nil {
		if errors.Is(err, instances.ErrInstanceNotFound) {
			return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", w.nameOrID)
		}
		return err
	}

	// Get the actual ID (in case user provided a name)
	instanceID := instance.GetID()

	// Start terminal session
	err = w.manager.Terminal(cmd.Context(), instanceID, w.command)
	if err != nil {
		return err
	}

	return nil
}

func NewWorkspaceTerminalCmd() *cobra.Command {
	c := &workspaceTerminalCmd{}

	cmd := &cobra.Command{
		Use:   "terminal NAME|ID [COMMAND...]",
		Short: "Connect to a workspace with an interactive terminal",
		Long: `Connect to a workspace with an interactive terminal session.

The terminal command starts an interactive session inside a workspace instance.
If the workspace is not running, it will be automatically started first.
By default, it launches the agent command configured in the runtime. You can override
this by providing a custom command.`,
		Example: `# Connect using the default agent command (by ID)
kdn workspace terminal abc123

# Connect using the default agent command (by name)
kdn workspace terminal my-project

# Run a bash shell
kdn workspace terminal abc123 bash

# Run a command with flags (use -- to prevent kdn from parsing them)
kdn workspace terminal abc123 -- bash -c 'echo hello'`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeWorkspaceID,
		PreRunE:           c.preRun,
		RunE:              c.run,
	}

	return cmd
}
