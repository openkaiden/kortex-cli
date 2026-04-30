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

// Package openshell provides an OpenShell runtime implementation for sandbox-based workspaces.
package openshell

import (
	"fmt"
	"path/filepath"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

const (
	containerHome             = "/sandbox"
	containerWorkspaceSources = containerHome + "/workspace/sources"
	sandboxNamePrefix         = "kdn-"
)

// openshellRuntime implements the runtime.Runtime interface for OpenShell Gateway.
type openshellRuntime struct {
	executor          exec.Executor
	gatewayBinaryPath string
	storageDir        string
	states            *stateOverrides
	config            gatewayConfig
}

// Ensure openshellRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.StorageAware at compile time.
var _ runtime.StorageAware = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.Terminal at compile time.
var _ runtime.Terminal = (*openshellRuntime)(nil)

// New creates a new OpenShell Gateway runtime instance.
func New() runtime.Runtime {
	return &openshellRuntime{}
}

// newWithDeps creates a new OpenShell Gateway runtime instance with custom dependencies (for testing).
func newWithDeps(executor exec.Executor, gatewayBinaryPath, storageDir string) *openshellRuntime {
	return &openshellRuntime{
		executor:          executor,
		gatewayBinaryPath: gatewayBinaryPath,
		storageDir:        storageDir,
		states:            newStateOverrides(storageDir),
		config:            loadConfig(storageDir),
	}
}

// Available reports that OpenShell Gateway is always available since binaries are auto-downloaded.
func (r *openshellRuntime) Available() bool {
	return true
}

// Initialize implements runtime.StorageAware.
// It downloads the openshell, openshell-gateway, and openshell-driver-vm binaries
// if they are not already present.
func (r *openshellRuntime) Initialize(storageDir string) error {
	if storageDir == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	r.storageDir = storageDir

	binDir := filepath.Join(storageDir, "bin")

	gatewayPath, err := ensureBinary(binDir, "openshell-gateway", openshellGatewayRelease)
	if err != nil {
		return fmt.Errorf("failed to ensure openshell-gateway binary: %w", err)
	}
	r.gatewayBinaryPath = gatewayPath

	openshellPath, err := ensureBinary(binDir, "openshell", openshellRelease)
	if err != nil {
		return fmt.Errorf("failed to ensure openshell binary: %w", err)
	}
	r.executor = exec.New(openshellPath)

	vmDriverPath, err := ensureBinary(binDir, "openshell-driver-vm", openshellDriverVMRelease)
	if err != nil {
		return fmt.Errorf("failed to ensure openshell-driver-vm binary: %w", err)
	}
	if err := codesignBinary(vmDriverPath); err != nil {
		return fmt.Errorf("failed to codesign openshell-driver-vm: %w", err)
	}

	r.states = newStateOverrides(storageDir)
	r.config = loadConfig(storageDir)

	return nil
}

// Type returns the runtime type identifier.
func (r *openshellRuntime) Type() string {
	return "openshell"
}

// WorkspaceSourcesPath returns the path where sources are mounted inside the sandbox.
func (r *openshellRuntime) WorkspaceSourcesPath() string {
	return containerWorkspaceSources
}

// sandboxName returns the prefixed sandbox name for a given instance name.
func sandboxName(name string) string {
	return sandboxNamePrefix + name
}
