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

	"github.com/fatih/color"
	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

type secretListCmd struct {
	store  secret.Store
	output string
}

func (s *secretListCmd) preRun(cmd *cobra.Command, args []string) error {
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

func (s *secretListCmd) run(cmd *cobra.Command, args []string) error {
	items, err := s.store.List()
	if err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to list secrets: %w", err))
	}

	if s.output == "json" {
		return s.outputJSON(cmd, items)
	}

	return s.displayTable(cmd, items)
}

func (s *secretListCmd) displayTable(cmd *cobra.Command, items []secret.ListItem) error {
	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(out, "No secrets found")
		return nil
	}

	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("NAME", "TYPE", "DESCRIPTION")
	tbl.WithWriter(out)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, item := range items {
		tbl.AddRow(item.Name, item.Type, item.Description)
	}

	tbl.Print()
	return nil
}

func (s *secretListCmd) outputJSON(cmd *cobra.Command, items []secret.ListItem) error {
	secrets := make([]api.SecretInfo, 0, len(items))
	for _, item := range items {
		info := api.SecretInfo{
			Name: item.Name,
			Type: item.Type,
		}
		if item.Description != "" {
			info.Description = &item.Description
		}
		if len(item.Hosts) > 0 {
			hosts := item.Hosts
			info.Hosts = &hosts
		}
		if item.Path != "" {
			info.Path = &item.Path
		}
		if item.Header != "" {
			info.Header = &item.Header
		}
		if item.HeaderTemplate != "" {
			info.HeaderTemplate = &item.HeaderTemplate
		}
		if len(item.Envs) > 0 {
			envs := item.Envs
			info.Envs = &envs
		}
		secrets = append(secrets, info)
	}

	response := api.SecretsList{Items: secrets}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return outputErrorIfJSON(cmd, s.output, fmt.Errorf("failed to marshal secrets to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewSecretListCmd() *cobra.Command {
	c := &secretListCmd{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all secrets",
		Long:  "List all secrets stored in the kdn storage directory",
		Example: `# List all secrets
kdn secret list

# List secrets in JSON format
kdn secret list --output json

# List using short flag
kdn secret list -o json`,
		Args:    cobra.NoArgs,
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))

	return cmd
}
