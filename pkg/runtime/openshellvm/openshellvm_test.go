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

package openshellvm

import (
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshellvm/exec"
)

func TestNew(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt == nil {
		t.Fatal("New() returned nil")
	}

	if rt.Type() != "openshell-vm" {
		t.Errorf("Expected type 'openshell-vm', got %s", rt.Type())
	}
}

func TestOpenshellRuntime_Available(t *testing.T) {
	t.Parallel()

	rt := New()

	avail, ok := rt.(interface{ Available() bool })
	if !ok {
		t.Fatal("Expected runtime to implement Available interface")
	}

	if !avail.Available() {
		t.Error("Expected Available() to return true (binaries are auto-downloaded)")
	}
}

func TestOpenshellRuntime_WorkspaceSourcesPath(t *testing.T) {
	t.Parallel()

	rt := New()
	path := rt.WorkspaceSourcesPath()

	if path != "/sandbox/workspace/sources" {
		t.Errorf("WorkspaceSourcesPath() = %q, want %q", path, "/sandbox/workspace/sources")
	}
}

func TestOpenshellRuntime_Type(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/openshell-vm", t.TempDir())
	if rt.Type() != "openshell-vm" {
		t.Errorf("Type() = %q, want %q", rt.Type(), "openshell-vm")
	}
}

func TestOpenshellRuntime_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	var _ runtime.Runtime = (*openshellVMRuntime)(nil)
	var _ runtime.StorageAware = (*openshellVMRuntime)(nil)
	var _ runtime.Terminal = (*openshellVMRuntime)(nil)
}

func TestOpenshellRuntime_Initialize_EmptyStorageDir(t *testing.T) {
	t.Parallel()

	rt := &openshellVMRuntime{}
	err := rt.Initialize("")
	if err == nil {
		t.Error("Expected error for empty storage directory")
	}
}

func TestSandboxName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"my-project", "kdn-my-project"},
		{"test", "kdn-test"},
		{"", "kdn-"},
	}

	for _, tt := range tests {
		got := sandboxName(tt.input)
		if got != tt.expected {
			t.Errorf("sandboxName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
