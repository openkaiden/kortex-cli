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
	"context"
	"fmt"
	"os"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestRemove_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Remove(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestRemove_RefusesRunning(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		// Sandbox exists and is running (no stopped override)
		return []byte("kdn-test\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Remove(context.Background(), "kdn-test")
	if err == nil {
		t.Error("Expected error when removing running sandbox")
	}
}

func TestRemove_DeleteError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "delete" {
			return fmt.Errorf("delete failed")
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	err := rt.Remove(context.Background(), "kdn-test")
	if err == nil {
		t.Error("Expected error when delete fails")
	}
}

func TestRemove_DeletesStopped(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	// Set stopped override
	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	err := rt.Remove(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Verify executor was called with delete
	found := false
	for _, call := range fakeExec.RunCalls {
		if len(call) >= 3 && call[0] == "sandbox" && call[1] == "delete" && call[2] == "kdn-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected executor to be called with 'sandbox delete kdn-test'")
	}

	// Verify override is cleaned up
	_, ok := rt.states.Get("kdn-test")
	if ok {
		t.Error("Expected state override to be removed after delete")
	}
}

func TestRemove_CleansSandboxData(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	// Write sandbox data
	data := sandboxData{SourcePath: "/src", ProjectID: "proj1", Agent: "claude"}
	if err := rt.writeSandboxData("kdn-test", data); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	// Verify it exists
	dir := rt.sandboxDataDir("kdn-test")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Expected sandbox data directory to exist: %v", err)
	}

	// Set stopped override (required for remove)
	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	if err := rt.Remove(context.Background(), "kdn-test"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Verify sandbox data is cleaned up
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Expected sandbox data directory to be removed, got err: %v", err)
	}
}
