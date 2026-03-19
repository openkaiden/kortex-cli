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

// Package podman provides a Podman runtime implementation for container-based workspaces.
package podman

import (
	"context"
	"fmt"

	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/system"
)

// podmanRuntime implements the runtime.Runtime interface for Podman.
type podmanRuntime struct {
	system system.System
}

// Ensure podmanRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*podmanRuntime)(nil)

// New creates a new Podman runtime instance.
func New() runtime.Runtime {
	return newWithSystem(system.New())
}

// newWithSystem creates a new Podman runtime instance with a custom system (for testing).
func newWithSystem(sys system.System) runtime.Runtime {
	return &podmanRuntime{
		system: sys,
	}
}

// Available implements runtimesetup.Available.
// It checks if the podman CLI is available on the system.
func (p *podmanRuntime) Available() bool {
	return p.system.CommandExists("podman")
}

// Type returns the runtime type identifier.
func (p *podmanRuntime) Type() string {
	return "podman"
}

// Create creates a new Podman runtime instance.
func (p *podmanRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}

// Start starts a Podman runtime instance.
func (p *podmanRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}

// Stop stops a Podman runtime instance.
func (p *podmanRuntime) Stop(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

// Remove removes a Podman runtime instance.
func (p *podmanRuntime) Remove(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

// Info retrieves information about a Podman runtime instance.
func (p *podmanRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}
