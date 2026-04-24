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
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtimesetup"
	"github.com/spf13/cobra"
)

// workspaceDashboardCmd contains the configuration for the workspace dashboard command
type workspaceDashboardCmd struct {
	manager     instances.Manager
	nameOrID    string
	openBrowser func(ctx context.Context, url string) error
}

// preRun validates the parameters and flags
func (w *workspaceDashboardCmd) preRun(cmd *cobra.Command, args []string) error {
	w.nameOrID = args[0]

	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return fmt.Errorf("failed to read --storage flag: %w", err)
	}

	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for storage directory: %w", err)
	}

	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if err := runtimesetup.RegisterAll(manager); err != nil {
		return fmt.Errorf("failed to register runtimes: %w", err)
	}

	w.manager = manager
	return nil
}

// run executes the workspace dashboard command logic
func (w *workspaceDashboardCmd) run(cmd *cobra.Command, args []string) error {
	url, err := w.manager.GetDashboardURL(cmd.Context(), w.nameOrID)
	if err != nil {
		if errors.Is(err, instances.ErrInstanceNotFound) {
			return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", w.nameOrID)
		}
		if errors.Is(err, instances.ErrDashboardNotSupported) {
			return fmt.Errorf("dashboard not supported for workspace %q", w.nameOrID)
		}
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), url)
	if w.openBrowser != nil {
		_ = w.openBrowser(cmd.Context(), url)
	}
	return nil
}

// openBrowser attempts to open the URL in the user's default browser.
// Start is used instead of Run so the browser process is launched non-blocking.
func openBrowser(ctx context.Context, url string) error {
	var browserCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		browserCmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		browserCmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		browserCmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	return browserCmd.Start()
}

func NewWorkspaceDashboardCmd() *cobra.Command {
	c := &workspaceDashboardCmd{
		openBrowser: openBrowser,
	}

	cmd := &cobra.Command{
		Use:   "dashboard NAME|ID",
		Short: "Open the dashboard for a workspace",
		Long:  "Open the dashboard for a running workspace in the default browser and print the URL",
		Example: `# Open dashboard by workspace ID
kdn workspace dashboard abc123

# Open dashboard by workspace name
kdn workspace dashboard my-project`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDashboardWorkspaceID,
		PreRunE:           c.preRun,
		RunE:              c.run,
	}

	return cmd
}
