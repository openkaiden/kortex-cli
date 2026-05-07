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

// ImageConfig represents the base image configuration.
type ImageConfig struct {
	Version     string   `json:"version"`
	Packages    []string `json:"packages"`
	Sudo        []string `json:"sudo"`
	RunCommands []string `json:"run_commands"`
}

// AgentConfig represents agent-specific configuration.
type AgentConfig struct {
	Packages        []string          `json:"packages"`
	RunCommands     []string          `json:"run_commands"`
	TerminalCommand []string          `json:"terminal_command"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
}
