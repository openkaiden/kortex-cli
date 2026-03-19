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

	"github.com/kortex-hub/kortex-cli/pkg/runtime"
)

// Start starts a previously created Podman container.
func (p *podmanRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	// Validate the ID parameter
	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Start the container
	if err := p.startContainer(ctx, id); err != nil {
		return runtime.RuntimeInfo{}, err
	}

	// Get updated container information
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to get container info after start: %w", err)
	}

	return info, nil
}

// startContainer starts a podman container by ID.
func (p *podmanRuntime) startContainer(ctx context.Context, id string) error {
	if err := p.executor.Run(ctx, "start", id); err != nil {
		return fmt.Errorf("failed to start podman container: %w", err)
	}
	return nil
}

// getContainerInfo retrieves detailed information about a container.
func (p *podmanRuntime) getContainerInfo(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	// Use podman inspect to get container details in a format we can parse
	// Format: ID|State|ImageName (custom fields from creation)
	output, err := p.executor.Output(ctx, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse the output
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) != 3 {
		return runtime.RuntimeInfo{}, fmt.Errorf("unexpected inspect output format: %s", string(output))
	}

	containerID := fields[0]
	state := fields[1]
	imageName := fields[2]

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
