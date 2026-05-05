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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtimesetup"
	"github.com/spf13/cobra"
)

// workspaceOpenCmd contains the configuration for the workspace open command
type workspaceOpenCmd struct {
	manager     instances.Manager
	nameOrID    string
	port        int // target port, 0 means not specified
	openBrowser func(ctx context.Context, url string) error
}

// preRun validates the parameters and flags
func (w *workspaceOpenCmd) preRun(cmd *cobra.Command, args []string) error {
	w.nameOrID = args[0]

	if len(args) > 1 {
		port, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid port %q: must be a number", args[1])
		}
		w.port = port
	}

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

// run executes the workspace open command logic
func (w *workspaceOpenCmd) run(cmd *cobra.Command, args []string) error {
	instance, err := w.manager.Get(w.nameOrID)
	if err != nil {
		if errors.Is(err, instances.ErrInstanceNotFound) {
			return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", w.nameOrID)
		}
		return err
	}

	forwards := instanceForwards(instance)
	if len(forwards) == 0 {
		return fmt.Errorf("no port forwards configured for workspace %q", w.nameOrID)
	}

	var forward *api.WorkspaceForward
	if w.port == 0 {
		if len(forwards) > 1 {
			return fmt.Errorf("workspace %q has multiple port forwards; specify a port", w.nameOrID)
		}
		forward = &forwards[0]
	} else {
		for i := range forwards {
			if forwards[i].Target == w.port {
				forward = &forwards[i]
				break
			}
		}
		if forward == nil {
			return fmt.Errorf("no port forward found for port %d in workspace %q", w.port, w.nameOrID)
		}
	}

	url := fmt.Sprintf("http://%s:%d", forward.Bind, forward.Port)
	fmt.Fprintln(cmd.OutOrStdout(), url)
	if w.openBrowser != nil {
		_ = w.openBrowser(cmd.Context(), url)
	}
	return nil
}

// instanceForwards extracts the port forwards from an instance's runtime data.
func instanceForwards(instance instances.Instance) []api.WorkspaceForward {
	runtimeData := instance.GetRuntimeData()
	forwardsJSON, ok := runtimeData.Info["forwards"]
	if !ok {
		return nil
	}
	var forwards []api.WorkspaceForward
	if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err != nil {
		return nil
	}
	return forwards
}

func NewWorkspaceOpenCmd() *cobra.Command {
	c := &workspaceOpenCmd{
		openBrowser: openBrowser,
	}

	cmd := &cobra.Command{
		Use:   "open NAME|ID [PORT]",
		Short: "Open a forwarded port of a workspace in the browser",
		Long:  "Open a forwarded port of a running workspace in the default browser and print the URL",
		Example: `# Open the only forwarded port of a workspace
kdn workspace open my-project

# Open a specific forwarded port of a workspace
kdn workspace open my-project 8080`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeOpenArgs,
		PreRunE:           c.preRun,
		RunE:              c.run,
	}

	return cmd
}
