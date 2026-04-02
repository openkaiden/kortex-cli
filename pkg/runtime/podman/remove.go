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

package podman

import (
	"context"
	"fmt"
	"strings"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/logger"
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

// Remove removes a Podman container and its associated resources.
func (p *podmanRuntime) Remove(ctx context.Context, id string) error {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	// Validate the ID parameter
	if id == "" {
		return fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Check if the container exists and get its state
	stepLogger.Start("Checking container state", "Container state checked")
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		// If the container doesn't exist, treat it as already removed (idempotent)
		if isNotFoundError(err) {
			return nil
		}
		stepLogger.Fail(err)
		return err
	}

	// Check if the container is running
	if info.State == api.WorkspaceStateRunning {
		err := fmt.Errorf("container %s is still running, stop it first", id)
		stepLogger.Fail(err)
		return err
	}

	// Remove the container
	stepLogger.Start(fmt.Sprintf("Removing container: %s", id), "Container removed")
	if err := p.removeContainer(ctx, id); err != nil {
		stepLogger.Fail(err)
		return err
	}

	return nil
}

// removeContainer removes a podman container by ID.
func (p *podmanRuntime) removeContainer(ctx context.Context, id string) error {
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "rm", id); err != nil {
		return fmt.Errorf("failed to remove podman container: %w", err)
	}
	return nil
}

// isNotFoundError checks if an error indicates that a container was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// Check for podman-specific "not found" error messages
	return strings.Contains(errMsg, "no such container") ||
		strings.Contains(errMsg, "no such object") ||
		strings.Contains(errMsg, "error getting container") ||
		strings.Contains(errMsg, "failed to inspect container")
}
