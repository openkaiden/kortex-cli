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
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestEnsureGatewayRegistered_AlreadyRegistered(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "gateway" && args[1] == "add" {
			return fmt.Errorf("exit status 1\nopenshell stderr:\nGateway '127.0.0.1' already exists.")
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.ensureGatewayRegistered(context.Background())
	if err != nil {
		t.Fatalf("ensureGatewayRegistered() should succeed when gateway already exists: %v", err)
	}
}

func TestEnsureGatewayRegistered_RegistersNew(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.ensureGatewayRegistered(context.Background())
	if err != nil {
		t.Fatalf("ensureGatewayRegistered() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}

	call := fakeExec.RunCalls[0]
	if len(call) < 4 {
		t.Fatalf("Expected at least 4 args, got %d: %v", len(call), call)
	}
	if call[0] != "gateway" || call[1] != "add" {
		t.Errorf("Expected 'gateway add', got %v", call[:2])
	}
	if call[2] != gatewayURL {
		t.Errorf("Expected gateway URL %q, got %q", gatewayURL, call[2])
	}
	if call[3] != "--local" {
		t.Errorf("Expected '--local' flag, got %q", call[3])
	}
}

func TestBuildGatewayCommand_Podman(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverPodman

	cmd, err := rt.buildGatewayCommand("abc123")
	if err != nil {
		t.Fatalf("buildGatewayCommand() failed: %v", err)
	}

	args := cmd.Args
	// Should contain --drivers podman
	found := false
	for i, arg := range args {
		if arg == "--drivers" && i+1 < len(args) && args[i+1] == "podman" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --drivers podman in args: %v", args)
	}

	// Should use file-based sqlite
	foundDB := false
	for i, arg := range args {
		if arg == "--db-url" && i+1 < len(args) && strings.HasPrefix(args[i+1], "sqlite:") && strings.HasSuffix(args[i+1], "openshell-podman.db") {
			foundDB = true
			break
		}
	}
	if !foundDB {
		t.Errorf("Expected --db-url sqlite:<path>/openshell-podman.db in args: %v", args)
	}

	// Should have env vars set
	hasEnv := false
	for _, env := range cmd.Env {
		if fmt.Sprintf("OPENSHELL_SUPERVISOR_IMAGE=%s", supervisorImage) == env {
			hasEnv = true
			break
		}
	}
	if !hasEnv {
		t.Error("Expected OPENSHELL_SUPERVISOR_IMAGE env var for podman driver")
	}
}

func TestBuildGatewayCommand_VM(t *testing.T) {
	t.Parallel()

	if _, err := platformAsset("openshell-driver-vm"); err != nil {
		t.Skipf("VM driver not supported on this platform: %v", err)
	}

	storageDir := t.TempDir()
	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", storageDir)
	rt.config.Driver = DriverVM

	cmd, err := rt.buildGatewayCommand("abc123")
	if err != nil {
		t.Fatalf("buildGatewayCommand() failed: %v", err)
	}

	args := cmd.Args
	// Should contain --drivers vm
	found := false
	for i, arg := range args {
		if arg == "--drivers" && i+1 < len(args) && args[i+1] == "vm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --drivers vm in args: %v", args)
	}

	// Should contain --driver-dir
	foundDriverDir := false
	for i, arg := range args {
		if arg == "--driver-dir" && i+1 < len(args) {
			foundDriverDir = true
			break
		}
	}
	if !foundDriverDir {
		t.Errorf("Expected --driver-dir in args: %v", args)
	}

	// Should contain --grpc-endpoint
	foundGRPC := false
	for i, arg := range args {
		if arg == "--grpc-endpoint" && i+1 < len(args) && args[i+1] == gatewayURL {
			foundGRPC = true
			break
		}
	}
	if !foundGRPC {
		t.Errorf("Expected --grpc-endpoint %s in args: %v", gatewayURL, args)
	}

	// Should contain --ssh-handshake-secret as a flag (not env var)
	foundSecret := false
	for i, arg := range args {
		if arg == "--ssh-handshake-secret" && i+1 < len(args) && args[i+1] == "abc123" {
			foundSecret = true
			break
		}
	}
	if !foundSecret {
		t.Errorf("Expected --ssh-handshake-secret abc123 in args: %v", args)
	}

	// Should NOT have OPENSHELL_SUPERVISOR_IMAGE env var
	for _, env := range cmd.Env {
		if fmt.Sprintf("OPENSHELL_SUPERVISOR_IMAGE=%s", supervisorImage) == env {
			t.Error("VM driver should not set OPENSHELL_SUPERVISOR_IMAGE env var")
			break
		}
	}

	// Should have OPENSHELL_VM_DRIVER_STATE_DIR env var pointing to storage
	expectedStateDir := fmt.Sprintf("OPENSHELL_VM_DRIVER_STATE_DIR=%s", filepath.Join(storageDir, "vm-driver"))
	hasStateDir := false
	for _, env := range cmd.Env {
		if env == expectedStateDir {
			hasStateDir = true
			break
		}
	}
	if !hasStateDir {
		t.Errorf("Expected OPENSHELL_VM_DRIVER_STATE_DIR env var, got env: %v", cmd.Env)
	}
}

func TestBuildGatewayCommand_UnsupportedDriver(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = "invalid"

	_, err := rt.buildGatewayCommand("abc123")
	if err == nil {
		t.Error("Expected error for unsupported driver")
	}
}

func TestBuildGatewayCommand_DefaultDriverUsesPodman(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = ""

	cmd, err := rt.buildGatewayCommand("abc123")
	if err != nil {
		t.Fatalf("buildGatewayCommand() failed: %v", err)
	}

	found := false
	for i, arg := range cmd.Args {
		if arg == "--drivers" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "podman" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected --drivers podman for empty driver, got args: %v", cmd.Args)
	}
}

func TestGatewayExitError_WithLog(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", storageDir)

	logContent := "line1\nline2\nerror: failed to start\n"
	if err := os.WriteFile(filepath.Join(storageDir, gatewayLogFile), []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}

	err := rt.gatewayExitError(fmt.Errorf("exit status 1"))
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("Expected exit error in message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "error: failed to start") {
		t.Errorf("Expected log content in message, got: %v", err)
	}
}

func TestGatewayExitError_WithoutLog(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", storageDir)

	err := rt.gatewayExitError(fmt.Errorf("exit status 1"))
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Errorf("Expected 'exited unexpectedly' in message, got: %v", err)
	}
}

func TestGatewayExitError_TruncatesLongLog(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", storageDir)

	var lines []string
	for i := range 30 {
		lines = append(lines, fmt.Sprintf("log line %d", i))
	}
	logContent := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(storageDir, gatewayLogFile), []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}

	err := rt.gatewayExitError(fmt.Errorf("exit status 1"))
	if err == nil {
		t.Fatal("Expected error")
	}
	// Should contain last lines but not the first ones (truncated to 20)
	if !strings.Contains(err.Error(), "log line 29") {
		t.Errorf("Expected last log line in message, got: %v", err)
	}
	if strings.Contains(err.Error(), "log line 0") {
		t.Errorf("Expected first log lines to be truncated, got: %v", err)
	}
}

func TestIsGatewayReady_True(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if !rt.isGatewayReady(context.Background()) {
		t.Error("Expected isGatewayReady to return true when executor succeeds")
	}
}

func TestIsGatewayReady_False(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if rt.isGatewayReady(context.Background()) {
		t.Error("Expected isGatewayReady to return false when executor fails")
	}
}

func TestEnsureGatewayRunning_AlreadyReady(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("ensureGatewayRunning() should succeed when gateway is ready: %v", err)
	}
}

func TestEnsureGatewayRunning_RegistersBeforeCheck(t *testing.T) {
	t.Parallel()

	var callOrder []string
	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "gateway" && args[1] == "add" {
			callOrder = append(callOrder, "register")
		}
		return nil
	}
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			callOrder = append(callOrder, "ready-check")
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("ensureGatewayRunning() failed: %v", err)
	}

	if len(callOrder) < 2 {
		t.Fatalf("Expected at least 2 calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "register" {
		t.Errorf("Expected register before ready-check, got order: %v", callOrder)
	}
}

func TestEnsureGatewayRunning_StartsGatewayProcess(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("Shell scripts not available on Windows")
	}

	storageDir := t.TempDir()

	gatewayScript := filepath.Join(storageDir, "fake-gateway")
	if err := os.WriteFile(gatewayScript, []byte("#!/bin/sh\nexec sleep 30\n"), 0755); err != nil {
		t.Fatalf("Failed to write gateway script: %v", err)
	}

	readyCalls := 0
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			readyCalls++
			if readyCalls <= 1 {
				return nil, fmt.Errorf("not ready")
			}
			return []byte("kdn-test\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, gatewayScript, storageDir)

	t.Cleanup(func() {
		state, err := loadGatewayState(storageDir)
		if err == nil && state.PID > 0 {
			if p, err := os.FindProcess(state.PID); err == nil {
				p.Kill()
			}
		}
	})

	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("ensureGatewayRunning() should start gateway and detect readiness: %v", err)
	}

	// Verify gateway state was written
	state, err := loadGatewayState(storageDir)
	if err != nil {
		t.Fatalf("Expected gateway state to be created: %v", err)
	}
	if state.PID <= 0 {
		t.Errorf("Expected valid PID in state, got %d", state.PID)
	}
	if state.Driver != DriverPodman {
		t.Errorf("Expected driver %q in state, got %q", DriverPodman, state.Driver)
	}

	// Verify log file was created
	if _, err := os.Stat(filepath.Join(storageDir, gatewayLogFile)); err != nil {
		t.Errorf("Expected log file to be created: %v", err)
	}
}

func TestEnsureGatewayRunning_GatewayExitsEarly(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("Shell scripts not available on Windows")
	}

	storageDir := t.TempDir()

	// Create a gateway that exits immediately with an error
	gatewayScript := filepath.Join(storageDir, "fake-gateway")
	if err := os.WriteFile(gatewayScript, []byte("#!/bin/sh\necho 'fatal error' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatalf("Failed to write gateway script: %v", err)
	}

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return nil, fmt.Errorf("not ready")
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, gatewayScript, storageDir)

	err := rt.ensureGatewayRunning(context.Background())
	if err == nil {
		t.Fatal("Expected error when gateway exits immediately")
	}
	if !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Errorf("Expected 'exited unexpectedly' error, got: %v", err)
	}
}

func TestEnsureGatewayRunning_ContextCancelled(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("Shell scripts not available on Windows")
	}

	storageDir := t.TempDir()

	gatewayScript := filepath.Join(storageDir, "fake-gateway")
	if err := os.WriteFile(gatewayScript, []byte("#!/bin/sh\nexec sleep 30\n"), 0755); err != nil {
		t.Fatalf("Failed to write gateway script: %v", err)
	}

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("not ready")
	}

	rt := newWithDeps(fakeExec, gatewayScript, storageDir)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	t.Cleanup(func() {
		state, loadErr := loadGatewayState(storageDir)
		if loadErr == nil && state.PID > 0 {
			if p, findErr := os.FindProcess(state.PID); findErr == nil {
				p.Kill()
			}
		}
	})

	err := rt.ensureGatewayRunning(ctx)
	if err == nil {
		t.Fatal("Expected error when context is cancelled")
	}
}

func TestEnsureGatewayRunning_BuildCommandError(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("not ready")
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.config.Driver = "invalid-driver"

	err := rt.ensureGatewayRunning(context.Background())
	if err == nil {
		t.Fatal("Expected error for invalid driver")
	}
	if !strings.Contains(err.Error(), "unsupported gateway driver") {
		t.Errorf("Expected 'unsupported gateway driver' error, got: %v", err)
	}
}

func TestHasActiveSandboxes_WithKdnSandboxes(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-myproject\nother-sandbox\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if !rt.hasActiveSandboxes(context.Background()) {
		t.Error("Expected hasActiveSandboxes to return true when kdn-prefixed sandboxes exist")
	}
}

func TestHasActiveSandboxes_NoKdnSandboxes(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("other-sandbox\nunrelated\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if rt.hasActiveSandboxes(context.Background()) {
		t.Error("Expected hasActiveSandboxes to return false when no kdn-prefixed sandboxes exist")
	}
}

func TestHasActiveSandboxes_EmptyList(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if rt.hasActiveSandboxes(context.Background()) {
		t.Error("Expected hasActiveSandboxes to return false on empty list")
	}
}

func TestHasActiveSandboxes_ExecutorError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	if rt.hasActiveSandboxes(context.Background()) {
		t.Error("Expected hasActiveSandboxes to return false on executor error")
	}
}

func TestEnsureGatewayRunning_DriverConflict_WithActiveSandboxes(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()

	// Write state indicating gateway is running with VM driver
	_ = saveGatewayState(storageDir, gatewayState{PID: 99999, Driver: DriverVM})

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-myproject\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.config.Driver = DriverPodman

	err := rt.ensureGatewayRunning(context.Background())
	if err == nil {
		t.Fatal("Expected error for driver conflict with active sandboxes")
	}
	if !strings.Contains(err.Error(), "cannot switch") {
		t.Errorf("Expected 'cannot switch' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), DriverVM) || !strings.Contains(err.Error(), DriverPodman) {
		t.Errorf("Expected both driver names in error, got: %v", err)
	}
}

func TestEnsureGatewayRunning_DriverConflict_NoActiveSandboxes(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("Shell scripts not available on Windows")
	}

	storageDir := t.TempDir()

	// Start a real sleep process to act as the old gateway
	gatewayScript := filepath.Join(storageDir, "fake-gateway")
	if err := os.WriteFile(gatewayScript, []byte("#!/bin/sh\nexec sleep 30\n"), 0755); err != nil {
		t.Fatalf("Failed to write gateway script: %v", err)
	}

	readyCalls := 0
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			readyCalls++
			if readyCalls <= 1 {
				// First call: gateway is "ready" (old driver)
				return []byte(""), nil
			}
			if readyCalls == 2 {
				// Second call: hasActiveSandboxes check — no kdn sandboxes
				return []byte(""), nil
			}
			if readyCalls <= 4 {
				// After stop, gateway not ready yet
				return nil, fmt.Errorf("not ready")
			}
			// Eventually ready with new driver
			return []byte(""), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, gatewayScript, storageDir)
	rt.config.Driver = DriverPodman

	// Write state indicating gateway is running with VM driver, using a real PID
	// We need a real process. Start one.
	sleepCmd := exec.NewFake() //nolint:ineffassign,staticcheck
	_ = sleepCmd
	// Actually write the state with a fake PID that doesn't exist
	_ = saveGatewayState(storageDir, gatewayState{PID: 99999, Driver: DriverVM})

	t.Cleanup(func() {
		state, loadErr := loadGatewayState(storageDir)
		if loadErr == nil && state.PID > 0 {
			if p, findErr := os.FindProcess(state.PID); findErr == nil {
				p.Kill()
			}
		}
	})

	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("Expected driver switch to succeed with no active sandboxes: %v", err)
	}

	// Verify gateway state was updated with new driver
	state, err := loadGatewayState(storageDir)
	if err != nil {
		t.Fatalf("Expected gateway state after restart: %v", err)
	}
	if state.Driver != DriverPodman {
		t.Errorf("Expected driver %q after switch, got %q", DriverPodman, state.Driver)
	}
}

func TestEnsureGatewayRunning_SameDriver_NoConflict(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	_ = saveGatewayState(storageDir, gatewayState{PID: 99999, Driver: DriverPodman})

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.config.Driver = DriverPodman

	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("Expected no error for same driver: %v", err)
	}
}

func TestEnsureGatewayRunning_UnknownRunningDriver_NoConflict(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	// No gateway state file — backward compat with old installs

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-test\n"), nil
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)
	rt.config.Driver = DriverVM

	err := rt.ensureGatewayRunning(context.Background())
	if err != nil {
		t.Fatalf("Expected no error when gateway state is unknown: %v", err)
	}
}

func TestStopGateway_NoStateFile(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", storageDir)

	err := rt.stopGateway(context.Background())
	if err != nil {
		t.Fatalf("Expected no error when no state file exists: %v", err)
	}
}

func TestStopGateway_DeadProcess(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	_ = saveGatewayState(storageDir, gatewayState{PID: 99999, Driver: DriverPodman})

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("not ready")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	err := rt.stopGateway(context.Background())
	if err != nil {
		t.Fatalf("Expected no error for dead process: %v", err)
	}

	// Verify state files were cleaned up
	if _, err := loadGatewayState(storageDir); err == nil {
		t.Error("Expected gateway state file to be removed")
	}
}

func TestGatewayState_RoundTrip(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	state := gatewayState{PID: 12345, Driver: DriverVM}

	if err := saveGatewayState(storageDir, state); err != nil {
		t.Fatalf("saveGatewayState() failed: %v", err)
	}

	loaded, err := loadGatewayState(storageDir)
	if err != nil {
		t.Fatalf("loadGatewayState() failed: %v", err)
	}
	if loaded.PID != state.PID {
		t.Errorf("Expected PID %d, got %d", state.PID, loaded.PID)
	}
	if loaded.Driver != state.Driver {
		t.Errorf("Expected driver %q, got %q", state.Driver, loaded.Driver)
	}
}

func TestGatewayState_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := loadGatewayState(t.TempDir())
	if err == nil {
		t.Error("Expected error when state file is missing")
	}
}

func TestGatewayState_InvalidJSON(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(storageDir, gatewayStateFile), []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := loadGatewayState(storageDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestRemoveGatewayState_CleansUpBothFiles(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()

	// Create both state file and legacy PID file
	_ = saveGatewayState(storageDir, gatewayState{PID: 123, Driver: DriverPodman})
	_ = os.WriteFile(filepath.Join(storageDir, gatewayPIDFile), []byte("123"), 0644)

	removeGatewayState(storageDir)

	if _, err := os.Stat(filepath.Join(storageDir, gatewayStateFile)); err == nil {
		t.Error("Expected gateway state file to be removed")
	}
	if _, err := os.Stat(filepath.Join(storageDir, gatewayPIDFile)); err == nil {
		t.Error("Expected legacy PID file to be removed")
	}
}

func TestGatewayState_JSONFormat(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	_ = saveGatewayState(storageDir, gatewayState{PID: 42, Driver: DriverVM})

	data, err := os.ReadFile(filepath.Join(storageDir, gatewayStateFile))
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("State file is not valid JSON: %v", err)
	}
	if _, ok := raw["pid"]; !ok {
		t.Error("Expected 'pid' field in state JSON")
	}
	if _, ok := raw["driver"]; !ok {
		t.Error("Expected 'driver' field in state JSON")
	}
}

func TestGenerateSSHSecret(t *testing.T) {
	t.Parallel()

	secret, err := generateSSHSecret()
	if err != nil {
		t.Fatalf("generateSSHSecret() failed: %v", err)
	}

	// 16 bytes = 32 hex characters
	if len(secret) != 32 {
		t.Errorf("Expected 32 hex characters, got %d: %q", len(secret), secret)
	}

	// Verify uniqueness (generate another)
	secret2, err := generateSSHSecret()
	if err != nil {
		t.Fatalf("generateSSHSecret() failed: %v", err)
	}
	if secret == secret2 {
		t.Error("Expected unique secrets, got identical values")
	}
}
