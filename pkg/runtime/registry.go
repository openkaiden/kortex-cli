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

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Registry manages available runtime implementations.
type Registry interface {
	// Register adds a runtime to the registry.
	// Returns an error if a runtime with the same type is already registered.
	Register(runtime Runtime) error

	// Get retrieves a runtime by type.
	// Returns ErrRuntimeNotFound if the runtime type is not registered.
	Get(runtimeType string) (Runtime, error)

	// List returns all registered runtime types.
	List() []string
}

// StorageAware is an optional interface that runtimes can implement
// to receive a dedicated storage directory during registration.
//
// When a runtime implements this interface, the Registry will:
//  1. Create a directory at REGISTRY_STORAGE/runtimeType
//  2. Call Initialize with the absolute path to this directory
//  3. The runtime can use this directory to persist private data
//
// Example implementation:
//
//	type myRuntime struct {
//	    storageDir string
//	}
//
//	func (r *myRuntime) Initialize(storageDir string) error {
//	    r.storageDir = storageDir
//	    // Optional: create subdirectories, load state, etc.
//	    return os.MkdirAll(filepath.Join(storageDir, "instances"), 0755)
//	}
//
//	func (r *myRuntime) Create(ctx context.Context, params CreateParams) (RuntimeInfo, error) {
//	    // Use r.storageDir to persist instance data
//	    instanceFile := filepath.Join(r.storageDir, "instances", id+".json")
//	    return os.WriteFile(instanceFile, data, 0644)
//	}
type StorageAware interface {
	// Initialize is called during registration with the runtime's private storage directory.
	// The directory is created before this method is called and is guaranteed to exist.
	// The path will be: REGISTRY_STORAGE/runtimeType
	//
	// Runtimes should store the directory path and use it for persisting private data.
	// This method is called exactly once during registration, before the runtime is available for use.
	Initialize(storageDir string) error
}

// registry is the concrete implementation of Registry.
type registry struct {
	mu         sync.RWMutex
	runtimes   map[string]Runtime
	storageDir string
}

// Ensure registry implements Registry interface at compile time.
var _ Registry = (*registry)(nil)

// NewRegistry creates a new runtime registry with the specified storage directory.
// The storage directory is used to create runtime-specific subdirectories at REGISTRY_STORAGE/runtimeType.
func NewRegistry(storageDir string) (Registry, error) {
	if storageDir == "" {
		return nil, fmt.Errorf("storage directory cannot be empty")
	}

	// Convert to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve storage directory: %w", err)
	}

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(absStorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &registry{
		runtimes:   make(map[string]Runtime),
		storageDir: absStorageDir,
	}, nil
}

// Register adds a runtime to the registry.
func (r *registry) Register(runtime Runtime) error {
	if runtime == nil {
		return fmt.Errorf("runtime cannot be nil")
	}

	runtimeType := runtime.Type()
	if runtimeType == "" {
		return fmt.Errorf("runtime type cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runtimes[runtimeType]; exists {
		return fmt.Errorf("runtime already registered: %s", runtimeType)
	}

	// Initialize runtime with storage directory if it supports it
	if storageAware, ok := runtime.(StorageAware); ok {
		// Create runtime-specific storage directory
		runtimeStorageDir := filepath.Join(r.storageDir, runtimeType)
		if err := os.MkdirAll(runtimeStorageDir, 0755); err != nil {
			return fmt.Errorf("failed to create runtime storage directory: %w", err)
		}

		if err := storageAware.Initialize(runtimeStorageDir); err != nil {
			return fmt.Errorf("failed to initialize runtime storage: %w", err)
		}
	}

	r.runtimes[runtimeType] = runtime
	return nil
}

// Get retrieves a runtime by type.
func (r *registry) Get(runtimeType string) (Runtime, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtime, exists := r.runtimes[runtimeType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrRuntimeNotFound, runtimeType)
	}

	return runtime, nil
}

// List returns all registered runtime types.
func (r *registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.runtimes))
	for runtimeType := range r.runtimes {
		types = append(types, runtimeType)
	}

	return types
}
