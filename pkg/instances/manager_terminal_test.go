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

package instances

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/openkaiden/kdn/pkg/agent"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

// fakeTerminalRuntime is a fake runtime that implements runtime.Terminal
type fakeTerminalRuntime struct {
	runtime.Runtime
	terminalCalls []terminalCall
	terminalErr   error
}

type terminalCall struct {
	instanceID string
	agent      string
	command    []string
}

// Compile-time check
var _ runtime.Terminal = (*fakeTerminalRuntime)(nil)

func (f *fakeTerminalRuntime) Terminal(ctx context.Context, instanceID string, agent string, command []string) error {
	f.terminalCalls = append(f.terminalCalls, terminalCall{
		instanceID: instanceID,
		agent:      agent,
		command:    command,
	})
	return f.terminalErr
}

// newFakeTerminalRuntime creates a fake runtime that supports Terminal interface
func newFakeTerminalRuntime(terminalErr error) *fakeTerminalRuntime {
	return &fakeTerminalRuntime{
		Runtime:       fake.New(),
		terminalCalls: make([]terminalCall, 0),
		terminalErr:   terminalErr,
	}
}

// newTestRegistryWithTerminal creates a registry with a terminal-supporting runtime
func newTestRegistryWithTerminal(tmpDir string, terminalErr error) runtime.Registry {
	reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
	fakeRT := newFakeTerminalRuntime(terminalErr)
	_ = reg.Register(fakeRT)
	return reg
}

func TestManager_Terminal(t *testing.T) {
	t.Parallel()

	t.Run("connects to running instance successfully", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		// Create registry with terminal-supporting runtime
		fakeRT := newFakeTerminalRuntime(nil)
		reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
		_ = reg.Register(fakeRT)

		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), reg, agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Agent: "test-agent"})

		// Start the instance
		_ = manager.Start(ctx, added.GetID())

		// Connect to terminal
		command := []string{"bash"}
		err := manager.Terminal(ctx, added.GetID(), command)
		if err != nil {
			t.Fatalf("Terminal() unexpected error = %v", err)
		}

		// Verify Terminal was called on the runtime
		if len(fakeRT.terminalCalls) != 1 {
			t.Fatalf("Expected 1 Terminal call, got %d", len(fakeRT.terminalCalls))
		}

		call := fakeRT.terminalCalls[0]
		if call.instanceID != added.GetRuntimeData().InstanceID {
			t.Errorf("Terminal called with instanceID = %v, want %v", call.instanceID, added.GetRuntimeData().InstanceID)
		}

		if call.agent != "test-agent" {
			t.Errorf("Terminal called with agent = %v, want test-agent", call.agent)
		}

		if len(call.command) != 1 || call.command[0] != "bash" {
			t.Errorf("Terminal called with command = %v, want [bash]", call.command)
		}
	})

	t.Run("connects with multiple command arguments", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		fakeRT := newFakeTerminalRuntime(nil)
		reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
		_ = reg.Register(fakeRT)

		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), reg, agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Agent: "test-agent"})
		_ = manager.Start(ctx, added.GetID())

		// Connect with command and arguments
		command := []string{"claude-code", "--debug"}
		err := manager.Terminal(ctx, added.GetID(), command)
		if err != nil {
			t.Fatalf("Terminal() unexpected error = %v", err)
		}

		call := fakeRT.terminalCalls[0]
		if len(call.command) != 2 || call.command[0] != "claude-code" || call.command[1] != "--debug" {
			t.Errorf("Terminal called with command = %v, want [claude-code --debug]", call.command)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistryWithTerminal(tmpDir, nil), agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		err := manager.Terminal(context.Background(), "nonexistent-id", []string{"bash"})
		if !errors.Is(err, ErrInstanceNotFound) {
			t.Errorf("Terminal() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("auto-starts stopped instance", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		fakeRT := newFakeTerminalRuntime(nil)
		reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
		_ = reg.Register(fakeRT)

		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), reg, agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Agent: "test-agent"})

		// Instance is in "created" state (not running) — Terminal should auto-start it
		command := []string{"bash"}
		err := manager.Terminal(ctx, added.GetID(), command)
		if err != nil {
			t.Fatalf("Terminal() unexpected error = %v", err)
		}

		// Verify instance was started
		got, _ := manager.Get(added.GetID())
		if got.GetRuntimeData().State != "running" {
			t.Errorf("Expected instance state to be running after auto-start, got %s", got.GetRuntimeData().State)
		}

		// Verify Terminal was called on the runtime
		if len(fakeRT.terminalCalls) != 1 {
			t.Fatalf("Expected 1 Terminal call, got %d", len(fakeRT.terminalCalls))
		}

		call := fakeRT.terminalCalls[0]
		if call.agent != "test-agent" {
			t.Errorf("Terminal called with agent = %v, want test-agent", call.agent)
		}
	})

	t.Run("returns error when runtime doesn't support terminal", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		// Use regular fake runtime (doesn't implement Terminal)
		reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
		regularFakeRT := fake.New()
		_ = reg.Register(regularFakeRT)

		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), reg, agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Agent: "test-agent"})
		_ = manager.Start(ctx, added.GetID())

		// Try to connect - should fail because runtime doesn't support Terminal
		err := manager.Terminal(ctx, added.GetID(), []string{"bash"})
		if err == nil {
			t.Fatal("Expected error when runtime doesn't support terminal")
		}

		// Error message should mention terminal not supported
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	})

	t.Run("propagates runtime terminal error", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		expectedErr := errors.New("terminal exec failed")
		fakeRT := newFakeTerminalRuntime(expectedErr)
		reg, _ := runtime.NewRegistry(filepath.Join(tmpDir, "runtimes"))
		_ = reg.Register(fakeRT)

		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), reg, agent.NewRegistry(), secretservice.NewRegistry(), secret.NewStore(tmpDir), newFakeProjectDetector(), time.Now)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Agent: "test-agent"})
		_ = manager.Start(ctx, added.GetID())

		// Terminal should propagate the runtime error
		err := manager.Terminal(ctx, added.GetID(), []string{"bash"})
		if err == nil {
			t.Fatal("Expected error to be propagated")
		}

		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got: %v", expectedErr, err)
		}
	})
}
