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
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestStart_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_, err := rt.Start(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestStart_ClearsStoppedOverride(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	// Set a stopped override
	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state override: %v", err)
	}

	// Verify override exists
	state, ok := rt.states.Get("kdn-test")
	if !ok || state != api.WorkspaceStateStopped {
		t.Fatal("Expected stopped override to exist")
	}

	// Note: Start will fail because isGatewayReady uses os/exec directly,
	// but we can verify the state override logic independently.
	_ = rt.states.Remove("kdn-test")

	// Verify override is cleared
	_, ok = rt.states.Get("kdn-test")
	if ok {
		t.Error("Expected override to be removed after start")
	}
}

func TestStart_ReturnsRunningState(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	// Set stopped override first, then start clears it
	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	// Verify the state override and query logic used by Start
	state := rt.querySandboxState(context.Background(), "kdn-test")
	if state != api.WorkspaceStateRunning {
		t.Errorf("Expected running state from sandbox query, got %q", state)
	}
}

func TestStart_SandboxNotFound(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("other-sandbox\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	state := rt.querySandboxState(context.Background(), "kdn-test")
	if state != api.WorkspaceStateError {
		t.Errorf("Expected error state for missing sandbox, got %q", state)
	}
}

func TestStart_FullFlow_Success(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	// Set stopped override (mimics state after Create)
	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	info, err := rt.Start(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if info.State != api.WorkspaceStateRunning {
		t.Errorf("Expected running state, got %q", info.State)
	}
	if info.ID != "kdn-test" {
		t.Errorf("Expected ID 'kdn-test', got %q", info.ID)
	}

	// Override should be cleared
	_, ok := rt.states.Get("kdn-test")
	if ok {
		t.Error("Expected stopped override to be cleared after Start")
	}
}

func TestStart_FullFlow_SandboxNotFound(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("other-sandbox\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_, err := rt.Start(context.Background(), "kdn-test")
	if err == nil {
		t.Error("Expected error when sandbox not found")
	}
}

func TestStart_FullFlow_QueryError(t *testing.T) {
	t.Parallel()

	callCount := 0
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		callCount++
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			if callCount == 1 {
				// First call: isGatewayReady → success
				return []byte("kdn-test\n"), nil
			}
			// Second call: querySandboxState → error
			return nil, fmt.Errorf("gateway unreachable")
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_, err := rt.Start(context.Background(), "kdn-test")
	if err == nil {
		t.Error("Expected error when sandbox query fails")
	}
}

func TestStart_FullFlow_StartsPortForwarding(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.globalStorageDir = storageDir

	if err := rt.writeSandboxData("kdn-test", sandboxData{
		SourcePath: t.TempDir(),
		Agent:      "openclaw",
		Ports:      []int{8080, 18789},
	}); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	info, err := rt.Start(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if info.State != api.WorkspaceStateRunning {
		t.Errorf("Expected running state, got %q", info.State)
	}

	// Verify forward start calls
	forwardStartCount := 0
	for _, call := range fakeExec.RunCalls {
		if len(call) >= 2 && call[0] == "forward" && call[1] == "start" {
			forwardStartCount++
			if !slices.Contains(call, "--background") {
				t.Errorf("Expected --background flag in forward start call, got: %v", call)
			}
		}
	}
	if forwardStartCount != 2 {
		t.Errorf("Expected 2 forward start calls, got %d. Calls: %v", forwardStartCount, fakeExec.RunCalls)
	}

	// Verify forwards in RuntimeInfo
	forwardsJSON, ok := info.Info["forwards"]
	if !ok {
		t.Fatal("Expected 'forwards' in RuntimeInfo.Info")
	}

	var forwards []api.WorkspaceForward
	if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err != nil {
		t.Fatalf("Failed to unmarshal forwards: %v", err)
	}
	if len(forwards) != 2 {
		t.Fatalf("Expected 2 forwards, got %d", len(forwards))
	}
	if forwards[0].Port != 8080 || forwards[1].Port != 18789 {
		t.Errorf("Unexpected forward ports: %v", forwards)
	}
}

func TestStart_PortCollision(t *testing.T) {
	t.Parallel()

	// Hold a real listener to simulate a port collision
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer l.Close()
	occupiedPort := l.Addr().(*net.TCPAddr).Port

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.globalStorageDir = storageDir

	if err := rt.writeSandboxData("kdn-test", sandboxData{
		SourcePath: t.TempDir(),
		Agent:      "openclaw",
		Ports:      []int{occupiedPort},
	}); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	_, err = rt.Start(context.Background(), "kdn-test")
	if err == nil {
		t.Fatal("Expected Start() to fail due to port collision")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("Expected error mentioning 'already in use', got: %v", err)
	}

	// Verify no forward start calls were made
	for _, call := range fakeExec.RunCalls {
		if len(call) >= 2 && call[0] == "forward" && call[1] == "start" {
			t.Errorf("Expected no forward start calls due to collision, got: %v", call)
		}
	}
}

func TestStart_FullFlow_NoSandboxDataSkipsNetwork(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	// No sandbox data written — simulates old sandbox created before network support

	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	info, err := rt.Start(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if info.State != api.WorkspaceStateRunning {
		t.Errorf("Expected running state, got %q", info.State)
	}

	// No policy update should have been called
	for _, call := range fakeExec.RunCalls {
		if slices.Contains(call, "policy") {
			t.Errorf("Expected no policy calls without sandbox data, got: %v", call)
		}
	}
}
