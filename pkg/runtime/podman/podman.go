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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/config"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/system"
)

// podmanRuntime implements the runtime.Runtime interface for Podman.
type podmanRuntime struct {
	system          system.System
	executor        exec.Executor
	storageDir      string                // Runtime-specific storage: <globalStorageDir>/runtimes/podman
	globalStorageDir string               // Top-level kdn storage dir: where config/projects.json lives
	config          config.Config         // Configuration manager for runtime settings
	onecliBaseURLFn func(port int) string // overridable in tests; nil uses default http://localhost:<port>
}

// onecliURL returns the base URL for the OneCLI service on the given port.
func (p *podmanRuntime) onecliURL(port int) string {
	if p.onecliBaseURLFn != nil {
		return p.onecliBaseURLFn(port)
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

// Ensure podmanRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*podmanRuntime)(nil)

// Ensure podmanRuntime implements runtime.StorageAware at compile time.
var _ runtime.StorageAware = (*podmanRuntime)(nil)

// Ensure podmanRuntime implements runtime.AgentLister at compile time.
var _ runtime.AgentLister = (*podmanRuntime)(nil)

// New creates a new Podman runtime instance.
func New() runtime.Runtime {
	return newWithDeps(system.New(), exec.New())
}

// newWithDeps creates a new Podman runtime instance with custom dependencies (for testing).
func newWithDeps(sys system.System, executor exec.Executor) runtime.Runtime {
	return &podmanRuntime{
		system:   sys,
		executor: executor,
	}
}

// Available implements runtimesetup.Available.
// It checks if the podman CLI is available on the system.
func (p *podmanRuntime) Available() bool {
	return p.system.CommandExists("podman")
}

// Initialize implements runtime.StorageAware.
// It sets up the storage directory for persisting runtime-specific data.
func (p *podmanRuntime) Initialize(storageDir string) error {
	if storageDir == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	p.storageDir = storageDir
	// storageDir is <globalStorageDir>/runtimes/podman; the global dir is two levels up.
	p.globalStorageDir = filepath.Dir(filepath.Dir(storageDir))

	// Create config directory
	configDir := filepath.Join(storageDir, "config")

	// Create config instance
	cfg, err := config.NewConfig(configDir)
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}
	p.config = cfg

	// Generate default configurations if they don't exist
	if err := p.config.GenerateDefaults(); err != nil {
		return fmt.Errorf("failed to generate default configs: %w", err)
	}

	return nil
}

const (
	podYAMLFile = "onecli-pod.yaml"
	podNameFile = "podname"
)

// podDir returns the directory for storing pod metadata for a given container ID.
func (p *podmanRuntime) podDir(containerID string) string {
	return filepath.Join(p.storageDir, "pods", containerID)
}

// podYAMLPath returns the path to the pod YAML for a given container ID.
func (p *podmanRuntime) podYAMLPath(containerID string) string {
	return filepath.Join(p.podDir(containerID), podYAMLFile)
}

// podNamePath returns the path to the pod name file for a given container ID.
func (p *podmanRuntime) podNamePath(containerID string) string {
	return filepath.Join(p.podDir(containerID), podNameFile)
}

// writePodFiles writes the per-workspace pod YAML and pod name file.
// The YAML is rendered from the embedded template using the supplied data.
func (p *podmanRuntime) writePodFiles(containerID string, data podTemplateData) error {
	dir := p.podDir(containerID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create pod directory: %w", err)
	}

	if err := writePodYAMLFile(p.podYAMLPath(containerID), data); err != nil {
		return err
	}
	if err := os.WriteFile(p.podNamePath(containerID), []byte(data.Name), 0644); err != nil {
		return fmt.Errorf("failed to write pod name file: %w", err)
	}

	tmplJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal pod template data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, podTemplateDataFile), tmplJSON, 0644); err != nil {
		return fmt.Errorf("failed to write pod template data: %w", err)
	}
	return nil
}

// readPodName reads the pod name for a given container ID from the stored file.
func (p *podmanRuntime) readPodName(containerID string) (string, error) {
	data, err := os.ReadFile(p.podNamePath(containerID))
	if err != nil {
		return "", fmt.Errorf("failed to read pod name: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// cleanupPodFiles removes the pod metadata directory for a given container ID.
func (p *podmanRuntime) cleanupPodFiles(containerID string) {
	os.RemoveAll(p.podDir(containerID))
}

const (
	podTemplateDataFile = "pod-template-data.json"
	caContainerPathFile = "ca-container-path"
)

// writeCAContainerPath persists the CA certificate container path for a workspace.
func (p *podmanRuntime) writeCAContainerPath(containerID, caPath string) error {
	return os.WriteFile(filepath.Join(p.podDir(containerID), caContainerPathFile), []byte(caPath), 0644)
}

// readCAContainerPath reads the persisted CA certificate container path.
// Returns empty string if not set.
func (p *podmanRuntime) readCAContainerPath(containerID string) string {
	data, err := os.ReadFile(filepath.Join(p.podDir(containerID), caContainerPathFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readPodTemplateData reads the persisted pod template data for a container.
func (p *podmanRuntime) readPodTemplateData(containerID string) (podTemplateData, error) {
	path := filepath.Join(p.podDir(containerID), podTemplateDataFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return podTemplateData{}, fmt.Errorf("failed to read pod template data: %w", err)
	}
	var tmplData podTemplateData
	if err := json.Unmarshal(data, &tmplData); err != nil {
		return podTemplateData{}, fmt.Errorf("failed to unmarshal pod template data: %w", err)
	}
	return tmplData, nil
}

// ListAgents implements runtime.AgentLister.
// It returns the names of all configured agents by delegating to the config manager.
// Returns an empty slice if the runtime has not been initialized.
func (p *podmanRuntime) ListAgents() ([]string, error) {
	if p.config == nil {
		return []string{}, nil
	}
	return p.config.ListAgents()
}

// Type returns the runtime type identifier.
func (p *podmanRuntime) Type() string {
	return "podman"
}

// WorkspaceSourcesPath returns the path where sources are mounted inside the workspace.
func (p *podmanRuntime) WorkspaceSourcesPath() string {
	return containerWorkspaceSources
}
