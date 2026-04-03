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

func TestListCmd(t *testing.T) {
	t.Parallel()

	cmd := NewListCmd()
	if cmd == nil {
		t.Fatal("NewListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Expected Use to be 'list', got '%s'", cmd.Use)
	}

	// Verify it includes the original workspace list Short description
	workspaceListCmd := NewWorkspaceListCmd()
	if !strings.Contains(cmd.Short, workspaceListCmd.Short) {
		t.Errorf("Expected Short to contain workspace list Short '%s', got '%s'", workspaceListCmd.Short, cmd.Short)
	}

	// Verify it includes the alias indicator
	if !strings.Contains(cmd.Short, "(alias for 'workspace list')") {
		t.Errorf("Expected Short to contain alias indicator, got '%s'", cmd.Short)
	}
}
