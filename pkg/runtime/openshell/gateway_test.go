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
	expectedStateDir := fmt.Sprintf("OPENSHELL_VM_DRIVER_STATE_DIR=%s/vm-driver", storageDir)
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
