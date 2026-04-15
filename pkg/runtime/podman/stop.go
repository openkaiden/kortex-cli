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

// Stop stops all containers in the workspace pod.
func (p *podmanRuntime) Stop(ctx context.Context, id string) error {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	if id == "" {
		return fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Resolve the pod name from the stored mapping
	podName, err := p.readPodName(id)
	if err != nil {
		return fmt.Errorf("failed to resolve pod name: %w", err)
	}

	// Stop the entire pod (all containers at once)
	stepLogger.Start(fmt.Sprintf("Stopping pod: %s", podName), "Pod stopped")
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "pod", "stop", podName); err != nil {
		stepLogger.Fail(err)
		return fmt.Errorf("failed to stop pod: %w", err)
	}

	return nil
}
