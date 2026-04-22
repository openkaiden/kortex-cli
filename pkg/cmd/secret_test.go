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
	"bytes"
	"strings"
	"testing"
)

func TestSecretCmd(t *testing.T) {
	t.Parallel()

	cmd := NewSecretCmd()
	if cmd == nil {
		t.Fatal("NewSecretCmd() returned nil")
	}
	if cmd.Use != "secret" {
		t.Errorf("expected Use %q, got %q", "secret", cmd.Use)
	}

	subCmds := cmd.Commands()
	if len(subCmds) == 0 {
		t.Fatal("expected secret command to have subcommands")
	}

	foundCreate := false
	for _, sub := range subCmds {
		if sub.Use == "create <name>" {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		t.Error("expected secret command to have 'create' subcommand")
	}
}

func TestSecretCmd_UnknownCommand(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"secret", "foobar"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected Execute() to return an error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("expected 'foobar' in error, got: %s", err.Error())
	}
}
