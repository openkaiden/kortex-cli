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
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestTerminal_EmptyID(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Terminal(context.Background(), "", "claude", nil)
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestTerminal_DefaultCommand(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_ = rt.Terminal(context.Background(), "kdn-test", "claude", nil)

	if len(fakeExec.RunInteractiveCalls) != 1 {
		t.Fatalf("Expected 1 RunInteractive call, got %d", len(fakeExec.RunInteractiveCalls))
	}

	args := fakeExec.RunInteractiveCalls[0]

	// Should use sandbox connect for interactive terminal
	if args[0] != "sandbox" || args[1] != "connect" {
		t.Errorf("Expected 'sandbox connect', got %v", args[:2])
	}

	// Instance ID should be a positional argument
	if args[2] != "kdn-test" {
		t.Errorf("Expected instance ID 'kdn-test', got %s", args[2])
	}

	if len(args) != 3 {
		t.Errorf("Expected 3 args, got %d: %v", len(args), args)
	}
}

func TestTerminal_CustomCommand(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_ = rt.Terminal(context.Background(), "kdn-test", "claude", []string{"claude-code", "--debug"})

	if len(fakeExec.RunInteractiveCalls) != 1 {
		t.Fatalf("Expected 1 RunInteractive call, got %d", len(fakeExec.RunInteractiveCalls))
	}

	lastArg := fakeExec.RunInteractiveCalls[0][len(fakeExec.RunInteractiveCalls[0])-1]
	if !strings.Contains(lastArg, "exec claude-code --debug") {
		t.Errorf("Expected custom command in args, got %q", lastArg)
	}
}

func TestTerminal_CustomCommandSourcesEnvAndChangesDir(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	_ = rt.Terminal(context.Background(), "kdn-test", "claude", []string{"bash"})

	if len(fakeExec.RunInteractiveCalls) != 1 {
		t.Fatalf("Expected 1 RunInteractive call, got %d", len(fakeExec.RunInteractiveCalls))
	}

	args := fakeExec.RunInteractiveCalls[0]

	// Should use sandbox exec with --name
	if args[0] != "sandbox" || args[1] != "exec" {
		t.Errorf("Expected 'sandbox exec', got %v", args[:2])
	}
	if args[2] != "--name" || args[3] != "kdn-test" {
		t.Errorf("Expected '--name kdn-test', got %v", args[2:4])
	}

	lastArg := args[len(args)-1]
	if !strings.Contains(lastArg, "source /sandbox/.kdn-env") {
		t.Errorf("Expected env sourcing in wrapped command, got %q", lastArg)
	}
	if !strings.Contains(lastArg, "cd /sandbox/workspace/sources") {
		t.Errorf("Expected cd to workspace sources in wrapped command, got %q", lastArg)
	}
}

func TestTerminal_PropagatesError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunInteractiveFunc = func(_ context.Context, args ...string) error {
		return fmt.Errorf("connection refused")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.Terminal(context.Background(), "kdn-test", "claude", nil)
	if err == nil {
		t.Error("Expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("Expected 'connection refused' error, got: %v", err)
	}
}
