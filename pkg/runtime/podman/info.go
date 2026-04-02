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
)

// mapPodmanState maps podman container states to valid WorkspaceState values.
// Podman states: https://docs.podman.io/en/latest/markdown/podman-ps.1.html
func mapPodmanState(podmanState string) api.WorkspaceState {
	switch podmanState {
	case "running":
		return api.WorkspaceStateRunning
	case "created", "exited", "stopped", "paused", "removing":
		return api.WorkspaceStateStopped
	case "dead":
		return api.WorkspaceStateError
	default:
		return api.WorkspaceStateUnknown
	}
}

// Info retrieves information about a Podman runtime instance.
func (p *podmanRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	// Validate the ID parameter
	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Get container information
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		return runtime.RuntimeInfo{}, err
	}

	return info, nil
}

// getContainerInfo retrieves detailed information about a container.
func (p *podmanRuntime) getContainerInfo(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	// Use podman inspect to get container details in a format we can parse
	// Format: ID|State|ImageName (custom fields from creation)
	l := logger.FromContext(ctx)
	output, err := p.executor.Output(ctx, l.Stderr(), "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse the output
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) != 3 {
		return runtime.RuntimeInfo{}, fmt.Errorf("unexpected inspect output format: %s", string(output))
	}

	containerID := fields[0]
	podmanState := fields[1]
	imageName := fields[2]

	// Map podman state to valid WorkspaceState
	state := mapPodmanState(podmanState)

	// Build the info map
	info := map[string]string{
		"container_id": containerID,
		"image_name":   imageName,
	}

	return runtime.RuntimeInfo{
		ID:    containerID,
		State: state,
		Info:  info,
	}, nil
}
