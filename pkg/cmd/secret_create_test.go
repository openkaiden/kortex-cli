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
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/spf13/cobra"
)

// fakeStore records Create calls for assertion in tests.
type fakeStore struct {
	params secret.CreateParams
	err    error
}

func (f *fakeStore) Create(params secret.CreateParams) error {
	f.params = params
	return f.err
}

// buildPreRunCmd creates a cobra.Command that mirrors the flag set seen by
// preRun when called through the real command tree.
func buildPreRunCmd(storageDir string) *cobra.Command {
	cmd := &cobra.Command{}
	// storage is a global persistent flag; add it directly so GetString works
	// in unit tests that bypass the full command tree.
	cmd.Flags().String("storage", storageDir, "")
	cmd.Flags().String("path", "", "")
	cmd.Flags().String("header", "", "")
	cmd.Flags().String("headerTemplate", "", "")
	cmd.Flags().StringArray("host", nil, "")
	return cmd
}

// testValidTypes is a fixed list used by unit tests that construct secretCreateCmd directly.
var testValidTypes = []string{"github", secret.TypeOther}

func TestSecretCreateCmd(t *testing.T) {
	t.Parallel()

	cmd := NewSecretCreateCmd()
	if cmd == nil {
		t.Fatal("NewSecretCreateCmd() returned nil")
	}
	if cmd.Use != "create <name>" {
		t.Errorf("expected Use %q, got %q", "create <name>", cmd.Use)
	}
}

func TestSecretCreateCmd_Examples(t *testing.T) {
	t.Parallel()

	cmd := NewSecretCreateCmd()
	if cmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	commands, err := testutil.ParseExampleCommands(cmd.Example)
	if err != nil {
		t.Fatalf("failed to parse examples: %v", err)
	}

	expectedCount := 3
	if len(commands) != expectedCount {
		t.Errorf("expected %d example commands, got %d", expectedCount, len(commands))
	}

	rootCmd := NewRootCmd()
	if err := testutil.ValidateCommandExamples(rootCmd, cmd.Example); err != nil {
		t.Errorf("example validation failed: %v", err)
	}
}

func TestSecretCreateCmd_PreRun(t *testing.T) {
	t.Parallel()

	t.Run("missing --type", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--type is required") {
			t.Errorf("expected '--type is required' error, got: %v", err)
		}
	})

	t.Run("invalid --type", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "custom", value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "invalid --type") {
			t.Errorf("expected 'invalid --type' error, got: %v", err)
		}
	})

	t.Run("missing --value", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--value is required") {
			t.Errorf("expected '--value is required' error, got: %v", err)
		}
	})

	t.Run("github with --host rejected", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", value: "v", hosts: []string{"example.com"}, validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--host is only valid when --type=other") {
			t.Errorf("expected '--host is only valid' error, got: %v", err)
		}
	})

	t.Run("github with --path rejected", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := cmd.Flags().Set("path", "/api"); err != nil {
			t.Fatal(err)
		}
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--path is only valid when --type=other") {
			t.Errorf("expected '--path is only valid' error, got: %v", err)
		}
	})

	t.Run("github with --header rejected", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := cmd.Flags().Set("header", "Authorization"); err != nil {
			t.Fatal(err)
		}
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--header is only valid when --type=other") {
			t.Errorf("expected '--header is only valid' error, got: %v", err)
		}
	})

	t.Run("github with --headerTemplate rejected", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := cmd.Flags().Set("headerTemplate", "Bearer ${value}"); err != nil {
			t.Fatal(err)
		}
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--headerTemplate is only valid when --type=other") {
			t.Errorf("expected '--headerTemplate is only valid' error, got: %v", err)
		}
	})

	t.Run("other without --host", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "other", value: "v", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--host is required when --type=other") {
			t.Errorf("expected '--host is required' error, got: %v", err)
		}
	})

	t.Run("other without --path", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "other", value: "v", hosts: []string{"example.com"}, validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--path is required when --type=other") {
			t.Errorf("expected '--path is required' error, got: %v", err)
		}
	})

	t.Run("other without --header", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "other", value: "v", hosts: []string{"example.com"}, validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := cmd.Flags().Set("path", "/"); err != nil {
			t.Fatal(err)
		}
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--header is required when --type=other") {
			t.Errorf("expected '--header is required' error, got: %v", err)
		}
	})

	t.Run("other without --headerTemplate", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "other", value: "v", hosts: []string{"example.com"}, validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := cmd.Flags().Set("path", "/"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("header", "Authorization"); err != nil {
			t.Fatal(err)
		}
		err := c.preRun(cmd, []string{"name"})
		if err == nil || !strings.Contains(err.Error(), "--headerTemplate is required when --type=other") {
			t.Errorf("expected '--headerTemplate is required' error, got: %v", err)
		}
	})

	t.Run("valid github params", func(t *testing.T) {
		t.Parallel()

		c := &secretCreateCmd{secretType: "github", value: "ghp_token", validTypes: testValidTypes}
		cmd := buildPreRunCmd(t.TempDir())
		if err := c.preRun(cmd, []string{"my-token"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.store == nil {
			t.Error("expected store to be initialised")
		}
	})
}

func TestSecretCreateCmd_Run(t *testing.T) {
	t.Parallel()

	t.Run("calls store with correct params", func(t *testing.T) {
		t.Parallel()

		fs := &fakeStore{}
		c := &secretCreateCmd{
			secretType:     "other",
			value:          "v",
			description:    "my description",
			hosts:          []string{"api.example.com"},
			path:           "/api",
			header:         "Authorization",
			headerTemplate: "Bearer ${value}",
			store:          fs,
			validTypes:     testValidTypes,
		}

		root := &cobra.Command{}
		var out strings.Builder
		root.SetOut(&out)
		child := &cobra.Command{RunE: c.run}
		root.AddCommand(child)

		if err := child.RunE(child, []string{"my-secret"}); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if fs.params.Name != "my-secret" {
			t.Errorf("Name: want %q, got %q", "my-secret", fs.params.Name)
		}
		if fs.params.Type != "other" {
			t.Errorf("Type: want %q, got %q", "other", fs.params.Type)
		}
		if fs.params.Value != "v" {
			t.Errorf("Value: want %q, got %q", "v", fs.params.Value)
		}
		if fs.params.Description != "my description" {
			t.Errorf("Description: want %q, got %q", "my description", fs.params.Description)
		}
		if len(fs.params.Hosts) != 1 || fs.params.Hosts[0] != "api.example.com" {
			t.Errorf("Hosts: want [api.example.com], got %v", fs.params.Hosts)
		}
	})

	t.Run("store error propagates", func(t *testing.T) {
		t.Parallel()

		fs := &fakeStore{err: os.ErrPermission}
		c := &secretCreateCmd{store: fs, validTypes: testValidTypes}

		cmd := &cobra.Command{}
		err := c.run(cmd, []string{"x"})
		if err == nil {
			t.Fatal("expected error when store fails")
		}
	})
}
