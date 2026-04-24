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
	"context"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshellvm/exec"
)

func TestStop_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-vm", t.TempDir())

	err := rt.Stop(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestStop_SetsStoppedOverride(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-vm", t.TempDir())

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
	rt := newWithDeps(fakeExec, "/fake/openshell-vm", t.TempDir())

	_ = rt.Stop(context.Background(), "kdn-test")

	if len(fakeExec.RunCalls) != 0 {
		t.Error("Stop should not call the executor (no actual sandbox stop)")
	}
	if len(fakeExec.OutputCalls) != 0 {
		t.Error("Stop should not call the executor")
	}
}
