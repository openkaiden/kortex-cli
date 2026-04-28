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

	l := logger.FromContext(ctx)

	// Query container names dynamically so the list stays correct if the pod
	// definition gains or loses containers in the future.
	// `podman pod stop` is a NOP when any container is already stopped/exited,
	// so we stop each container individually instead.
	output, err := p.executor.Output(ctx, l.Stderr(),
		"pod", "inspect", "--format", "{{range .Containers}}{{.Name}}\n{{end}}", podName)
	if err != nil {
		return fmt.Errorf("failed to inspect pod %s: %w", podName, err)
	}
	containerNames := strings.Fields(string(output))

	stepLogger.Start(fmt.Sprintf("Stopping pod: %s", podName), "Pod stopped")
	for _, c := range containerNames {
		if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "stop", c); err != nil {
			stepLogger.Fail(err)
			return fmt.Errorf("failed to stop container %s: %w", c, err)
		}
	}

	return nil
}
