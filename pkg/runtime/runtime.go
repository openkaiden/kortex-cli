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

// Package runtime provides interfaces and types for managing AI agent workspace runtimes.
// A runtime is an execution environment (e.g., container, process) that hosts a workspace instance.
package runtime

import (
	"context"
	"fmt"

	api "github.com/openkaiden/kdn-api/cli/go"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
)

// Runtime manages the lifecycle of workspace instances in a specific execution environment.
// Implementations might use containers (podman, docker), processes, or other isolation mechanisms.
type Runtime interface {
	// Type returns the runtime type identifier (e.g., "podman", "docker", "process", "fake").
	Type() string

	// WorkspaceSourcesPath returns the path where sources will be mounted inside the workspace.
	// This is a constant for each runtime type and doesn't require an instance to exist.
	WorkspaceSourcesPath() string

	// Create creates a new runtime instance without starting it.
	// Returns RuntimeInfo with the assigned instance ID and initial state.
	Create(ctx context.Context, params CreateParams) (RuntimeInfo, error)

	// Start starts a previously created runtime instance.
	// Returns updated RuntimeInfo with running state.
	Start(ctx context.Context, id string) (RuntimeInfo, error)

	// Stop stops a running runtime instance without removing it.
	// The instance can be started again later.
	Stop(ctx context.Context, id string) error

	// Remove removes a runtime instance and cleans up all associated resources.
	// The instance must be stopped before removal.
	Remove(ctx context.Context, id string) error

	// Info retrieves current information about a runtime instance.
	Info(ctx context.Context, id string) (RuntimeInfo, error)
}

// CreateParams contains parameters for creating a new runtime instance.
type CreateParams struct {
	// Name is the human-readable name for the instance.
	Name string

	// SourcePath is the absolute path to the workspace source directory.
	SourcePath string

	// WorkspaceConfig is the workspace configuration (optional, can be nil if no configuration exists).
	WorkspaceConfig *workspace.WorkspaceConfiguration

	// Agent is the agent name for loading agent-specific configuration (required, cannot be empty).
	Agent string

	// AgentSettings contains the agent settings files to embed into the agent user's
	// home directory in the workspace image. Keys are relative file paths using
	// forward slashes (e.g., ".claude/settings.json"), values are file contents.
	// This map can be nil or empty if no default settings are configured.
	AgentSettings map[string][]byte

	// OnecliSecrets contains pre-mapped OneCLI secret definitions to provision
	// when the workspace is first started. These are created by the manager from
	// workspace configuration secrets. Can be nil or empty.
	OnecliSecrets []onecli.CreateSecretInput

	// SecretEnvVars maps environment variable names to placeholder values.
	// These are derived from secret service definitions (e.g. GH_TOKEN, GITHUB_TOKEN
	// for the "github" secret type) and injected into the workspace container so
	// that CLI tools detect a configured credential. Real auth goes through OneCLI proxy.
	SecretEnvVars map[string]string
}

// RuntimeInfo contains information about a runtime instance.
type RuntimeInfo struct {
	// ID is the runtime-assigned instance identifier.
	ID string

	// State is the current runtime state.
	State api.WorkspaceState

	// Info contains runtime-specific metadata as key-value pairs.
	// Examples: container_id, pid, created_at, network addresses.
	Info map[string]string
}

// AgentLister is an optional interface for runtimes that can report which agents they support.
// Runtimes implementing this interface enable discovery of available agents
// without requiring direct knowledge of the runtime's internal configuration.
//
// Example implementation:
//
//	type myRuntime struct {
//	    configDir string
//	}
//
//	func (r *myRuntime) ListAgents() ([]string, error) {
//	    // Scan configuration directory for agent definitions
//	    return []string{"claude", "goose"}, nil
//	}
type AgentLister interface {
	// ListAgents returns the names of all agents supported by this runtime.
	// Returns an empty slice if no agents are configured.
	ListAgents() ([]string, error)
}

// Dashboard is an optional interface for runtimes that provide a web dashboard.
// Runtimes implementing this interface expose a URL for accessing the workspace dashboard.
//
// Callers (e.g. the instances manager) are responsible for verifying that the
// instance is in the running state before invoking GetURL. Implementations
// may assume the instance is running and focus solely on resolving the URL.
//
// Example implementation:
//
//	func (r *myRuntime) GetURL(ctx context.Context, instanceID string) (string, error) {
//	    port, err := r.readStoredPort(instanceID)
//	    if err != nil {
//	        return "", err
//	    }
//	    return fmt.Sprintf("http://localhost:%d", port), nil
//	}
type Dashboard interface {
	// GetURL returns the dashboard URL for the given runtime instance ID.
	// The instance is guaranteed to be running when this method is called.
	// Returns an error if the dashboard URL cannot be determined.
	GetURL(ctx context.Context, instanceID string) (string, error)
}

// Terminal is an optional interface for runtimes that support interactive terminal sessions.
// Runtimes implementing this interface enable the terminal command for connecting to running instances.
//
// When a runtime implements this interface, users can:
//  1. Connect to running instances with an interactive terminal
//  2. Execute commands directly inside the instance environment
//  3. Interact with agents or shells running in the instance
//
// Example implementation:
//
//	type myRuntime struct {
//	    // ... other fields
//	}
//
//	func (r *myRuntime) Terminal(ctx context.Context, instanceID string, agent string, command []string) error {
//	    // Execute command interactively (stdin/stdout/stderr connected)
//	    return r.exec.RunInteractive(ctx, "exec", "-it", instanceID, command...)
//	}
type Terminal interface {
	// Terminal starts an interactive terminal session inside a running instance.
	// The command is executed with stdin/stdout/stderr connected directly to the user's terminal.
	// Returns an error if the instance is not running or command execution fails.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - instanceID: The runtime instance identifier
	//   - agent: The agent name for loading agent-specific configuration
	//   - command: The command to execute (e.g., ["bash"], ["claude-code", "--debug"]).
	//              If empty, the runtime will use the agent's configured terminal command.
	Terminal(ctx context.Context, instanceID string, agent string, command []string) error
}

// ValidateState validates that a runtime state is one of the valid WorkspaceState values.
// Valid states are: "running", "stopped", "error", "unknown".
// Returns an error if the state is not valid.
func ValidateState(state api.WorkspaceState) error {
	if !state.Valid() {
		return fmt.Errorf("invalid runtime state: %q (must be one of: running, stopped, error, unknown)", state)
	}
	return nil
}
