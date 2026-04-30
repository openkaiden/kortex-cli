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

package openshell

import (
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
)

func TestStateOverrides_SetAndGet(t *testing.T) {
	t.Parallel()

	s := newStateOverrides(t.TempDir())

	// Initially no override
	_, ok := s.Get("sandbox-1")
	if ok {
		t.Error("Expected no override for new sandbox")
	}

	// Set override
	if err := s.Set("sandbox-1", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Get override
	state, ok := s.Get("sandbox-1")
	if !ok {
		t.Fatal("Expected override to exist")
	}
	if state != api.WorkspaceStateStopped {
		t.Errorf("Get() = %q, want %q", state, api.WorkspaceStateStopped)
	}
}

func TestStateOverrides_Remove(t *testing.T) {
	t.Parallel()

	s := newStateOverrides(t.TempDir())

	if err := s.Set("sandbox-1", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	if err := s.Remove("sandbox-1"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	_, ok := s.Get("sandbox-1")
	if ok {
		t.Error("Expected override to be removed")
	}
}

func TestStateOverrides_RemoveNonexistent(t *testing.T) {
	t.Parallel()

	s := newStateOverrides(t.TempDir())

	if err := s.Remove("nonexistent"); err != nil {
		t.Fatalf("Remove() failed for nonexistent key: %v", err)
	}
}

func TestStateOverrides_MultipleSandboxes(t *testing.T) {
	t.Parallel()

	s := newStateOverrides(t.TempDir())

	if err := s.Set("sandbox-1", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	if err := s.Set("sandbox-2", api.WorkspaceStateRunning); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	state1, ok := s.Get("sandbox-1")
	if !ok || state1 != api.WorkspaceStateStopped {
		t.Errorf("sandbox-1: got %q, want %q", state1, api.WorkspaceStateStopped)
	}

	state2, ok := s.Get("sandbox-2")
	if !ok || state2 != api.WorkspaceStateRunning {
		t.Errorf("sandbox-2: got %q, want %q", state2, api.WorkspaceStateRunning)
	}
}
