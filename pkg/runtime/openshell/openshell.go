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
	"sync"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

const (
	containerHome             = "/sandbox"
	containerWorkspaceSources = containerHome + "/workspace/sources"
	sandboxNamePrefix         = "kdn-"
)

// openshellRuntime implements the runtime.Runtime interface for OpenShell Gateway.
type openshellRuntime struct {
	executor              exec.Executor
	gatewayBinaryPath     string
	storageDir            string
	globalStorageDir      string
	states                *stateOverrides
	config                gatewayConfig
	binariesOnce          sync.Once
	binariesErr           error
	secretStore           secret.Store
	secretServiceRegistry secretservice.Registry
}

// Ensure openshellRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.StorageAware at compile time.
var _ runtime.StorageAware = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.Terminal at compile time.
var _ runtime.Terminal = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.SecretServiceRegistryAware at compile time.
var _ runtime.SecretServiceRegistryAware = (*openshellRuntime)(nil)

// Ensure openshellRuntime implements runtime.FlagProvider at compile time.
var _ runtime.FlagProvider = (*openshellRuntime)(nil)

// New creates a new OpenShell Gateway runtime instance.
func New() runtime.Runtime {
	return &openshellRuntime{}
}

// newWithDeps creates a new OpenShell Gateway runtime instance with custom dependencies (for testing).
func newWithDeps(executor exec.Executor, gatewayBinaryPath, storageDir string) *openshellRuntime {
	rt := &openshellRuntime{
		executor:          executor,
		gatewayBinaryPath: gatewayBinaryPath,
		storageDir:        storageDir,
		states:            newStateOverrides(storageDir),
		config:            loadConfig(storageDir),
	}
	// Mark binaries as resolved so ensureBinaries() is a no-op in tests.
	rt.binariesOnce.Do(func() {})
	return rt
}

// Available reports whether the OpenShell Gateway runtime is supported on the current platform.
func (r *openshellRuntime) Available() bool {
	_, err := platformAsset("openshell-gateway")
	return err == nil
}

// Flags implements runtime.FlagProvider.
func (r *openshellRuntime) Flags() []runtime.FlagDef {
	return []runtime.FlagDef{
		{
			Name:        "openshell-driver",
			Usage:       "OpenShell driver to use (podman, vm)",
			Completions: []string{"podman", "vm"},
		},
	}
}

// SetSecretServiceRegistry implements runtime.SecretServiceRegistryAware.
func (r *openshellRuntime) SetSecretServiceRegistry(reg secretservice.Registry) {
	r.secretServiceRegistry = reg
}

// Initialize implements runtime.StorageAware.
// It sets up directories and configuration. Binary downloads are deferred
// to first use (ensureBinaries) to avoid network calls during registration.
func (r *openshellRuntime) Initialize(storageDir string) error {
	if storageDir == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	r.storageDir = storageDir
	r.globalStorageDir = filepath.Dir(filepath.Dir(storageDir))
	r.states = newStateOverrides(storageDir)
	r.config = loadConfig(storageDir)
	r.secretStore = secret.NewStore(r.globalStorageDir)

	return nil
}

// ensureBinaries downloads the openshell, openshell-gateway, and
// openshell-driver-vm binaries if they are not already present.
// It is safe to call from multiple entry points — the download
// runs at most once per runtime instance.
func (r *openshellRuntime) ensureBinaries() error {
	r.binariesOnce.Do(func() {
		r.binariesErr = r.downloadBinaries()
	})
	return r.binariesErr
}

func (r *openshellRuntime) downloadBinaries() error {
	binDir := filepath.Join(r.storageDir, "bin")

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

	if _, assetErr := platformAsset("openshell-driver-vm"); assetErr == nil {
		vmDriverPath, dlErr := ensureBinary(binDir, "openshell-driver-vm", openshellDriverVMRelease)
		if dlErr != nil {
			return fmt.Errorf("failed to ensure openshell-driver-vm binary: %w", dlErr)
		}
		if err := codesignBinary(vmDriverPath); err != nil {
			return fmt.Errorf("failed to codesign openshell-driver-vm: %w", err)
		}
	}

	return nil
}

// Type returns the runtime type identifier.
func (r *openshellRuntime) Type() string {
	return "openshell"
}

// Description returns a human-readable description of the OpenShell runtime.
func (r *openshellRuntime) Description() string {
	return "Sandbox-based workspaces using OpenShell Gateway"
}

// Local reports whether the runtime executes workspaces locally.
func (r *openshellRuntime) Local() bool {
	return false
}

// WorkspaceSourcesPath returns the path where sources are mounted inside the sandbox.
func (r *openshellRuntime) WorkspaceSourcesPath() string {
	return containerWorkspaceSources
}

// sandboxName returns the prefixed sandbox name for a given instance name.
func sandboxName(name string) string {
	return sandboxNamePrefix + name
}
