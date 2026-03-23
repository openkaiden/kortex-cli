// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"path/filepath"
)

// validateImageConfig validates the image configuration.
func validateImageConfig(cfg *ImageConfig) error {
	if cfg == nil {
		return fmt.Errorf("image config cannot be nil")
	}

	// Version cannot be empty
	if cfg.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Packages can be empty (valid use case)

	// Sudo binaries must be absolute paths
	for _, binary := range cfg.Sudo {
		if !filepath.IsAbs(binary) {
			return fmt.Errorf("sudo binary must be an absolute path: %s", binary)
		}
	}

	// RunCommands can be empty (valid use case)

	return nil
}

// validateAgentConfig validates the agent configuration.
func validateAgentConfig(cfg *AgentConfig) error {
	if cfg == nil {
		return fmt.Errorf("agent config cannot be nil")
	}

	// Packages can be empty (valid use case)

	// RunCommands can be empty (valid use case)

	// Terminal command must have at least one element
	if len(cfg.TerminalCommand) == 0 || cfg.TerminalCommand[0] == "" {
		return fmt.Errorf("terminal command must have at least one non-empty element")
	}

	return nil
}
