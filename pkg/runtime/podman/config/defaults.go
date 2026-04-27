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

	"github.com/openkaiden/kdn/pkg/runtime/podman/constants"
)

const (
	// DefaultVersion is the default Fedora version tag
	DefaultVersion = "latest"

	// ImageConfigFileName is the filename for base image configuration
	ImageConfigFileName = "image.json"

	// ClaudeConfigFileName is the filename for Claude agent configuration
	ClaudeConfigFileName = "claude.json"

	// GooseConfigFileName is the filename for Goose agent configuration
	GooseConfigFileName = "goose.json"

	// CursorConfigFileName is the filename for Cursor agent configuration
	CursorConfigFileName = "cursor.json"

	// OpenCodeConfigFileName is the filename for OpenCode agent configuration
	OpenCodeConfigFileName = "opencode.json"
)

// defaultImageConfig returns the default base image configuration.
func defaultImageConfig() *ImageConfig {
	return &ImageConfig{
		Version: DefaultVersion,
		Packages: []string{
			"which",
			"procps-ng",
			"wget2",
			"@development-tools",
			"jq",
			"gh",
			"golang",
			"golangci-lint",
			"python3",
			"python3-pip",
		},
		Sudo: []string{
			"/usr/bin/dnf",
			"/bin/nice",
			"/bin/kill",
			"/usr/bin/kill",
			"/usr/bin/killall",
		},
		RunCommands: []string{},
	}
}

// defaultClaudeConfig returns the default Claude agent configuration.
func defaultClaudeConfig() *AgentConfig {
	return &AgentConfig{
		Packages: []string{},
		RunCommands: []string{
			"curl -fsSL --proto-redir '-all,https' --tlsv1.3 https://claude.ai/install.sh | bash",
			fmt.Sprintf("mkdir -p /home/%s/.config", constants.ContainerUser),
		},
		TerminalCommand: []string{"claude"},
	}
}

// defaultGooseConfig returns the default Goose agent configuration.
func defaultGooseConfig() *AgentConfig {
	return &AgentConfig{
		Packages: []string{},
		RunCommands: []string{
			"cd /tmp && curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | CONFIGURE=false bash",
			fmt.Sprintf("mkdir -p /home/%s/.config/goose", constants.ContainerUser),
		},
		TerminalCommand: []string{"goose"},
	}
}

// defaultCursorConfig returns the default Cursor agent configuration.
func defaultCursorConfig() *AgentConfig {
	return &AgentConfig{
		Packages: []string{},
		RunCommands: []string{
			"curl https://cursor.com/install -fsS | bash",
		},
		TerminalCommand: []string{"agent"},
	}
}

// defaultOpenCodeConfig returns the default OpenCode agent configuration.
// The installer places the binary in ~/.opencode/bin/ which is not in the
// container's ENV PATH, so we symlink it into ~/.local/bin/.
func defaultOpenCodeConfig() *AgentConfig {
	return &AgentConfig{
		Packages: []string{},
		RunCommands: []string{
			"cd /tmp && curl -fsSL https://opencode.ai/install | bash",
			fmt.Sprintf("mkdir -p /home/%s/.local/bin && ln -sf /home/%s/.opencode/bin/opencode /home/%s/.local/bin/opencode", constants.ContainerUser, constants.ContainerUser, constants.ContainerUser),
			fmt.Sprintf("mkdir -p /home/%s/.config/opencode", constants.ContainerUser),
		},
		TerminalCommand: []string{"opencode"},
	}
}
