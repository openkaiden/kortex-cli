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

// Stop stops a Podman container.
func (p *podmanRuntime) Stop(ctx context.Context, id string) error {
	logger := steplogger.FromContext(ctx)
	defer logger.Complete()

	// Validate the ID parameter
	if id == "" {
		return fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Stop the container
	logger.Start(fmt.Sprintf("Stopping container: %s", id), "Container stopped")
	if err := p.stopContainer(ctx, id); err != nil {
		logger.Fail(err)
		return err
	}

	return nil
}

// stopContainer stops a podman container by ID.
func (p *podmanRuntime) stopContainer(ctx context.Context, id string) error {
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "stop", id); err != nil {
		return fmt.Errorf("failed to stop podman container: %w", err)
	}
	return nil
}
