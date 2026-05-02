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

// Remove deletes an OpenShell sandbox.
func (r *openshellRuntime) Remove(ctx context.Context, id string) error {
	step := steplogger.FromContext(ctx)
	defer step.Complete()

	if id == "" {
		return fmt.Errorf("%w: sandbox ID is required", runtime.ErrInvalidParams)
	}

	if err := r.ensureBinaries(); err != nil {
		return err
	}

	// Check if sandbox is "stopped" (has override) — required before removal
	info, err := r.Info(ctx, id)
	if err != nil {
		return err
	}
	if info.State == api.WorkspaceStateRunning {
		return fmt.Errorf("sandbox %s is still running, stop it first", id)
	}

	// Delete the sandbox
	step.Start(fmt.Sprintf("Removing sandbox: %s", id), "Sandbox removed")
	l := logger.FromContext(ctx)
	if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), "sandbox", "delete", id); err != nil {
		step.Fail(err)
		return fmt.Errorf("failed to delete sandbox: %w", err)
	}

	// Clean up state override and sandbox data
	_ = r.states.Remove(id)
	r.removeSandboxData(id)

	return nil
}
