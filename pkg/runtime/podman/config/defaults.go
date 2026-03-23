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

const (
	// DefaultVersion is the default Fedora version tag
	DefaultVersion = "latest"

	// ImageConfigFileName is the filename for base image configuration
	ImageConfigFileName = "image.json"

	// ClaudeConfigFileName is the filename for Claude agent configuration
	ClaudeConfigFileName = "claude.json"
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
			"mkdir -p /home/claude/.config",
		},
		TerminalCommand: []string{"claude"},
	}
}
