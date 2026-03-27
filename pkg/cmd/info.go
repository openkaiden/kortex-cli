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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/runtimesetup"
	"github.com/kortex-hub/kortex-cli/pkg/version"
	"github.com/spf13/cobra"
)

// infoCmd contains the configuration for the info command
type infoCmd struct {
	output string
}

// preRun validates the parameters and flags
func (i *infoCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if i.output != "" && i.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", i.output)
	}

	return nil
}

// run executes the info command logic
func (i *infoCmd) run(cmd *cobra.Command, args []string) error {
	runtimes := runtimesetup.ListAvailable()

	// Discover agents from runtimes that implement AgentLister
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve storage directory path: %w", err))
	}

	runtimeStorageDir := filepath.Join(absStorageDir, "runtimes")
	agents, err := runtimesetup.ListAgents(runtimeStorageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to list agents: %w", err))
	}

	if i.output == "json" {
		return i.outputJSON(cmd, agents, runtimes)
	}

	// Text output
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Version: %s\n", version.Version)
	fmt.Fprintf(out, "Agents: %s\n", strings.Join(agents, ", "))
	fmt.Fprintf(out, "Runtimes: %s\n", strings.Join(runtimes, ", "))

	return nil
}

// outputJSON outputs the info response as JSON
func (i *infoCmd) outputJSON(cmd *cobra.Command, agents, runtimes []string) error {
	if agents == nil {
		agents = []string{}
	}
	if runtimes == nil {
		runtimes = []string{}
	}
	response := api.Info{
		Version:  version.Version,
		Agents:   agents,
		Runtimes: runtimes,
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to marshal info to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewInfoCmd() *cobra.Command {
	c := &infoCmd{}

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display information about kortex-cli",
		Example: `# Show info
kortex-cli info

# Show info in JSON format
kortex-cli info --output json

# Show info using short flag
kortex-cli info -o json`,
		Args:    cobra.NoArgs,
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))

	return cmd
}
