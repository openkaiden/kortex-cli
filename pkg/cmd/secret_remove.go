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

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/spf13/cobra"
)

type secretRemoveCmd struct {
	store  secret.Store
	output string
}

func (s *secretRemoveCmd) preRun(cmd *cobra.Command, args []string) error {
	if s.output != "" && s.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", s.output)
	}

	if s.output == "json" {
		cmd.SilenceErrors = true
	}

	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to resolve storage directory path: %w", err))
	}

	s.store = secret.NewStore(absStorageDir)
	return nil
}

func (s *secretRemoveCmd) run(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := s.store.Remove(name); err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to remove secret: %w", err))
	}

	if s.output == "json" {
		return s.outputJSON(cmd, name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Secret %q removed successfully\n", name)
	return nil
}

func (s *secretRemoveCmd) outputJSON(cmd *cobra.Command, name string) error {
	response := api.SecretName{Name: name}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to marshal to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewSecretRemoveCmd() *cobra.Command {
	c := &secretRemoveCmd{}

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a secret",
		Long:    "Remove a secret from the system keychain and from the kdn storage directory.",
		Example: `# Remove a secret by name
kdn secret remove my-github-token

# Remove a secret with JSON output
kdn secret remove my-github-token --output json

# Remove a secret with short flag
kdn secret remove my-github-token -o json`,
		Args:    cobra.ExactArgs(1),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))
	cmd.ValidArgsFunction = completeSecretName

	return cmd
}
