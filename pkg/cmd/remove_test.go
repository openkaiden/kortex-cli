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
	"strings"
	"testing"
)

func TestRemoveCmd(t *testing.T) {
	t.Parallel()

	cmd := NewRemoveCmd()
	if cmd == nil {
		t.Fatal("NewRemoveCmd() returned nil")
	}

	if cmd.Use != "remove NAME|ID" {
		t.Errorf("Expected Use to be 'remove NAME|ID', got '%s'", cmd.Use)
	}

	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "rm" {
		t.Errorf("Expected Aliases to be [rm], got %v", cmd.Aliases)
	}

	// Verify it includes the original workspace remove Short description
	workspaceRemoveCmd := NewWorkspaceRemoveCmd()
	if !strings.Contains(cmd.Short, workspaceRemoveCmd.Short) {
		t.Errorf("Expected Short to contain workspace remove Short '%s', got '%s'", workspaceRemoveCmd.Short, cmd.Short)
	}

	// Verify it includes the alias indicator
	if !strings.Contains(cmd.Short, "(alias for 'workspace remove')") {
		t.Errorf("Expected Short to contain alias indicator, got '%s'", cmd.Short)
	}
}
