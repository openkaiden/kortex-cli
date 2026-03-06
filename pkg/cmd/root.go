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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	// Compute default storage directory path cross-platform
	homeDir, err := os.UserHomeDir()
	defaultStoragePath := ".kortex-cli" // fallback to current directory
	if err == nil {
		defaultStoragePath = filepath.Join(homeDir, ".kortex-cli")
	}

	// Check for environment variable
	if envStorage := os.Getenv("KORTEX_CLI_STORAGE"); envStorage != "" {
		defaultStoragePath = envStorage
	}

	rootCmd := &cobra.Command{
		Use:   "kortex-cli",
		Short: "Launch and manage AI agent workspaces with custom configurations",
	}

	// Add subcommands
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewWorkspaceCmd())
	rootCmd.AddCommand(NewListCmd())

	// Global flags
	rootCmd.PersistentFlags().String("storage", defaultStoragePath, "Directory where kortex-cli will store all its files")

	return rootCmd
}
