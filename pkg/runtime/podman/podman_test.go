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

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
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

	t.Run("returns true when cli path is set", func(t *testing.T) {
		t.Parallel()

		path := "/usr/bin/podman"
		rt := &podmanRuntime{cli: &path}
		if !rt.Available() {
			t.Error("Expected Available() to return true when cli is set")
		}
	})

	t.Run("returns false when cli path is nil", func(t *testing.T) {
		t.Parallel()

		rt := &podmanRuntime{}
		if rt.Available() {
			t.Error("Expected Available() to return false when cli is nil")
		}
	})
}

func TestPodmanRuntime_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("creates config directory and default configs", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rt := newWithDeps(system.New(), exec.New("podman"))

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

		rt := newWithDeps(system.New(), exec.New("podman"))

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
		rt := newWithDeps(system.New(), exec.New("podman"))

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

		rt2 := newWithDeps(system.New(), exec.New("podman"))
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

	t.Run("creates YAML with workspace-specific pod name and ports", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}
		containerID := "abc123"

		data := podTemplateData{
			Name:              "my-project",
			OnecliWebPort:     30001,
			OnecliVersion:     "1.17",
			AgentUID:          1000,
			BaseImageRegistry: "registry.fedoraproject.org/fedora",
			BaseImageVersion:  "latest",
		}

		err := p.writePodFiles(containerID, data)
		if err != nil {
			t.Fatalf("writePodFiles() failed: %v", err)
		}

		content, err := os.ReadFile(p.podYAMLPath(containerID))
		if err != nil {
			t.Fatalf("Failed to read pod YAML: %v", err)
		}

		yamlStr := string(content)

		if !strings.Contains(yamlStr, "  name: my-project\n") {
			t.Error("Pod YAML should contain workspace-specific pod name 'my-project'")
		}

		if !strings.Contains(yamlStr, "- name: onecli\n") {
			t.Error("Container name within pod should remain 'onecli'")
		}

		if !strings.Contains(yamlStr, "hostPort: 30001") {
			t.Error("Pod YAML should contain onecli web hostPort 30001")
		}
		if !strings.Contains(yamlStr, "ghcr.io/onecli/onecli:1.17") {
			t.Error("Pod YAML should contain versioned onecli image")
		}
	})

	t.Run("writes pod name file", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}
		containerID := "def456"

		data := podTemplateData{
			Name:              "test-ws",
			OnecliWebPort:     40001,
			OnecliVersion:     "1.17",
			AgentUID:          1000,
			BaseImageRegistry: "registry.fedoraproject.org/fedora",
			BaseImageVersion:  "latest",
		}

		err := p.writePodFiles(containerID, data)
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

	data := podTemplateData{
		Name:              "my-ws",
		OnecliWebPort:     50001,
		OnecliVersion:     "1.17",
		AgentUID:          1000,
		BaseImageRegistry: "registry.fedoraproject.org/fedora",
		BaseImageVersion:  "latest",
	}

	if err := p.writePodFiles(containerID, data); err != nil {
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

func TestRenderPodYAML(t *testing.T) {
	t.Parallel()

	t.Run("renders all template fields", func(t *testing.T) {
		t.Parallel()

		data := podTemplateData{
			Name:               "my-project",
			OnecliWebPort:      32101,
			OnecliVersion:      "1.17",
			AgentUID:           1000,
			BaseImageRegistry:  "registry.fedoraproject.org/fedora",
			BaseImageVersion:   "latest",
			ApprovalHandlerDir: "/tmp/approval-handler/my-project",
		}

		result, err := renderPodYAML(data)
		if err != nil {
			t.Fatalf("renderPodYAML() failed: %v", err)
		}

		yamlStr := string(result)

		if !strings.Contains(yamlStr, "  name: my-project\n") {
			t.Error("Expected rendered YAML to contain pod name 'my-project'")
		}
		if !strings.Contains(yamlStr, "hostPort: 32101") {
			t.Error("Expected rendered YAML to contain onecli web hostPort 32101")
		}
		if !strings.Contains(yamlStr, "ghcr.io/onecli/onecli:1.17") {
			t.Error("Expected rendered YAML to contain versioned onecli image")
		}
		if !strings.Contains(yamlStr, "approval-handler") {
			t.Error("Expected rendered YAML to contain approval-handler container")
		}
		if !strings.Contains(yamlStr, "/tmp/approval-handler/my-project") {
			t.Error("Expected rendered YAML to contain approval handler hostPath")
		}
		if !strings.Contains(yamlStr, "volumeMounts") {
			t.Error("Expected rendered YAML to contain volumeMounts for approval-handler")
		}
		if !strings.Contains(yamlStr, "network-guard") {
			t.Error("Expected rendered YAML to contain network-guard container")
		}
		if !strings.Contains(yamlStr, "NET_ADMIN") {
			t.Error("Expected rendered YAML to contain NET_ADMIN capability for network-guard")
		}
		if !strings.Contains(yamlStr, "registry.fedoraproject.org/fedora:latest") {
			t.Error("Expected rendered YAML to contain base image for network-guard")
		}

		// Postgres (5432) and proxy (10255) ports should NOT have hostPort mappings
		if strings.Contains(yamlStr, "hostPort: 5432") {
			t.Error("Expected rendered YAML to NOT contain hostPort for postgres (5432)")
		}
		if strings.Contains(yamlStr, "hostPort: 10255") {
			t.Error("Expected rendered YAML to NOT contain hostPort for proxy (10255)")
		}
	})

	t.Run("does not contain original template placeholders", func(t *testing.T) {
		t.Parallel()

		data := podTemplateData{
			Name:               "test",
			OnecliWebPort:      10001,
			OnecliVersion:      "2.0",
			AgentUID:           1000,
			BaseImageRegistry:  "registry.fedoraproject.org/fedora",
			BaseImageVersion:   "42",
			ApprovalHandlerDir: "/tmp/approval-handler/test",
		}

		result, err := renderPodYAML(data)
		if err != nil {
			t.Fatalf("renderPodYAML() failed: %v", err)
		}

		yamlStr := string(result)

		if strings.Contains(yamlStr, "{{") || strings.Contains(yamlStr, "}}") {
			t.Error("Expected rendered YAML to not contain any template placeholders")
		}
	})
}

func TestFindFreePorts(t *testing.T) {
	t.Parallel()

	t.Run("returns requested number of ports", func(t *testing.T) {
		t.Parallel()

		ports, err := findFreePorts(3)
		if err != nil {
			t.Fatalf("findFreePorts() failed: %v", err)
		}

		if len(ports) != 3 {
			t.Fatalf("Expected 3 ports, got %d", len(ports))
		}

		for i, port := range ports {
			if port <= 0 || port > 65535 {
				t.Errorf("Port %d has invalid value: %d", i, port)
			}
		}
	})

	t.Run("returns unique ports", func(t *testing.T) {
		t.Parallel()

		ports, err := findFreePorts(3)
		if err != nil {
			t.Fatalf("findFreePorts() failed: %v", err)
		}

		seen := make(map[int]bool)
		for _, port := range ports {
			if seen[port] {
				t.Errorf("Duplicate port found: %d", port)
			}
			seen[port] = true
		}
	})

	t.Run("returns zero ports for zero count", func(t *testing.T) {
		t.Parallel()

		ports, err := findFreePorts(0)
		if err != nil {
			t.Fatalf("findFreePorts() failed: %v", err)
		}

		if len(ports) != 0 {
			t.Errorf("Expected 0 ports, got %d", len(ports))
		}
	})
}

func TestPodmanRuntime_DisplayName(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt.DisplayName() != "Podman" {
		t.Errorf("DisplayName() = %q, want %q", rt.DisplayName(), "Podman")
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

		rt := newWithDeps(system.New(), exec.New("podman"))

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
		rt := newWithDeps(system.New(), exec.New("podman"))

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

		expected := []string{"claude", "cursor", "goose", "openclaw", "opencode"}
		if !slices.Equal(agents, expected) {
			t.Errorf("Expected %v, got: %v", expected, agents)
		}
	})
}

func TestPodmanRuntime_TransformConfig(t *testing.T) {
	t.Parallel()

	t.Run("rewrites localhost URLs in MCP command args", func(t *testing.T) {
		t.Parallel()

		rt := &podmanRuntime{}
		args := []string{"--url", "http://localhost:8080/api"}
		config := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "test", Command: "cmd", Args: &args},
				},
			},
		}

		if err := rt.TransformConfig(config); err != nil {
			t.Fatalf("TransformConfig() error = %v", err)
		}

		got := (*(*config.Mcp.Commands)[0].Args)[1]
		want := "http://host.containers.internal:8080/api"
		if got != want {
			t.Errorf("arg = %q, want %q", got, want)
		}
	})

	t.Run("handles nil config", func(t *testing.T) {
		t.Parallel()

		rt := &podmanRuntime{}
		if err := rt.TransformConfig(nil); err != nil {
			t.Fatalf("TransformConfig(nil) error = %v", err)
		}
	})

	t.Run("handles config without MCP", func(t *testing.T) {
		t.Parallel()

		rt := &podmanRuntime{}
		config := &workspace.WorkspaceConfiguration{}
		if err := rt.TransformConfig(config); err != nil {
			t.Fatalf("TransformConfig() error = %v", err)
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
