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
	"fmt"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Stop marks an OpenShell sandbox as stopped without actually stopping it.
// OpenShell sandboxes cannot be paused, so this records a local state override.
// The sandbox continues running in the background.
func (r *openshellRuntime) Stop(ctx context.Context, id string) error {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if id == "" {
		return fmt.Errorf("%w: sandbox ID is required", runtime.ErrInvalidParams)
	}

	// Stop port forwards (best-effort — forwards may not exist)
	data, readErr := r.readSandboxData(id)
	if readErr == nil && len(data.Ports) > 0 {
		step.Start("Stopping port forwarding", "Port forwarding stopped")
		r.stopPortForwards(ctx, id, data.Ports)
	}

	step.Start(fmt.Sprintf("Stopping sandbox: %s", id), "Sandbox stopped")
	if err := r.states.Set(id, api.WorkspaceStateStopped); err != nil {
		step.Fail(err)
		return fmt.Errorf("failed to set stopped state: %w", err)
	}

	return nil
}

// stopPortForwards stops background port forwards. Best-effort — errors are ignored.
func (r *openshellRuntime) stopPortForwards(ctx context.Context, sandboxName string, ports []int) {
	if r.executor == nil {
		return
	}
	l := logger.FromContext(ctx)
	for _, port := range ports {
		_ = r.executor.Run(ctx, l.Stdout(), l.Stderr(),
			"forward", "stop", fmt.Sprintf("%d", port), sandboxName,
		)
	}
}
