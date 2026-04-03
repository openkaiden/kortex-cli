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

package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// cursorAgent is the implementation of Agent for Cursor.
type cursorAgent struct{}

// Compile-time check to ensure cursorAgent implements Agent interface
var _ Agent = (*cursorAgent)(nil)

// NewCursor creates a new Cursor agent implementation.
func NewCursor() Agent {
	return &cursorAgent{}
}

// Name returns the agent name.
func (c *cursorAgent) Name() string {
	return "cursor"
}

// SkipOnboarding modifies Cursor settings to skip onboarding prompts.
// It creates a .workspace-trusted file in the Cursor projects directory
// for the given workspace sources path.
func (c *cursorAgent) SkipOnboarding(settings map[string][]byte, workspaceSourcesPath string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	cursorDir := workspacePathToCursorDir(workspaceSourcesPath)
	filePath := fmt.Sprintf(".cursor/projects/%s/.workspace-trusted", cursorDir)

	content := map[string]interface{}{
		"trustedAt":     time.Now().UTC().Format(time.RFC3339Nano),
		"workspacePath": workspaceSourcesPath,
	}

	contentJSON, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cursor workspace-trusted file: %w", err)
	}

	settings[filePath] = contentJSON
	return settings, nil
}

// workspacePathToCursorDir converts a workspace path to the directory name
// used by Cursor in its projects directory. Cursor replaces '/' with '-'
// and the resulting string has any leading '-' stripped.
func workspacePathToCursorDir(workspacePath string) string {
	dir := strings.ReplaceAll(workspacePath, "/", "-")
	return strings.TrimLeft(dir, "-")
}
