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
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime/podman/config"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/system"
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

		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err := storageAware.Initialize(storageDir)
		if err != nil {
			t.Fatalf("Initialize() failed: %v", err)
		}

		configDir := filepath.Join(storageDir, "config")
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("Config directory was not created")
		}

		imageConfigPath := filepath.Join(configDir, config.ImageConfigFileName)
		if _, err := os.Stat(imageConfigPath); os.IsNotExist(err) {
			t.Error("Default image config was not created")
		}

		claudeConfigPath := filepath.Join(configDir, config.ClaudeConfigFileName)
		if _, err := os.Stat(claudeConfigPath); os.IsNotExist(err) {
			t.Error("Default claude config was not created")
		}
	})

	t.Run("returns error for empty storage directory", func(t *testing.T) {
		t.Parallel()

		rt := newWithDeps(system.New(), exec.New())

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

		storageAware, ok := rt.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err := storageAware.Initialize(storageDir)
		if err != nil {
			t.Fatalf("First Initialize() failed: %v", err)
		}

		configDir := filepath.Join(storageDir, "config")
		imageConfigPath := filepath.Join(configDir, config.ImageConfigFileName)
		customContent := []byte(`{"version":"40","packages":[],"sudo":[],"run_commands":[]}`)
		if err := os.WriteFile(imageConfigPath, customContent, 0644); err != nil {
			t.Fatalf("Failed to write custom config: %v", err)
		}

		rt2 := newWithDeps(system.New(), exec.New())
		storageAware2, ok := rt2.(interface{ Initialize(string) error })
		if !ok {
			t.Fatal("Expected runtime to implement StorageAware interface")
		}

		err = storageAware2.Initialize(storageDir)
		if err != nil {
			t.Fatalf("Second Initialize() failed: %v", err)
		}

		content, err := os.ReadFile(imageConfigPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if string(content) != string(customContent) {
			t.Error("Custom config was overwritten")
		}
	})
}

func TestWritePodFiles(t *testing.T) {
	t.Parallel()

	t.Run("creates YAML with workspace-specific pod name", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}
		containerID := "abc123"
		workspaceName := "my-project"

		err := p.writePodFiles(containerID, workspaceName)
		if err != nil {
			t.Fatalf("writePodFiles() failed: %v", err)
		}

		content, err := os.ReadFile(p.podYAMLPath(containerID))
		if err != nil {
			t.Fatalf("Failed to read pod YAML: %v", err)
		}

		if !strings.Contains(string(content), "  name: my-project\n") {
			t.Error("Pod YAML should contain workspace-specific pod name 'my-project'")
		}

		// Container name within pod should remain unchanged
		if !strings.Contains(string(content), "- name: onecli\n") {
			t.Error("Container name within pod should remain 'onecli'")
		}
	})

	t.Run("writes pod name file", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}
		containerID := "def456"
		workspaceName := "test-ws"

		err := p.writePodFiles(containerID, workspaceName)
		if err != nil {
			t.Fatalf("writePodFiles() failed: %v", err)
		}

		name, err := p.readPodName(containerID)
		if err != nil {
			t.Fatalf("readPodName() failed: %v", err)
		}

		if name != "test-ws" {
			t.Errorf("readPodName() = %q, want %q", name, "test-ws")
		}
	})

	t.Run("returns error for missing pod name file", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}

		_, err := p.readPodName("nonexistent")
		if err == nil {
			t.Error("Expected error for missing pod name file, got nil")
		}
	})
}

func TestCleanupPodFiles(t *testing.T) {
	t.Parallel()

	p := &podmanRuntime{storageDir: t.TempDir()}
	containerID := "abc123"

	if err := p.writePodFiles(containerID, "my-ws"); err != nil {
		t.Fatalf("writePodFiles() failed: %v", err)
	}

	if _, err := os.Stat(p.podDir(containerID)); os.IsNotExist(err) {
		t.Fatal("Pod directory should exist before cleanup")
	}

	p.cleanupPodFiles(containerID)

	if _, err := os.Stat(p.podDir(containerID)); !os.IsNotExist(err) {
		t.Error("Pod directory should be removed after cleanup")
	}
}

func TestReplaceYAMLPodName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		workspace string
		expected  string
	}{
		{"my-project", "  name: my-project\n"},
		{"test", "  name: test\n"},
		{"foo-bar-baz", "  name: foo-bar-baz\n"},
	}

	for _, tt := range tests {
		t.Run(tt.workspace, func(t *testing.T) {
			t.Parallel()

			result := replaceYAMLPodName(tt.workspace)
			if !strings.Contains(string(result), tt.expected) {
				t.Errorf("replaceYAMLPodName(%q) should contain %q", tt.workspace, tt.expected)
			}
			// The original "  name: onecli\n" should be replaced
			if strings.Contains(string(result), "  name: onecli\n") {
				t.Errorf("replaceYAMLPodName(%q) should not contain original 'onecli' pod name", tt.workspace)
			}
		})
	}
}

func TestPodmanRuntime_WorkspaceSourcesPath(t *testing.T) {
	t.Parallel()

	rt := New()
	path := rt.WorkspaceSourcesPath()

	if path != "/workspace/sources" {
		t.Errorf("WorkspaceSourcesPath() = %q, want %q", path, "/workspace/sources")
	}

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

		expected := []string{"claude", "cursor", "goose", "opencode"}
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
