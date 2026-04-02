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

// Package fake provides a fake runtime implementation for testing.
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
)

const (
	// storageFileName is the name of the file used to persist fake runtime instances
	storageFileName = "instances.json"
)

// fakeRuntime is a persistent fake runtime for testing.
// It stores instance state on disk when initialized with a storage directory via the StorageAware interface.
// If no storage is provided, it falls back to in-memory mode for backward compatibility.
type fakeRuntime struct {
	mu          sync.RWMutex
	instances   map[string]*instanceState
	nextID      int
	storageDir  string // Empty if not initialized with storage
	storageFile string // Empty if not initialized with storage
}

// instanceState tracks the state of a fake runtime instance.
type instanceState struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	State  api.WorkspaceState `json:"state"`
	Info   map[string]string  `json:"info"`
	Source string             `json:"source"`
}

// persistedData is the structure stored on disk
type persistedData struct {
	NextID    int                       `json:"next_id"`
	Instances map[string]*instanceState `json:"instances"`
}

// Ensure fakeRuntime implements runtime.Runtime at compile time.
var _ runtime.Runtime = (*fakeRuntime)(nil)

// Ensure fakeRuntime implements runtime.StorageAware at compile time.
var _ runtime.StorageAware = (*fakeRuntime)(nil)

// New creates a new fake runtime instance.
// The runtime will operate in memory-only mode until Initialize is called.
func New() runtime.Runtime {
	return &fakeRuntime{
		instances: make(map[string]*instanceState),
		nextID:    1,
	}
}

// Initialize implements runtime.StorageAware.
// It sets up the storage directory and loads any existing instances from disk.
func (f *fakeRuntime) Initialize(storageDir string) error {
	if storageDir == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.storageDir = storageDir
	f.storageFile = filepath.Join(storageDir, storageFileName)

	// Load existing instances from disk if the file exists
	return f.loadFromDisk()
}

// loadFromDisk loads instances from the storage file.
// Must be called with f.mu locked.
func (f *fakeRuntime) loadFromDisk() error {
	// If no storage is configured, skip loading
	if f.storageFile == "" {
		return nil
	}

	// If file doesn't exist, nothing to load
	if _, err := os.Stat(f.storageFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(f.storageFile)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	// Empty file case
	if len(data) == 0 {
		return nil
	}

	var persisted persistedData
	if err := json.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("failed to unmarshal storage data: %w", err)
	}

	f.nextID = persisted.NextID
	f.instances = persisted.Instances
	if f.instances == nil {
		f.instances = make(map[string]*instanceState)
	}

	// Harden each loaded instance by ensuring Info map is non-nil
	// to prevent panics in Start/Stop methods that write to Info map
	for _, inst := range f.instances {
		if inst.Info == nil {
			inst.Info = make(map[string]string)
		}
	}

	return nil
}

// saveToDisk saves instances to the storage file.
// Must be called with f.mu locked.
func (f *fakeRuntime) saveToDisk() error {
	// If no storage is configured, skip saving
	if f.storageFile == "" {
		return nil
	}

	persisted := persistedData{
		NextID:    f.nextID,
		Instances: f.instances,
	}

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage data: %w", err)
	}

	if err := os.WriteFile(f.storageFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

// Type returns the runtime type identifier.
func (f *fakeRuntime) Type() string {
	return "fake"
}

// Create creates a new fake runtime instance.
func (f *fakeRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	if params.Name == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: name is required", runtime.ErrInvalidParams)
	}
	if params.SourcePath == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: source path is required", runtime.ErrInvalidParams)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if instance already exists with same name
	for _, inst := range f.instances {
		if inst.Name == params.Name {
			return runtime.RuntimeInfo{}, fmt.Errorf("instance with name %s already exists", params.Name)
		}
	}

	// Generate sequential ID
	id := fmt.Sprintf("fake-%03d", f.nextID)
	f.nextID++

	// Create instance state
	info := map[string]string{
		"created_at": time.Now().Format(time.RFC3339),
		"source":     params.SourcePath,
	}

	// Add workspace config info if provided
	if params.WorkspaceConfig != nil {
		info["has_config"] = "true"
		if params.WorkspaceConfig.Environment != nil {
			info["env_vars_count"] = fmt.Sprintf("%d", len(*params.WorkspaceConfig.Environment))
		}
	}

	state := &instanceState{
		ID:     id,
		Name:   params.Name,
		State:  api.WorkspaceStateStopped,
		Source: params.SourcePath,
		Info:   info,
	}

	f.instances[id] = state

	// Persist to disk if storage is configured
	if err := f.saveToDisk(); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist instance: %w", err)
	}

	return runtime.RuntimeInfo{
		ID:    id,
		State: state.State,
		Info:  copyMap(state.Info),
	}, nil
}

// Start starts a fake runtime instance.
func (f *fakeRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	inst, exists := f.instances[id]
	if !exists {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: %s", runtime.ErrInstanceNotFound, id)
	}

	if inst.State == api.WorkspaceStateRunning {
		return runtime.RuntimeInfo{}, fmt.Errorf("instance %s is already running", id)
	}

	inst.State = api.WorkspaceStateRunning
	inst.Info["started_at"] = time.Now().Format(time.RFC3339)

	// Persist to disk if storage is configured
	if err := f.saveToDisk(); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist instance state: %w", err)
	}

	return runtime.RuntimeInfo{
		ID:    inst.ID,
		State: inst.State,
		Info:  copyMap(inst.Info),
	}, nil
}

// Stop stops a fake runtime instance.
func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	inst, exists := f.instances[id]
	if !exists {
		return fmt.Errorf("%w: %s", runtime.ErrInstanceNotFound, id)
	}

	if inst.State != api.WorkspaceStateRunning {
		return fmt.Errorf("instance %s is not running", id)
	}

	inst.State = api.WorkspaceStateStopped
	inst.Info["stopped_at"] = time.Now().Format(time.RFC3339)

	// Persist to disk if storage is configured
	if err := f.saveToDisk(); err != nil {
		return fmt.Errorf("failed to persist instance state: %w", err)
	}

	return nil
}

// Remove removes a fake runtime instance.
func (f *fakeRuntime) Remove(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	inst, exists := f.instances[id]
	if !exists {
		// Treat missing instances as already removed (idempotent operation).
		// This is safe because the instance isn't in memory or on disk.
		return nil
	}

	if inst.State == api.WorkspaceStateRunning {
		return fmt.Errorf("instance %s is still running, stop it first", id)
	}

	delete(f.instances, id)

	// Persist to disk if storage is configured
	if err := f.saveToDisk(); err != nil {
		return fmt.Errorf("failed to persist instance removal: %w", err)
	}

	return nil
}

// Info retrieves information about a fake runtime instance.
func (f *fakeRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	inst, exists := f.instances[id]
	if !exists {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: %s", runtime.ErrInstanceNotFound, id)
	}

	return runtime.RuntimeInfo{
		ID:    inst.ID,
		State: inst.State,
		Info:  copyMap(inst.Info),
	}, nil
}

// copyMap creates a shallow copy of a string map.
func copyMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
