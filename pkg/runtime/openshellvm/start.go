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

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Start verifies an OpenShell sandbox exists and marks it as running.
// Since OpenShell sandboxes are always running after creation, this
// just clears the stopped state override and validates the sandbox exists.
func (r *openshellVMRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: sandbox ID is required", runtime.ErrInvalidParams)
	}

	// Ensure the VM is running
	if err := r.ensureVMRunning(ctx); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to ensure VM is running: %w", err)
	}

	// Verify the sandbox still exists
	step.Start(fmt.Sprintf("Starting sandbox: %s", id), "Sandbox started")
	state := r.querySandboxState(ctx, id)
	if state != api.WorkspaceStateRunning {
		err := fmt.Errorf("sandbox %s not found", id)
		step.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Clear any stopped override
	if err := r.states.Remove(id); err != nil {
		step.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to clear state override: %w", err)
	}

	return runtime.RuntimeInfo{
		ID:    id,
		State: api.WorkspaceStateRunning,
		Info:  map[string]string{"sandbox_name": id},
	}, nil
}
