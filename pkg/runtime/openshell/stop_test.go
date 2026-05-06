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
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestStop_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Stop(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestStop_SetsStoppedOverride(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Stop(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	state, ok := rt.states.Get("kdn-test")
	if !ok {
		t.Fatal("Expected state override to exist after stop")
	}
	if state != api.WorkspaceStateStopped {
		t.Errorf("Expected state %q, got %q", api.WorkspaceStateStopped, state)
	}
}

func TestStop_DoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_ = rt.Stop(context.Background(), "kdn-test")

	if len(fakeExec.RunCalls) != 0 {
		t.Error("Stop should not call the executor (no actual sandbox stop)")
	}
	if len(fakeExec.OutputCalls) != 0 {
		t.Error("Stop should not call the executor")
	}
}

func TestStop_StateSetError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", "/nonexistent/path")

	err := rt.Stop(context.Background(), "kdn-test")
	if err == nil {
		t.Error("Expected error when state set fails")
	}
}

func TestStop_StopsPortForwards(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	if err := rt.writeSandboxData("kdn-test", sandboxData{
		SourcePath: "/src",
		Agent:      "openclaw",
		Ports:      []int{8080, 18789},
	}); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	err := rt.Stop(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	forwardStopCount := 0
	for _, call := range fakeExec.RunCalls {
		if len(call) >= 2 && call[0] == "forward" && call[1] == "stop" {
			forwardStopCount++
		}
	}
	if forwardStopCount != 2 {
		t.Errorf("Expected 2 forward stop calls, got %d. Calls: %v", forwardStopCount, fakeExec.RunCalls)
	}
}

func TestStop_NilExecutorWithPorts(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	rt := &openshellRuntime{
		storageDir: storageDir,
		states:     newStateOverrides(storageDir),
		config:     loadConfig(storageDir),
	}

	if err := rt.writeSandboxData("kdn-test", sandboxData{
		SourcePath: "/src",
		Agent:      "claude",
		Ports:      []int{8080},
	}); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	err := rt.Stop(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Stop() should not panic or error with nil executor, got: %v", err)
	}

	state, ok := rt.states.Get("kdn-test")
	if !ok || state != api.WorkspaceStateStopped {
		t.Errorf("Expected stopped state, got %q", state)
	}
}

func TestStop_OverwritesPreviousState(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	// Set to running, then stop
	if err := rt.states.Set("kdn-test", api.WorkspaceStateRunning); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	if err := rt.Stop(context.Background(), "kdn-test"); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	state, ok := rt.states.Get("kdn-test")
	if !ok || state != api.WorkspaceStateStopped {
		t.Errorf("Expected stopped state after Stop, got %q", state)
	}
}
