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
	"path/filepath"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

// initCmd contains the configuration for the init command
type initCmd struct {
	sourcesDir         string
	workspaceConfigDir string
	name               string
	absSourcesDir      string
	absConfigDir       string
	manager            instances.Manager
	verbose            bool
}

// preRun validates the parameters and flags
func (i *initCmd) preRun(cmd *cobra.Command, args []string) error {
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
	i.manager = manager

	// Get sources directory (default to current directory)
	i.sourcesDir = "."
	if len(args) > 0 {
		i.sourcesDir = args[0]
	}

	// Convert to absolute path for clarity
	absSourcesDir, err := filepath.Abs(i.sourcesDir)
	if err != nil {
		return fmt.Errorf("failed to resolve sources directory path: %w", err)
	}
	i.absSourcesDir = absSourcesDir

	// If workspace-configuration flag was not explicitly set, default to .kortex/ inside sources directory
	if !cmd.Flags().Changed("workspace-configuration") {
		i.workspaceConfigDir = filepath.Join(i.sourcesDir, ".kortex")
	}

	// Convert workspace config to absolute path
	absConfigDir, err := filepath.Abs(i.workspaceConfigDir)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace configuration directory path: %w", err)
	}
	i.absConfigDir = absConfigDir

	return nil
}

// run executes the init command logic
func (i *initCmd) run(cmd *cobra.Command, args []string) error {
	// Create a new instance
	instance, err := instances.NewInstance(instances.NewInstanceParams{
		SourceDir: i.absSourcesDir,
		ConfigDir: i.absConfigDir,
		Name:      i.name,
	})
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	// Add the instance to the manager
	addedInstance, err := i.manager.Add(instance)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	if i.verbose {
		cmd.Printf("Registered workspace:\n")
		cmd.Printf("  ID: %s\n", addedInstance.GetID())
		cmd.Printf("  Name: %s\n", addedInstance.GetName())
		cmd.Printf("  Sources directory: %s\n", addedInstance.GetSourceDir())
		cmd.Printf("  Configuration directory: %s\n", addedInstance.GetConfigDir())
	} else {
		cmd.Println(addedInstance.GetID())
	}

	return nil
}

func NewInitCmd() *cobra.Command {
	c := &initCmd{}

	cmd := &cobra.Command{
		Use:   "init [sources-directory]",
		Short: "Register a new workspace",
		Long: `Register a new workspace with the specified sources directory and configuration location.

The sources directory defaults to the current directory (.) if not specified.
The workspace configuration directory defaults to .kortex/ inside the sources directory if not specified.`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	// Add workspace-configuration flag
	cmd.Flags().StringVar(&c.workspaceConfigDir, "workspace-configuration", "", "Directory for workspace configuration (default: <sources-directory>/.kortex)")

	// Add name flag
	cmd.Flags().StringVarP(&c.name, "name", "n", "", "Name for the workspace (default: generated from sources directory)")

	// Add verbose flag
	cmd.Flags().BoolVarP(&c.verbose, "verbose", "v", false, "Show detailed output")

	return cmd
}
