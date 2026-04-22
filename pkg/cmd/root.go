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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	// Compute default storage directory path cross-platform
	homeDir, err := os.UserHomeDir()
	defaultStoragePath := ".kdn" // fallback to current directory
	if err == nil {
		defaultStoragePath = filepath.Join(homeDir, ".kdn")
	}

	// Check for environment variable
	if envStorage := os.Getenv("KDN_STORAGE"); envStorage != "" {
		defaultStoragePath = envStorage
	}

	rootCmd := &cobra.Command{
		Use:   "kdn",
		Short: "Launch and manage AI agent workspaces with custom configurations",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return nil
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add command groups
	rootCmd.AddGroup(&cobra.Group{
		ID:    "main",
		Title: "Main Commands:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "workspace",
		Title: "Workspace Commands:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "secret",
		Title: "Secret Commands:",
	})

	// Add subcommands with groups
	initCmd := NewInitCmd()
	initCmd.GroupID = "main"
	rootCmd.AddCommand(initCmd)

	listCmd := NewListCmd()
	listCmd.GroupID = "main"
	rootCmd.AddCommand(listCmd)

	removeCmd := NewRemoveCmd()
	removeCmd.GroupID = "main"
	rootCmd.AddCommand(removeCmd)

	startCmd := NewStartCmd()
	startCmd.GroupID = "main"
	rootCmd.AddCommand(startCmd)

	stopCmd := NewStopCmd()
	stopCmd.GroupID = "main"
	rootCmd.AddCommand(stopCmd)

	terminalCmd := NewTerminalCmd()
	terminalCmd.GroupID = "main"
	rootCmd.AddCommand(terminalCmd)

	workspaceCmd := NewWorkspaceCmd()
	workspaceCmd.GroupID = "workspace"
	rootCmd.AddCommand(workspaceCmd)

	secretCmd := NewSecretCmd()
	secretCmd.GroupID = "secret"
	rootCmd.AddCommand(secretCmd)

	// Commands without a group (will appear under "Additional Commands")
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewInfoCmd())
	rootCmd.AddCommand(NewServiceCmd())

	// Global flags
	rootCmd.PersistentFlags().String("storage", defaultStoragePath, "Directory where kdn will store all its files")

	return rootCmd
}
