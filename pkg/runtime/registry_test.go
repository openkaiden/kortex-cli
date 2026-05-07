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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeRuntime is a minimal Runtime implementation for testing.
type fakeRuntime struct {
	typeID string
}

func (f *fakeRuntime) Type() string        { return f.typeID }
func (f *fakeRuntime) DisplayName() string { return f.typeID }
func (f *fakeRuntime) Description() string { return "fake runtime for testing" }
func (f *fakeRuntime) Local() bool         { return true }
func (f *fakeRuntime) WorkspaceSourcesPath() string {
	return "/workspace/sources"
}

func (f *fakeRuntime) Create(ctx context.Context, params CreateParams) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func (f *fakeRuntime) Start(ctx context.Context, id string) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRuntime) Remove(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRuntime) Info(ctx context.Context, id string) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	rt := &fakeRuntime{typeID: "test-runtime"}

	// Register the runtime
	err = reg.Register(rt)
	if err != nil {
		t.Fatalf("Failed to register runtime: %v", err)
	}

	// Retrieve the runtime
	retrieved, err := reg.Get("test-runtime")
	if err != nil {
		t.Fatalf("Failed to get runtime: %v", err)
	}

	if retrieved.Type() != "test-runtime" {
		t.Errorf("Expected runtime type 'test-runtime', got '%s'", retrieved.Type())
	}
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	rt1 := &fakeRuntime{typeID: "test-runtime"}
	rt2 := &fakeRuntime{typeID: "test-runtime"}

	// Register first runtime
	err = reg.Register(rt1)
	if err != nil {
		t.Fatalf("Failed to register first runtime: %v", err)
	}

	// Try to register duplicate
	err = reg.Register(rt2)
	if err == nil {
		t.Fatal("Expected error when registering duplicate runtime, got nil")
	}

	expectedMsg := "runtime already registered: test-runtime"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRegistry_GetUnknownRuntime(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Try to get non-existent runtime
	_, err = reg.Get("unknown-runtime")
	if err == nil {
		t.Fatal("Expected error when getting unknown runtime, got nil")
	}

	if !errors.Is(err, ErrRuntimeNotFound) {
		t.Errorf("Expected ErrRuntimeNotFound, got %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Empty registry
	types := reg.List()
	if len(types) != 0 {
		t.Errorf("Expected empty list, got %d types", len(types))
	}

	// Register multiple runtimes
	rt1 := &fakeRuntime{typeID: "runtime-1"}
	rt2 := &fakeRuntime{typeID: "runtime-2"}

	if err := reg.Register(rt1); err != nil {
		t.Fatalf("Failed to register runtime-1: %v", err)
	}
	if err := reg.Register(rt2); err != nil {
		t.Fatalf("Failed to register runtime-2: %v", err)
	}

	// List should contain both
	types = reg.List()
	if len(types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(types))
	}

	// Check both types are present (order not guaranteed)
	typeMap := make(map[string]bool)
	for _, typ := range types {
		typeMap[typ] = true
	}

	if !typeMap["runtime-1"] || !typeMap["runtime-2"] {
		t.Errorf("Expected both runtime-1 and runtime-2 in list, got %v", types)
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	err = reg.Register(nil)
	if err == nil {
		t.Fatal("Expected error when registering nil runtime, got nil")
	}

	expectedMsg := "runtime cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRegistry_RegisterEmptyType(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	rt := &fakeRuntime{typeID: ""}

	err = reg.Register(rt)
	if err == nil {
		t.Fatal("Expected error when registering runtime with empty type, got nil")
	}

	expectedMsg := "runtime type cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRegistry_ThreadSafety(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Spawn multiple goroutines that register, get, and list concurrently
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		i := i // capture loop variable
		go func() {
			defer func() { done <- true }()

			// Register a unique runtime
			rt := &fakeRuntime{typeID: fmt.Sprintf("runtime-%d", i)}
			if err := reg.Register(rt); err != nil {
				t.Errorf("Failed to register runtime-%d: %v", i, err)
				return
			}

			// Try to get it
			retrieved, err := reg.Get(fmt.Sprintf("runtime-%d", i))
			if err != nil {
				t.Errorf("Failed to get runtime-%d: %v", i, err)
				return
			}

			if retrieved.Type() != fmt.Sprintf("runtime-%d", i) {
				t.Errorf("Wrong runtime type: expected runtime-%d, got %s", i, retrieved.Type())
			}

			// List all runtimes
			_ = reg.List()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all runtimes were registered
	types := reg.List()
	if len(types) != numGoroutines {
		t.Errorf("Expected %d registered runtimes, got %d", numGoroutines, len(types))
	}
}

func TestNewRegistry_EmptyStorageDir(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry("")
	if err == nil {
		t.Fatal("Expected error when creating registry with empty storage directory, got nil")
	}

	expectedMsg := "storage directory cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewRegistry_CreatesStorageDir(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	nestedDir := filepath.Join(storageDir, "nested", "path")

	reg, err := NewRegistry(nestedDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	if reg == nil {
		t.Fatal("Expected registry to be created, got nil")
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Errorf("Expected storage directory to be created at %s", nestedDir)
	}
}

func TestRegistry_CreatesRuntimeStorageDir(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Only StorageAware runtimes should have directories created
	rt := &storageAwareRuntime{typeID: "test-runtime"}
	err = reg.Register(rt)
	if err != nil {
		t.Fatalf("Failed to register runtime: %v", err)
	}

	// Verify runtime storage directory was created
	expectedPath := filepath.Join(storageDir, "test-runtime")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected runtime storage directory to be created at %s", expectedPath)
	}

	// Verify non-StorageAware runtimes do NOT have directories created
	rt2 := &fakeRuntime{typeID: "non-storage-runtime"}
	err = reg.Register(rt2)
	if err != nil {
		t.Fatalf("Failed to register non-storage runtime: %v", err)
	}

	// Directory should NOT exist for non-StorageAware runtime
	nonStoragePath := filepath.Join(storageDir, "non-storage-runtime")
	if _, err := os.Stat(nonStoragePath); !os.IsNotExist(err) {
		t.Errorf("Expected no directory for non-StorageAware runtime, but found: %s", nonStoragePath)
	}
}

// storageAwareRuntime is a fake runtime that implements StorageAware.
type storageAwareRuntime struct {
	typeID        string
	storageDir    string
	initializeErr error
}

func (s *storageAwareRuntime) Type() string        { return s.typeID }
func (s *storageAwareRuntime) DisplayName() string { return s.typeID }
func (s *storageAwareRuntime) Description() string { return "storage-aware runtime for testing" }
func (s *storageAwareRuntime) Local() bool         { return true }
func (s *storageAwareRuntime) WorkspaceSourcesPath() string {
	return "/workspace/sources"
}

func (s *storageAwareRuntime) Create(ctx context.Context, params CreateParams) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func (s *storageAwareRuntime) Start(ctx context.Context, id string) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func (s *storageAwareRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (s *storageAwareRuntime) Remove(ctx context.Context, id string) error {
	return nil
}

func (s *storageAwareRuntime) Info(ctx context.Context, id string) (RuntimeInfo, error) {
	return RuntimeInfo{}, nil
}

func (s *storageAwareRuntime) Initialize(storageDir string) error {
	s.storageDir = storageDir
	return s.initializeErr
}

func TestRegistry_CallsInitialize(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	rt := &storageAwareRuntime{typeID: "storage-aware"}
	err = reg.Register(rt)
	if err != nil {
		t.Fatalf("Failed to register runtime: %v", err)
	}

	// Verify Initialize was called with the correct directory
	expectedPath := filepath.Join(storageDir, "storage-aware")
	if rt.storageDir != expectedPath {
		t.Errorf("Expected Initialize to be called with %s, got %s", expectedPath, rt.storageDir)
	}
}

func TestRegistry_InitializeError(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	expectedErr := fmt.Errorf("initialization failed")
	rt := &storageAwareRuntime{
		typeID:        "storage-aware",
		initializeErr: expectedErr,
	}

	err = reg.Register(rt)
	if err == nil {
		t.Fatal("Expected error when Initialize fails, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap %v, got %v", expectedErr, err)
	}
}

func TestRegistry_StorageDirectoryStructure(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()

	// Create registry
	reg, err := NewRegistry(storageDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Register multiple runtimes, both StorageAware and non-StorageAware
	aware1 := &storageAwareRuntime{typeID: "aware1"}
	nonAware1 := &fakeRuntime{typeID: "nonAware1"}
	aware2 := &storageAwareRuntime{typeID: "aware2"}

	if err := reg.Register(aware1); err != nil {
		t.Fatalf("Failed to register aware1 runtime: %v", err)
	}
	if err := reg.Register(nonAware1); err != nil {
		t.Fatalf("Failed to register nonAware1 runtime: %v", err)
	}
	if err := reg.Register(aware2); err != nil {
		t.Fatalf("Failed to register aware2 runtime: %v", err)
	}

	// Verify directory structure - only StorageAware runtimes should have directories
	expectedDirs := []string{
		filepath.Join(storageDir, "aware1"),
		filepath.Join(storageDir, "aware2"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory to exist: %s", dir)
		}
	}

	// Verify non-StorageAware runtime does NOT have a directory
	nonAwareDir := filepath.Join(storageDir, "nonAware1")
	if _, err := os.Stat(nonAwareDir); !os.IsNotExist(err) {
		t.Errorf("Expected no directory for non-StorageAware runtime (nonAware1), but found: %s", nonAwareDir)
	}

	// Verify StorageAware runtimes received their directories
	expectedAware1Dir := filepath.Join(storageDir, "aware1")
	if aware1.storageDir != expectedAware1Dir {
		t.Errorf("Expected aware1 runtime to receive %s, got %s", expectedAware1Dir, aware1.storageDir)
	}

	expectedAware2Dir := filepath.Join(storageDir, "aware2")
	if aware2.storageDir != expectedAware2Dir {
		t.Errorf("Expected aware2 runtime to receive %s, got %s", expectedAware2Dir, aware2.storageDir)
	}

	// Verify non-StorageAware runtime still works
	retrieved, err := reg.Get("nonAware1")
	if err != nil {
		t.Fatalf("Failed to get nonAware1 runtime: %v", err)
	}
	if retrieved.Type() != "nonAware1" {
		t.Errorf("Expected nonAware1 runtime, got %s", retrieved.Type())
	}
}
