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

	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestWriteReadSandboxData(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	data := sandboxData{
		SourcePath: "/home/user/project",
		ProjectID:  "abc123",
		Agent:      "claude",
	}

	if err := rt.writeSandboxData("kdn-test", data); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	got, err := rt.readSandboxData("kdn-test")
	if err != nil {
		t.Fatalf("readSandboxData() failed: %v", err)
	}

	if got.SourcePath != data.SourcePath {
		t.Errorf("SourcePath = %q, want %q", got.SourcePath, data.SourcePath)
	}
	if got.ProjectID != data.ProjectID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, data.ProjectID)
	}
	if got.Agent != data.Agent {
		t.Errorf("Agent = %q, want %q", got.Agent, data.Agent)
	}
}

func TestReadSandboxData_NotFound(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	_, err := rt.readSandboxData("nonexistent")
	if err == nil {
		t.Error("Expected error for missing sandbox data")
	}
}

func TestRemoveSandboxData(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	data := sandboxData{SourcePath: "/src", ProjectID: "id", Agent: "claude"}
	if err := rt.writeSandboxData("kdn-test", data); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	dir := rt.sandboxDataDir("kdn-test")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Expected sandbox data directory to exist: %v", err)
	}

	rt.removeSandboxData("kdn-test")

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Expected sandbox data directory to be removed, got err: %v", err)
	}
}

func TestReadSandboxData_InvalidJSON(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	dir := rt.sandboxDataDir("kdn-test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, sandboxDataFile), []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	_, err := rt.readSandboxData("kdn-test")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestWriteSandboxData_UnwritableDir(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	rt := newWithDeps(exec.NewFake(), "/fake/gw", blocker)

	err := rt.writeSandboxData("kdn-test", sandboxData{
		SourcePath: "/src",
		ProjectID:  "id",
		Agent:      "claude",
	})
	if err == nil {
		t.Error("Expected error when storage directory does not exist")
	}
}

func TestSandboxDataDir(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", "/storage")

	got := rt.sandboxDataDir("kdn-test")
	want := filepath.Join("/storage", "sandboxes", "kdn-test")
	if got != want {
		t.Errorf("sandboxDataDir() = %q, want %q", got, want)
	}
}

func TestWriteReadSandboxData_WithPorts(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	data := sandboxData{
		SourcePath: "/home/user/project",
		ProjectID:  "abc123",
		Agent:      "openclaw",
		Ports:      []int{18789, 8080},
	}

	if err := rt.writeSandboxData("kdn-test", data); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	got, err := rt.readSandboxData("kdn-test")
	if err != nil {
		t.Fatalf("readSandboxData() failed: %v", err)
	}

	if len(got.Ports) != 2 {
		t.Fatalf("Ports length = %d, want 2", len(got.Ports))
	}
	if got.Ports[0] != 18789 {
		t.Errorf("Ports[0] = %d, want 18789", got.Ports[0])
	}
	if got.Ports[1] != 8080 {
		t.Errorf("Ports[1] = %d, want 8080", got.Ports[1])
	}
}

func TestWriteReadSandboxData_WithoutPorts(t *testing.T) {
	t.Parallel()

	rt := newWithDeps(exec.NewFake(), "/fake/gw", t.TempDir())

	data := sandboxData{
		SourcePath: "/home/user/project",
		ProjectID:  "abc123",
		Agent:      "claude",
	}

	if err := rt.writeSandboxData("kdn-test", data); err != nil {
		t.Fatalf("writeSandboxData() failed: %v", err)
	}

	got, err := rt.readSandboxData("kdn-test")
	if err != nil {
		t.Fatalf("readSandboxData() failed: %v", err)
	}

	if got.Ports != nil {
		t.Errorf("Ports = %v, want nil", got.Ports)
	}
}
