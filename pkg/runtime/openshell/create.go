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

package openshell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Create creates a new OpenShell sandbox.
func (r *openshellRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if err := validateCreateParams(params); err != nil {
		return runtime.RuntimeInfo{}, err
	}

	driver := params.RuntimeOptions["openshell-driver"]
	var allowHosts []string
	if v := params.RuntimeOptions["openshell-allow-hosts"]; v != "" {
		allowHosts = strings.Split(v, ",")
	}

	// Update driver in memory so ensureGatewayRunning uses the requested driver.
	// Config is persisted only after the gateway starts successfully.
	if driver != "" {
		r.config.Driver = driver
	}

	// Ensure the gateway is running (may fail on driver conflict)
	if err := r.ensureGatewayRunning(ctx); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to ensure gateway is running: %w", err)
	}

	if driver != "" {
		if err := saveConfig(r.storageDir, r.config); err != nil {
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to save driver config: %w", err)
		}
	}

	name := sandboxName(params.Name)
	l := logger.FromContext(ctx)

	// Collect ports from workspace config and agent defaults
	ports := collectPorts(params)

	// Create the sandbox
	step.Start(fmt.Sprintf("Creating sandbox: %s", name), "Sandbox created")
	if err := r.createSandbox(ctx, name, params.Agent, l); err != nil {
		step.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to create sandbox: %w", err)
	}

	// Upload sources
	step.Start("Uploading workspace sources", "Sources uploaded")
	if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"sandbox", "upload", name, params.SourcePath, containerWorkspaceSources,
	); err != nil {
		step.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to upload sources: %w", err)
	}

	// Upload agent settings
	if len(params.AgentSettings) > 0 {
		step.Start("Uploading agent settings", "Agent settings uploaded")
		if err := r.uploadAgentSettings(ctx, name, params.AgentSettings); err != nil {
			step.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to upload agent settings: %w", err)
		}
	}

	// Set environment variables
	if err := r.writeEnvFile(ctx, name, params); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to write env file: %w", err)
	}

	// Persist sandbox metadata for Start() to re-read network config
	if err := r.writeSandboxData(name, sandboxData{
		SourcePath: params.SourcePath,
		ProjectID:  params.ProjectID,
		Agent:      params.Agent,
		AllowHosts: allowHosts,
		Ports:      ports,
	}); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist sandbox data: %w", err)
	}

	// Configure network policy
	step.Start("Configuring network policy", "Network policy configured")
	if err := r.applyNetworkPolicy(ctx, name, params.WorkspaceConfig, allowHosts); err != nil {
		step.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to configure network policy: %w", err)
	}

	// Mark as stopped — the manager will call Start separately
	if err := r.states.Set(name, api.WorkspaceStateStopped); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to set initial state: %w", err)
	}

	info := map[string]string{
		"sandbox_name": name,
		"source_path":  params.SourcePath,
		"agent":        params.Agent,
	}
	if len(ports) > 0 {
		forwards := buildForwards(ports)
		if forwardsJSON, err := json.Marshal(forwards); err == nil {
			info["forwards"] = string(forwardsJSON)
		}
	}

	return runtime.RuntimeInfo{
		ID:    name,
		State: api.WorkspaceStateStopped,
		Info:  info,
	}, nil
}

func validateCreateParams(params runtime.CreateParams) error {
	if params.Name == "" {
		return fmt.Errorf("%w: name is required", runtime.ErrInvalidParams)
	}
	if params.SourcePath == "" {
		return fmt.Errorf("%w: source path is required", runtime.ErrInvalidParams)
	}
	if params.Agent == "" {
		return fmt.Errorf("%w: agent is required", runtime.ErrInvalidParams)
	}
	return nil
}

const (
	sandboxReadinessTimeout  = 2 * time.Minute
	sandboxReadinessInterval = 3 * time.Second
)

// createSandbox creates a new sandbox and waits for it to become ready.
// The create command may fail if the pod is still initializing when it tries
// to CONNECT. In that case, poll with exec until the sandbox becomes reachable.
func sandboxImage(agent string) (string, error) {
	switch agent {
	case "claude", "opencode", "codex", "copilot":
		return "base", nil
	case "gemini", "ollama":
		return agent, nil
	case "openclaw":
		return "quay.io/openkaiden/openshell-openclaw:2026.5.4", nil
	default:
		return "", fmt.Errorf("%w: unsupported agent %q", runtime.ErrInvalidParams, agent)
	}
}

func (r *openshellRuntime) createSandbox(ctx context.Context, name string, agent string, l logger.Logger) error {
	args := []string{"sandbox", "create", "--name", name}
	if r.config.Driver != DriverVM {
		image, err := sandboxImage(agent)
		if err != nil {
			return err
		}
		args = append(args, "--from", image)
	}
	args = append(args, "--no-tty", "--no-bootstrap", "--", "true")
	err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), args...)
	if err == nil {
		return nil
	}

	// Sandbox was created but SSH tunnel not ready yet — poll with exec
	deadline := time.Now().Add(sandboxReadinessTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sandboxReadinessInterval):
		}
		if execErr := r.executor.Run(ctx, l.Stdout(), l.Stderr(),
			"sandbox", "exec", "--name", name, "--no-tty", "--", "true",
		); execErr == nil {
			return nil
		}
	}

	return fmt.Errorf("sandbox did not become ready: %w", err)
}

// uploadAgentSettings writes agent settings files into the sandbox using sandbox upload.
func (r *openshellRuntime) uploadAgentSettings(ctx context.Context, name string, settings map[string][]byte) error {
	l := logger.FromContext(ctx)

	for relPath, content := range settings {
		tmpFile, err := os.CreateTemp("", "kdn-agent-setting-*")
		if err != nil {
			return fmt.Errorf("failed to create temp file for %s: %w", relPath, err)
		}
		tmpPath := tmpFile.Name()

		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write temp file for %s: %w", relPath, err)
		}
		tmpFile.Close()

		destPath := path.Join(containerHome, relPath)
		if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(),
			"sandbox", "upload", name, tmpPath, destPath,
		); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to upload %s: %w", relPath, err)
		}
		os.Remove(tmpPath)
	}

	return nil
}

// writeEnvFile writes environment variables to a file inside the sandbox using sandbox upload.
func (r *openshellRuntime) writeEnvFile(ctx context.Context, name string, params runtime.CreateParams) error {
	var envLines []string

	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Environment != nil {
		for _, env := range *params.WorkspaceConfig.Environment {
			if env.Value != nil && *env.Value != "" {
				envLines = append(envLines, fmt.Sprintf("export %s=%q", env.Name, *env.Value))
			}
		}
	}

	for k, v := range params.SecretEnvVars {
		envLines = append(envLines, fmt.Sprintf("export %s=%q", k, v))
	}

	if len(envLines) == 0 {
		return nil
	}

	tmpFile, err := os.CreateTemp("", "kdn-env-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	content := strings.Join(envLines, "\n") + "\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write env file: %w", err)
	}
	tmpFile.Close()

	destPath := path.Join(containerHome, ".kdn-env")
	l := logger.FromContext(ctx)
	return r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"sandbox", "upload", name, tmpPath, destPath,
	)
}

const openclawDefaultPort = 18789

// agentDefaultPorts returns ports that should be automatically forwarded for a given agent.
var agentDefaultPorts = map[string][]int{
	"openclaw": {openclawDefaultPort},
}

// collectPorts returns the deduplicated list of ports to forward, combining
// workspace config ports with agent-specific defaults.
func collectPorts(params runtime.CreateParams) []int {
	seen := make(map[int]bool)
	var ports []int

	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Ports != nil {
		for _, p := range *params.WorkspaceConfig.Ports {
			if !seen[p] {
				seen[p] = true
				ports = append(ports, p)
			}
		}
	}

	for _, p := range agentDefaultPorts[params.Agent] {
		if !seen[p] {
			seen[p] = true
			ports = append(ports, p)
		}
	}

	return ports
}

// buildForwards creates WorkspaceForward entries for the given ports.
// OpenShell uses the same port number on host and container sides.
func buildForwards(ports []int) []api.WorkspaceForward {
	forwards := make([]api.WorkspaceForward, len(ports))
	for i, port := range ports {
		forwards[i] = api.WorkspaceForward{
			Bind:   "127.0.0.1",
			Port:   port,
			Target: port,
		}
	}
	return forwards
}
