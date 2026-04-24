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

// Package openshellvm provides an OpenShell VM runtime implementation for sandbox-based workspaces.
package openshellvm

import (
	"fmt"
	"path/filepath"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshellvm/exec"
)

const (
	containerHome             = "/sandbox"
	containerWorkspaceSources = containerHome + "/workspace/sources"
	sandboxNamePrefix         = "kdn-"
)

// openshellVMRuntime implements the runtime.Runtime interface for OpenShell.
type openshellVMRuntime struct {
	executor     exec.Executor
	vmBinaryPath string
	storageDir   string
	states       *stateOverrides
}

// Ensure openshellVMRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*openshellVMRuntime)(nil)

// Ensure openshellVMRuntime implements runtime.StorageAware at compile time.
var _ runtime.StorageAware = (*openshellVMRuntime)(nil)

// Ensure openshellVMRuntime implements runtime.Terminal at compile time.
var _ runtime.Terminal = (*openshellVMRuntime)(nil)

// New creates a new OpenShell runtime instance.
func New() runtime.Runtime {
	return &openshellVMRuntime{}
}

// newWithDeps creates a new OpenShell runtime instance with custom dependencies (for testing).
func newWithDeps(executor exec.Executor, vmBinaryPath, storageDir string) *openshellVMRuntime {
	return &openshellVMRuntime{
		executor:     executor,
		vmBinaryPath: vmBinaryPath,
		storageDir:   storageDir,
		states:       newStateOverrides(storageDir),
	}
}

// Available reports that OpenShell is always available since binaries are auto-downloaded.
func (r *openshellVMRuntime) Available() bool {
	return true
}

// Initialize implements runtime.StorageAware.
// It downloads the openshell and openshell-vm binaries if they are not already present.
func (r *openshellVMRuntime) Initialize(storageDir string) error {
	if storageDir == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	r.storageDir = storageDir

	binDir := filepath.Join(storageDir, "bin")

	vmPath, err := ensureBinary(binDir, "openshell-vm", openshellVMRelease)
	if err != nil {
		return fmt.Errorf("failed to ensure openshell-vm binary: %w", err)
	}
	r.vmBinaryPath = vmPath

	openshellPath, err := ensureBinary(binDir, "openshell", openshellRelease)
	if err != nil {
		return fmt.Errorf("failed to ensure openshell binary: %w", err)
	}
	r.executor = exec.New(openshellPath)
	r.states = newStateOverrides(storageDir)

	return nil
}

// Type returns the runtime type identifier.
func (r *openshellVMRuntime) Type() string {
	return "openshell-vm"
}

// WorkspaceSourcesPath returns the path where sources are mounted inside the sandbox.
func (r *openshellVMRuntime) WorkspaceSourcesPath() string {
	return containerWorkspaceSources
}

// sandboxName returns the prefixed sandbox name for a given instance name.
func sandboxName(name string) string {
	return sandboxNamePrefix + name
}
