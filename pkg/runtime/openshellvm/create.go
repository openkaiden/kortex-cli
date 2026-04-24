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

package openshellvm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Create creates a new OpenShell sandbox.
func (r *openshellVMRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if err := validateCreateParams(params); err != nil {
		return runtime.RuntimeInfo{}, err
	}

	// Ensure the VM is running
	if err := r.ensureVMRunning(ctx); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to ensure VM is running: %w", err)
	}

	name := sandboxName(params.Name)
	l := logger.FromContext(ctx)

	// Create the sandbox
	step.Start(fmt.Sprintf("Creating sandbox: %s", name), "Sandbox created")
	if err := r.createSandbox(ctx, name, l); err != nil {
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

	// Mark as stopped — the manager will call Start separately
	if err := r.states.Set(name, api.WorkspaceStateStopped); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to set initial state: %w", err)
	}

	return runtime.RuntimeInfo{
		ID:    name,
		State: api.WorkspaceStateStopped,
		Info: map[string]string{
			"sandbox_name": name,
			"source_path":  params.SourcePath,
			"agent":        params.Agent,
		},
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
func (r *openshellVMRuntime) createSandbox(ctx context.Context, name string, l logger.Logger) error {
	args := []string{
		"sandbox", "create",
		"--name", name,
		"--from", "base",
		"--no-tty",
		"--no-bootstrap",
		"--", "true",
	}
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
func (r *openshellVMRuntime) uploadAgentSettings(ctx context.Context, name string, settings map[string][]byte) error {
	l := logger.FromContext(ctx)

	tmpDir, err := os.MkdirTemp("", "kdn-agent-settings-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for relPath, content := range settings {
		destPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", relPath, err)
		}
		if err := os.WriteFile(destPath, content, 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}
	}

	return r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"sandbox", "upload", name, tmpDir, containerHome,
	)
}

// writeEnvFile writes environment variables to a file inside the sandbox using sandbox upload.
func (r *openshellVMRuntime) writeEnvFile(ctx context.Context, name string, params runtime.CreateParams) error {
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

	tmpDir, err := os.MkdirTemp("", "kdn-env-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	content := strings.Join(envLines, "\n") + "\n"
	envFilePath := filepath.Join(tmpDir, ".kdn-env")
	if err := os.WriteFile(envFilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	l := logger.FromContext(ctx)
	return r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"sandbox", "upload", name, tmpDir, containerHome,
	)
}
