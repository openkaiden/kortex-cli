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

package fake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime"
)

func TestFakeRuntime_Type(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt.Type() != "fake" {
		t.Errorf("Expected type 'fake', got '%s'", rt.Type())
	}
}

func TestFakeRuntime_DisplayName(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt.DisplayName() != "fake" {
		t.Errorf("DisplayName() = %q, want %q", rt.DisplayName(), "fake")
	}
}

func TestFakeRuntime_WorkspaceSourcesPath(t *testing.T) {
	t.Parallel()

	rt := New()
	path := rt.WorkspaceSourcesPath()

	if path != "/project/sources" {
		t.Errorf("WorkspaceSourcesPath() = %q, want %q", path, "/project/sources")
	}

	// Verify it's consistent across calls
	path2 := rt.WorkspaceSourcesPath()
	if path != path2 {
		t.Errorf("WorkspaceSourcesPath() inconsistent: %q != %q", path, path2)
	}
}

func TestFakeRuntime_CreateStartStopRemove(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	params := runtime.CreateParams{
		Name:            "test-instance",
		SourcePath:      "/path/to/source",
		WorkspaceConfig: nil,
	}

	// Create instance
	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if info.ID == "" {
		t.Error("Expected non-empty instance ID")
	}
	if info.State != "stopped" {
		t.Errorf("Expected state 'stopped', got '%s'", info.State)
	}
	if !strings.HasPrefix(info.ID, "fake-") {
		t.Errorf("Expected ID to start with 'fake-', got '%s'", info.ID)
	}

	instanceID := info.ID

	// Start instance
	info, err = rt.Start(ctx, instanceID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info.State)
	}

	// Stop instance
	err = rt.Stop(ctx, instanceID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify stopped state
	info, err = rt.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.State != "stopped" {
		t.Errorf("Expected state 'stopped', got '%s'", info.State)
	}

	// Remove instance
	err = rt.Remove(ctx, instanceID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify instance is gone
	_, err = rt.Info(ctx, instanceID)
	if !errors.Is(err, runtime.ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound after remove, got %v", err)
	}
}

func TestFakeRuntime_InfoRetrievesCorrectState(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	params := runtime.CreateParams{
		Name:            "info-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instanceID := info.ID

	// Info should return created state
	info, err = rt.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.State != "stopped" {
		t.Errorf("Expected state 'stopped', got '%s'", info.State)
	}

	// Start and verify running state
	_, err = rt.Start(ctx, instanceID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	info, err = rt.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info.State)
	}

	// Verify info contains expected metadata
	if info.Info["source"] != "/source" {
		t.Errorf("Expected source '/source', got '%s'", info.Info["source"])
	}
	if info.Info["created_at"] == "" {
		t.Error("Expected created_at timestamp")
	}
	if info.Info["started_at"] == "" {
		t.Error("Expected started_at timestamp")
	}
}

func TestFakeRuntime_DuplicateCreate(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	params := runtime.CreateParams{
		Name:            "duplicate-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	// Create first instance
	_, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Try to create duplicate
	_, err = rt.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected error when creating duplicate instance, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got '%s'", err.Error())
	}
}

func TestFakeRuntime_UnknownInstanceID(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	// Try to start non-existent instance
	_, err := rt.Start(ctx, "unknown-id")
	if !errors.Is(err, runtime.ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}

	// Try to stop non-existent instance
	err = rt.Stop(ctx, "unknown-id")
	if !errors.Is(err, runtime.ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}

	// Try to remove non-existent instance
	// Note: Remove is idempotent for fake runtime (returns nil for non-existent instances).
	err = rt.Remove(ctx, "unknown-id")
	if err != nil {
		t.Errorf("Expected nil (idempotent remove), got %v", err)
	}

	// Try to get info for non-existent instance
	_, err = rt.Info(ctx, "unknown-id")
	if !errors.Is(err, runtime.ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestFakeRuntime_InvalidParams(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	tests := []struct {
		name   string
		params runtime.CreateParams
	}{
		{
			name: "missing name",
			params: runtime.CreateParams{
				SourcePath:      "/source",
				WorkspaceConfig: nil,
			},
		},
		{
			name: "missing source path",
			params: runtime.CreateParams{
				Name:            "test",
				WorkspaceConfig: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rt.Create(ctx, tt.params)
			if !errors.Is(err, runtime.ErrInvalidParams) {
				t.Errorf("Expected ErrInvalidParams, got %v", err)
			}
		})
	}
}

func TestFakeRuntime_StateTransitionErrors(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	params := runtime.CreateParams{
		Name:            "state-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instanceID := info.ID

	// Can't stop created instance
	err = rt.Stop(ctx, instanceID)
	if err == nil {
		t.Error("Expected error when stopping created instance")
	}

	// Can't remove running instance
	_, err = rt.Start(ctx, instanceID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = rt.Remove(ctx, instanceID)
	if err == nil {
		t.Error("Expected error when removing running instance")
	}

	// Can't start already running instance
	_, err = rt.Start(ctx, instanceID)
	if err == nil {
		t.Error("Expected error when starting already running instance")
	}
}

func TestFakeRuntime_SequentialIDs(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	// Create multiple instances and verify sequential IDs
	var ids []string
	for i := 1; i <= 3; i++ {
		params := runtime.CreateParams{
			Name:            fmt.Sprintf("instance-%d", i),
			SourcePath:      "/source",
			WorkspaceConfig: nil,
		}

		info, err := rt.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}

		ids = append(ids, info.ID)
	}

	// Verify IDs are sequential
	expectedIDs := []string{"fake-001", "fake-002", "fake-003"}
	for i, id := range ids {
		if id != expectedIDs[i] {
			t.Errorf("Expected ID %s, got %s", expectedIDs[i], id)
		}
	}
}

func TestFakeRuntime_ParallelOperations(t *testing.T) {
	t.Parallel()

	rt := New()
	ctx := context.Background()

	const numInstances = 10
	var wg sync.WaitGroup
	wg.Add(numInstances)

	// Create multiple instances in parallel
	for i := 0; i < numInstances; i++ {
		i := i
		go func() {
			defer wg.Done()

			params := runtime.CreateParams{
				Name:            fmt.Sprintf("parallel-%d", i),
				SourcePath:      "/source",
				WorkspaceConfig: nil,
			}

			info, err := rt.Create(ctx, params)
			if err != nil {
				t.Errorf("Create failed for instance %d: %v", i, err)
				return
			}

			// Start the instance
			_, err = rt.Start(ctx, info.ID)
			if err != nil {
				t.Errorf("Start failed for instance %d: %v", i, err)
			}
		}()
	}

	wg.Wait()
}

func TestFakeRuntime_Persistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageDir := t.TempDir()

	// Create and initialize first runtime
	rt1 := New()
	storageAware1, ok := rt1.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware1.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create an instance
	params := runtime.CreateParams{
		Name:            "persistent-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info1, err := rt1.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instanceID := info1.ID

	// Start the instance
	_, err = rt1.Start(ctx, instanceID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Create and initialize second runtime with same storage
	rt2 := New()
	storageAware2, ok := rt2.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware2.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Second initialize failed: %v", err)
	}

	// Verify instance was loaded from disk
	info2, err := rt2.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed on second runtime: %v", err)
	}

	if info2.ID != instanceID {
		t.Errorf("Expected ID %s, got %s", instanceID, info2.ID)
	}
	if info2.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info2.State)
	}
	if info2.Info["source"] != "/source" {
		t.Errorf("Expected source '/source', got '%s'", info2.Info["source"])
	}
}

func TestFakeRuntime_PersistenceStateChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageDir := t.TempDir()

	// Create and initialize runtime
	rt := New()
	storageAware, ok := rt.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create an instance
	params := runtime.CreateParams{
		Name:            "state-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instanceID := info.ID

	// Start the instance
	_, err = rt.Start(ctx, instanceID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Create new runtime and verify running state persisted
	rt2 := New()
	storageAware2, ok := rt2.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware2.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Second initialize failed: %v", err)
	}

	info, err = rt2.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info.State)
	}

	// Stop the instance
	err = rt2.Stop(ctx, instanceID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Create third runtime and verify stopped state persisted
	rt3 := New()
	storageAware3, ok := rt3.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware3.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Third initialize failed: %v", err)
	}

	info, err = rt3.Info(ctx, instanceID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.State != "stopped" {
		t.Errorf("Expected state 'stopped', got '%s'", info.State)
	}
}

func TestFakeRuntime_PersistenceRemoval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageDir := t.TempDir()

	// Create and initialize runtime
	rt := New()
	storageAware, ok := rt.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create an instance
	params := runtime.CreateParams{
		Name:            "removal-test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instanceID := info.ID

	// Remove the instance
	err = rt.Remove(ctx, instanceID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Create new runtime and verify instance is gone
	rt2 := New()
	storageAware2, ok := rt2.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware2.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Second initialize failed: %v", err)
	}

	_, err = rt2.Info(ctx, instanceID)
	if !errors.Is(err, runtime.ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestFakeRuntime_PersistenceSequentialIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageDir := t.TempDir()

	// Create and initialize first runtime
	rt1 := New()
	storageAware1, ok := rt1.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware1.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create first instance
	params := runtime.CreateParams{
		Name:            "instance-1",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	info1, err := rt1.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if info1.ID != "fake-001" {
		t.Errorf("Expected ID 'fake-001', got '%s'", info1.ID)
	}

	// Create and initialize second runtime with same storage
	rt2 := New()
	storageAware2, ok := rt2.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware2.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Second initialize failed: %v", err)
	}

	// Create second instance - should get next sequential ID
	params.Name = "instance-2"
	info2, err := rt2.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if info2.ID != "fake-002" {
		t.Errorf("Expected ID 'fake-002', got '%s'", info2.ID)
	}
}

func TestFakeRuntime_InitializeEmptyDirectory(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()

	rt := New()
	storageAware, ok := rt.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize with empty directory failed: %v", err)
	}

	// Verify we can create instances
	ctx := context.Background()
	params := runtime.CreateParams{
		Name:            "test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	_, err = rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestFakeRuntime_InitializeEmptyStorageDir(t *testing.T) {
	t.Parallel()

	rt := New()
	storageAware, ok := rt.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err := storageAware.Initialize("")
	if err == nil {
		t.Fatal("Expected error when initializing with empty storage directory")
	}
}

func TestFakeRuntime_WithoutInitialization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create runtime without initializing storage
	rt := New()

	params := runtime.CreateParams{
		Name:            "test",
		SourcePath:      "/source",
		WorkspaceConfig: nil,
	}

	// Should still work in memory-only mode
	info, err := rt.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it works
	_, err = rt.Info(ctx, info.ID)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
}

func TestFakeRuntime_LoadWithNilInfoMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageDir := t.TempDir()

	// Manually create a storage file with a nil Info map to simulate
	// a corrupted or manually edited storage file
	storageFile := filepath.Join(storageDir, "instances.json")
	corruptedData := `{
  "next_id": 2,
  "instances": {
    "fake-001": {
      "id": "fake-001",
      "name": "test-instance",
      "state": "stopped",
      "info": null,
      "source": "/source"
    }
  }
}`

	err := os.WriteFile(storageFile, []byte(corruptedData), 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupted storage file: %v", err)
	}

	// Initialize runtime with the corrupted storage
	rt := New()
	storageAware, ok := rt.(runtime.StorageAware)
	if !ok {
		t.Fatal("Expected runtime to implement StorageAware")
	}

	err = storageAware.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify the instance was loaded
	info, err := rt.Info(ctx, "fake-001")
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}

	if info.ID != "fake-001" {
		t.Errorf("Expected ID 'fake-001', got '%s'", info.ID)
	}

	// The critical test: Start should not panic even though Info was nil in storage
	// The hardening code should have initialized it to an empty map
	_, err = rt.Start(ctx, "fake-001")
	if err != nil {
		t.Fatalf("Start failed (should not panic with nil Info): %v", err)
	}

	// Verify Info was updated by Start
	info, err = rt.Info(ctx, "fake-001")
	if err != nil {
		t.Fatalf("Info failed after start: %v", err)
	}

	if info.Info["started_at"] == "" {
		t.Error("Expected started_at timestamp to be set")
	}

	// Test Stop as well
	err = rt.Stop(ctx, "fake-001")
	if err != nil {
		t.Fatalf("Stop failed (should not panic with nil Info): %v", err)
	}

	// Verify Info was updated by Stop
	info, err = rt.Info(ctx, "fake-001")
	if err != nil {
		t.Fatalf("Info failed after stop: %v", err)
	}

	if info.Info["stopped_at"] == "" {
		t.Error("Expected stopped_at timestamp to be set")
	}
}

func TestNewWithExperimental_ImplementsInterface(t *testing.T) {
	t.Parallel()

	rt := NewWithExperimental()

	if _, ok := rt.(runtime.Experimental); !ok {
		t.Fatal("NewWithExperimental() returned a runtime that does not implement runtime.Experimental")
	}
}

func TestNewWithExperimental_IsExperimental(t *testing.T) {
	t.Parallel()

	rt := NewWithExperimental()

	exp, ok := rt.(runtime.Experimental)
	if !ok {
		t.Fatal("NewWithExperimental() returned a runtime that does not implement runtime.Experimental")
	}
	// IsExperimental carries no return value; calling it must not panic.
	exp.IsExperimental()
}

func TestNewWithExperimental_IsFullRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rt := NewWithExperimental()

	info, err := rt.Create(ctx, runtime.CreateParams{
		Name:       "test-ws",
		SourcePath: "/tmp/src",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if _, err := rt.Start(ctx, info.ID); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if err := rt.Stop(ctx, info.ID); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if err := rt.Remove(ctx, info.ID); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
}

func TestNewWithDashboard_GetURL(t *testing.T) {
	t.Parallel()

	const dashboardURL = "http://localhost:8888"
	rt := NewWithDashboard(dashboardURL)

	d, ok := rt.(interface {
		GetURL(ctx context.Context, instanceID string) (string, error)
	})
	if !ok {
		t.Fatal("NewWithDashboard() returned a runtime that does not implement Dashboard")
	}

	url, err := d.GetURL(context.Background(), "any-instance-id")
	if err != nil {
		t.Fatalf("GetURL() failed: %v", err)
	}
	if url != dashboardURL {
		t.Errorf("GetURL() = %q, want %q", url, dashboardURL)
	}
}

func TestNewWithDashboard_IsFullRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rt := NewWithDashboard("http://localhost:9999")

	info, err := rt.Create(ctx, runtime.CreateParams{
		Name:       "test-ws",
		SourcePath: "/tmp/src",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if _, err := rt.Start(ctx, info.ID); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if err := rt.Stop(ctx, info.ID); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if err := rt.Remove(ctx, info.ID); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
}

func TestNewWithDashboard_GetURL_AnyInstance(t *testing.T) {
	t.Parallel()

	const dashboardURL = "http://localhost:7777"
	rt := NewWithDashboard(dashboardURL)

	d := rt.(interface {
		GetURL(ctx context.Context, instanceID string) (string, error)
	})

	for _, id := range []string{"", "fake-001", "some-other-id"} {
		url, err := d.GetURL(context.Background(), id)
		if err != nil {
			t.Errorf("GetURL(%q) failed: %v", id, err)
		}
		if url != dashboardURL {
			t.Errorf("GetURL(%q) = %q, want %q", id, url, dashboardURL)
		}
	}
}
