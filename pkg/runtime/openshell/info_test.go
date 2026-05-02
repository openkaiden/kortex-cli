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
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestInfo_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_, err := rt.Info(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestInfo_ReturnsOverrideState(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if err := rt.states.Set("kdn-test", api.WorkspaceStateStopped); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	info, err := rt.Info(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Info() failed: %v", err)
	}

	if info.State != api.WorkspaceStateStopped {
		t.Errorf("Expected state %q, got %q", api.WorkspaceStateStopped, info.State)
	}

	// Executor should not be called when override exists
	if len(fakeExec.OutputCalls) != 0 {
		t.Error("Expected no executor calls when override exists")
	}
}

func TestInfo_QueriesSandboxWhenNoOverride(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\nother-sandbox\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	info, err := rt.Info(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Info() failed: %v", err)
	}

	if info.State != api.WorkspaceStateRunning {
		t.Errorf("Expected state %q, got %q", api.WorkspaceStateRunning, info.State)
	}
}

func TestInfo_ReturnsErrorForMissingSandbox(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("other-sandbox\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	info, err := rt.Info(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Info() failed: %v", err)
	}

	if info.State != api.WorkspaceStateError {
		t.Errorf("Expected state %q, got %q", api.WorkspaceStateError, info.State)
	}
}

func TestInfo_ReturnsIDAndSandboxName(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-my-project\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	info, err := rt.Info(context.Background(), "kdn-my-project")
	if err != nil {
		t.Fatalf("Info() failed: %v", err)
	}

	if info.ID != "kdn-my-project" {
		t.Errorf("Expected ID 'kdn-my-project', got %q", info.ID)
	}
	if info.Info["sandbox_name"] != "kdn-my-project" {
		t.Errorf("Expected sandbox_name 'kdn-my-project', got %q", info.Info["sandbox_name"])
	}
}

func TestInfo_ExecutorError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	info, err := rt.Info(context.Background(), "kdn-test")
	if err != nil {
		t.Fatalf("Info() should not return error (returns error state): %v", err)
	}

	if info.State != api.WorkspaceStateError {
		t.Errorf("Expected error state when executor fails, got %q", info.State)
	}
}

func TestQuerySandboxState_EmptyOutput(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	state := rt.querySandboxState(context.Background(), "kdn-test")
	if state != api.WorkspaceStateError {
		t.Errorf("Expected error state for empty output, got %q", state)
	}
}

func TestQuerySandboxState_MultipleSandboxes(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-first\nkdn-second\nkdn-third\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if state := rt.querySandboxState(context.Background(), "kdn-second"); state != api.WorkspaceStateRunning {
		t.Errorf("Expected running for kdn-second, got %q", state)
	}
	if state := rt.querySandboxState(context.Background(), "kdn-missing"); state != api.WorkspaceStateError {
		t.Errorf("Expected error for kdn-missing, got %q", state)
	}
}
