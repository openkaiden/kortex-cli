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
	"strings"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
)

// Info retrieves information about an OpenShell sandbox.
func (r *openshellRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: sandbox ID is required", runtime.ErrInvalidParams)
	}

	// Check local state override first
	if state, ok := r.states.Get(id); ok {
		return runtime.RuntimeInfo{
			ID:    id,
			State: state,
			Info:  map[string]string{"sandbox_name": id},
		}, nil
	}

	// Query actual sandbox state
	state := r.querySandboxState(ctx, id)

	return runtime.RuntimeInfo{
		ID:    id,
		State: state,
		Info:  map[string]string{"sandbox_name": id},
	}, nil
}

// querySandboxState checks whether a sandbox exists and is running.
func (r *openshellRuntime) querySandboxState(ctx context.Context, id string) api.WorkspaceState {
	l := logger.FromContext(ctx)
	output, err := r.executor.Output(ctx, l.Stderr(), "sandbox", "list", "--names")
	if err != nil {
		return api.WorkspaceStateError
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == id {
			return api.WorkspaceStateRunning
		}
	}

	return api.WorkspaceStateError
}
