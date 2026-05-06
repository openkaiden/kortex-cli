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
	"os"
	"path/filepath"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

func TestNew(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt == nil {
		t.Fatal("New() returned nil")
	}

	if rt.Type() != "openshell" {
		t.Errorf("Expected type 'openshell', got %s", rt.Type())
	}
}

func TestOpenshellRuntime_Available(t *testing.T) {
	t.Parallel()

	rt := New()

	avail, ok := rt.(interface{ Available() bool })
	if !ok {
		t.Fatal("Expected runtime to implement Available interface")
	}

	_, err := platformAsset("openshell-gateway")
	expected := err == nil
	if avail.Available() != expected {
		t.Errorf("Expected Available() to return %v on this platform, got %v", expected, avail.Available())
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

	rt := newWithDeps(exec.NewFake(), "/fake/openshell-gateway", t.TempDir())
	if rt.Type() != "openshell" {
		t.Errorf("Type() = %q, want %q", rt.Type(), "openshell")
	}
}

func TestOpenshellRuntime_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	var _ runtime.Runtime = (*openshellRuntime)(nil)
	var _ runtime.StorageAware = (*openshellRuntime)(nil)
	var _ runtime.Terminal = (*openshellRuntime)(nil)
	var _ runtime.FlagProvider = (*openshellRuntime)(nil)
}

func TestOpenshellRuntime_Flags(t *testing.T) {
	t.Parallel()

	rt := &openshellRuntime{}
	flags := rt.Flags()

	if len(flags) != 1 {
		t.Fatalf("Expected 1 flag, got %d", len(flags))
	}

	if flags[0].Name != "openshell-driver" {
		t.Errorf("Expected first flag name 'openshell-driver', got %q", flags[0].Name)
	}
	if len(flags[0].Completions) != 2 {
		t.Errorf("Expected 2 completions for openshell-driver, got %d", len(flags[0].Completions))
	}
}

func TestOpenshellRuntime_SetSecretServiceRegistry(t *testing.T) {
	t.Parallel()

	rt := &openshellRuntime{}
	registry := &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"test": &fakeSecretService{hosts: []string{"example.com"}},
		},
	}

	rt.SetSecretServiceRegistry(registry)

	if rt.secretServiceRegistry == nil {
		t.Error("Expected secretServiceRegistry to be set")
	}
}

func TestOpenshellRuntime_Initialize_EmptyStorageDir(t *testing.T) {
	t.Parallel()

	rt := &openshellRuntime{}
	err := rt.Initialize("")
	if err == nil {
		t.Error("Expected error for empty storage directory")
	}
}

func TestOpenshellRuntime_Initialize_SetsUpState(t *testing.T) {
	t.Parallel()

	rt := &openshellRuntime{}
	storageDir := t.TempDir()
	err := rt.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if rt.storageDir != storageDir {
		t.Errorf("storageDir = %q, want %q", rt.storageDir, storageDir)
	}
	if rt.states == nil {
		t.Error("Expected states to be initialized")
	}
	if rt.config.Driver != defaultDriver {
		t.Errorf("Expected default driver %q, got %q", defaultDriver, rt.config.Driver)
	}
}

func TestOpenshellRuntime_Initialize_DoesNotDownload(t *testing.T) {
	t.Parallel()

	rt := &openshellRuntime{}
	storageDir := t.TempDir()
	err := rt.Initialize(storageDir)
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if rt.executor != nil {
		t.Error("Expected executor to be nil after Initialize (downloads are deferred)")
	}
	if rt.gatewayBinaryPath != "" {
		t.Error("Expected gatewayBinaryPath to be empty after Initialize (downloads are deferred)")
	}
}

func TestOpenshellRuntime_EnsureBinaries_SkipsWhenDepsInjected(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.ensureBinaries()
	if err != nil {
		t.Fatalf("ensureBinaries() should be no-op for test deps: %v", err)
	}

	if rt.executor != fakeExec {
		t.Error("Expected executor to remain the injected fake")
	}
}

func TestDownloadBinaries_WithPreExistingBinaries(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	binDir := filepath.Join(storageDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	for _, name := range []string{"openshell-gateway", "openshell"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("fake binary"), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}
	if _, err := platformAsset("openshell-driver-vm"); err == nil {
		if err := os.WriteFile(filepath.Join(binDir, "openshell-driver-vm"), []byte("fake vm driver"), 0755); err != nil {
			t.Fatalf("Failed to create openshell-driver-vm: %v", err)
		}
	}

	rt := &openshellRuntime{storageDir: storageDir}
	err := rt.downloadBinaries()
	if err != nil {
		t.Fatalf("downloadBinaries() with existing binaries: %v", err)
	}

	if rt.gatewayBinaryPath == "" {
		t.Error("Expected gatewayBinaryPath to be set")
	}
	if rt.executor == nil {
		t.Error("Expected executor to be set")
	}
}

func TestEnsureBinaries_RunsOnce(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	binDir := filepath.Join(storageDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	for _, name := range []string{"openshell-gateway", "openshell"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("fake binary"), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}
	if _, err := platformAsset("openshell-driver-vm"); err == nil {
		if err := os.WriteFile(filepath.Join(binDir, "openshell-driver-vm"), []byte("fake"), 0755); err != nil {
			t.Fatalf("Failed to create vm driver: %v", err)
		}
	}

	rt := &openshellRuntime{storageDir: storageDir}

	// Call ensureBinaries twice — downloadBinaries should only execute once
	if err := rt.ensureBinaries(); err != nil {
		t.Fatalf("First ensureBinaries() call: %v", err)
	}
	firstExecutor := rt.executor

	if err := rt.ensureBinaries(); err != nil {
		t.Fatalf("Second ensureBinaries() call: %v", err)
	}

	if rt.executor != firstExecutor {
		t.Error("Expected executor to be the same instance after second call (sync.Once)")
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
