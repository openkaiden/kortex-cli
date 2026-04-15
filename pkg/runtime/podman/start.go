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

	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Start starts all containers in the workspace pod.
func (p *podmanRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Resolve the pod name from the stored mapping
	podName, err := p.readPodName(id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to resolve pod name: %w", err)
	}

	// Start the entire pod (all containers at once)
	stepLogger.Start(fmt.Sprintf("Starting pod: %s", podName), "Pod started")
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "pod", "start", podName); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to start pod: %w", err)
	}

	// Verify workspace container status
	stepLogger.Start("Verifying container status", "Container status verified")
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to get container info after start: %w", err)
	}

	return info, nil
}
