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

	"github.com/kortex-hub/kortex-cli/pkg/logger"
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

// Start starts a previously created Podman container.
func (p *podmanRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	// Validate the ID parameter
	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Start the container
	stepLogger.Start(fmt.Sprintf("Starting container: %s", id), "Container started")
	if err := p.startContainer(ctx, id); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Get updated container information
	stepLogger.Start("Verifying container status", "Container status verified")
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to get container info after start: %w", err)
	}

	return info, nil
}

// startContainer starts a podman container by ID.
func (p *podmanRuntime) startContainer(ctx context.Context, id string) error {
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", id); err != nil {
		return fmt.Errorf("failed to start podman container: %w", err)
	}
	return nil
}
