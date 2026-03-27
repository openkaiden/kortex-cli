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

package podman

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/config"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/exec"
	"github.com/kortex-hub/kortex-cli/pkg/system"
)

func TestNew(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt == nil {
		t.Fatal("New() returned nil")
	}

	if rt.Type() != "podman" {
		t.Errorf("Expected type 'podman', got %s", rt.Type())
	}
}

func TestPodmanRuntime_Available(t *testing.T) {
	t.Parallel()

	t.Run("returns true when podman command exists", func(t *testing.T) {
		t.Parallel()

		fakeSys := &fakeSystem{commandExists: true}
		rt := newWithDeps(fakeSys, exec.New())

		avail, ok := rt.(interface{ Available() bool })
		if !ok {
			t.Fatal("Expected runtime to implement Available interface")
		}

		if !avail.Available() {
			t.Error("Expected Available() to return true when podman exists")
		}

		if fakeSys.checkedCommand != "podman" {
			t.Errorf("Expected to check for 'podman' command, got '%s'", fakeSys.checkedCommand)
		}
	})

	t.Run("returns false when podman command does not exist", func(t *testing.T) {
		t.Parallel()

		fakeSys := &fakeSystem{commandExists: false}
		rt := newWithDeps(fakeSys, exec.New())

		avail, ok := rt.(interface{ Available() bool })
		if !ok {
			t.Fatal("Expected runtime to implement Available interface")
		}

		if avail.Available() {
			t.Error("Expected Available() to return false when podman does not exist")
		}

		if fakeSys.checkedCommand != "podman" {
			t.Errorf("Expected to check for 'podman' command, got '%s'", fakeSys.checkedCommand)
		}
	})
}

func TestPodmanRuntime_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("creates config directory and default configs", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rt := newWithDeps(system.New(), exec.New())

		// Type assert to StorageAware to access Initialize method
		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err := storageAware.Initialize(storageDir)
		if err != nil {
			t.Fatalf("Initialize() failed: %v", err)
		}

		// Verify config directory was created
		configDir := filepath.Join(storageDir, "config")
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("Config directory was not created")
		}

		// Verify default image config was created
		imageConfigPath := filepath.Join(configDir, config.ImageConfigFileName)
		if _, err := os.Stat(imageConfigPath); os.IsNotExist(err) {
			t.Error("Default image config was not created")
		}

		// Verify default claude config was created
		claudeConfigPath := filepath.Join(configDir, config.ClaudeConfigFileName)
		if _, err := os.Stat(claudeConfigPath); os.IsNotExist(err) {
			t.Error("Default claude config was not created")
		}
	})

	t.Run("returns error for empty storage directory", func(t *testing.T) {
		t.Parallel()

		rt := newWithDeps(system.New(), exec.New())

		// Type assert to StorageAware to access Initialize method
		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err := storageAware.Initialize("")
		if err == nil {
			t.Error("Expected error for empty storage directory")
		}
	})

	t.Run("does not overwrite existing configs", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rt := newWithDeps(system.New(), exec.New())

		// Type assert to StorageAware to access Initialize method
		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		// Initialize once to create defaults
		err := storageAware.Initialize(storageDir)
		if err != nil {
			t.Fatalf("First Initialize() failed: %v", err)
		}

		// Modify the image config
		configDir := filepath.Join(storageDir, "config")
		imageConfigPath := filepath.Join(configDir, config.ImageConfigFileName)
		customContent := []byte(`{"version":"40","packages":[],"sudo":[],"run_commands":[]}`)
		if err := os.WriteFile(imageConfigPath, customContent, 0644); err != nil {
			t.Fatalf("Failed to write custom config: %v", err)
		}

		// Initialize again
		rt2 := newWithDeps(system.New(), exec.New())
		storageAware2, ok := rt2.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err = storageAware2.Initialize(storageDir)
		if err != nil {
			t.Fatalf("Second Initialize() failed: %v", err)
		}

		// Verify custom config was not overwritten
		content, err := os.ReadFile(imageConfigPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if string(content) != string(customContent) {
			t.Error("Custom config was overwritten")
		}
	})
}

func TestPodmanRuntime_WorkspaceSourcesPath(t *testing.T) {
	t.Parallel()

	rt := New()
	path := rt.WorkspaceSourcesPath()

	if path != "/workspace/sources" {
		t.Errorf("WorkspaceSourcesPath() = %q, want %q", path, "/workspace/sources")
	}

	// Verify it's consistent across calls
	path2 := rt.WorkspaceSourcesPath()
	if path != path2 {
		t.Errorf("WorkspaceSourcesPath() inconsistent: %q != %q", path, path2)
	}
}

func TestPodmanRuntime_ListAgents(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice when not initialized", func(t *testing.T) {
		t.Parallel()

		rt := newWithDeps(system.New(), exec.New())

		lister, ok := rt.(interface{ ListAgents() ([]string, error) })
		if !ok {
			t.Fatal("Expected runtime to implement AgentLister interface")
		}

		agents, err := lister.ListAgents()
		if err != nil {
			t.Fatalf("ListAgents() failed: %v", err)
		}

		if len(agents) != 0 {
			t.Errorf("Expected empty agents, got: %v", agents)
		}
	})

	t.Run("returns agents after initialization", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rt := newWithDeps(system.New(), exec.New())

		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err := storageAware.Initialize(storageDir)
		if err != nil {
			t.Fatalf("Initialize() failed: %v", err)
		}

		lister, ok := rt.(interface{ ListAgents() ([]string, error) })
		if !ok {
			t.Fatal("Expected runtime to implement AgentLister interface")
		}

		agents, err := lister.ListAgents()
		if err != nil {
			t.Fatalf("ListAgents() failed: %v", err)
		}

		// Default initialization creates config files for all default agents
		expected := []string{"claude", "cursor", "goose"}
		if !slices.Equal(agents, expected) {
			t.Errorf("Expected %v, got: %v", expected, agents)
		}
	})
}

// fakeSystem is a fake implementation of system.System for testing.
type fakeSystem struct {
	commandExists  bool
	checkedCommand string
	uid            int
	gid            int
}

// Ensure fakeSystem implements system.System at compile time.
var _ system.System = (*fakeSystem)(nil)

func (f *fakeSystem) CommandExists(name string) bool {
	f.checkedCommand = name
	return f.commandExists
}

func (f *fakeSystem) Getuid() int {
	if f.uid == 0 {
		return 1000 // Default UID for tests
	}
	return f.uid
}

func (f *fakeSystem) Getgid() int {
	if f.gid == 0 {
		return 1000 // Default GID for tests
	}
	return f.gid
}
