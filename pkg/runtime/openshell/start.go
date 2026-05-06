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

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Start verifies an OpenShell sandbox exists and marks it as running.
// Since OpenShell sandboxes are always running after creation, this
// just clears the stopped state override and validates the sandbox exists.
func (r *openshellRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: sandbox ID is required", runtime.ErrInvalidParams)
	}

	// Ensure the gateway is running
	if err := r.ensureGatewayRunning(ctx); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to ensure gateway is running: %w", err)
	}

	// Verify the sandbox still exists
	step.Start(fmt.Sprintf("Starting sandbox: %s", id), "Sandbox started")
	state := r.querySandboxState(ctx, id)
	if state != api.WorkspaceStateRunning {
		err := fmt.Errorf("sandbox %s not found", id)
		step.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Re-read merged config and apply network policy
	data, readErr := r.readSandboxData(id)
	if readErr == nil {
		step.Start("Configuring network policy", "Network policy configured")
		wsCfg, loadErr := loadNetworkConfig(data.SourcePath, r.globalStorageDir, data.ProjectID, data.Agent)
		if loadErr != nil {
			step.Fail(loadErr)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to load network config: %w", loadErr)
		}
		if err := r.applyNetworkPolicy(ctx, id, wsCfg); err != nil {
			step.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to configure network policy: %w", err)
		}
	}

	// Start port forwarding for configured ports
	if readErr == nil && len(data.Ports) > 0 {
		step.Start("Setting up port forwarding", "Port forwarding established")
		if err := r.startPortForwards(ctx, id, data.Ports); err != nil {
			step.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to set up port forwarding: %w", err)
		}
	}

	// Clear any stopped override
	if err := r.states.Remove(id); err != nil {
		step.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to clear state override: %w", err)
	}

	info := map[string]string{"sandbox_name": id}
	if readErr == nil && len(data.Ports) > 0 {
		forwards := buildForwards(data.Ports)
		if forwardsJSON, err := json.Marshal(forwards); err == nil {
			info["forwards"] = string(forwardsJSON)
		}
	}

	return runtime.RuntimeInfo{
		ID:    id,
		State: api.WorkspaceStateRunning,
		Info:  info,
	}, nil
}

// startPortForwards starts background port forwards for each configured port.
// Passes nil writers so no pipes are created — the daemonized forwarder would
// otherwise hold them open and cause Run to block indefinitely.
func (r *openshellRuntime) startPortForwards(ctx context.Context, sandboxName string, ports []int) error {
	for _, port := range ports {
		if err := r.executor.Run(ctx, nil, nil,
			"forward", "start", "--background", fmt.Sprintf("%d", port), sandboxName,
		); err != nil {
			return fmt.Errorf("failed to forward port %d: %w", port, err)
		}
	}
	return nil
}
