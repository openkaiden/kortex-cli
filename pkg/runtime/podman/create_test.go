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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/config"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

func TestValidateCreateParams(t *testing.T) {
	t.Parallel()

	// Use a real temp directory for cross-platform testing
	tempSourcePath := t.TempDir()

	tests := []struct {
		name        string
		params      runtime.CreateParams
		expectError bool
		errorType   error
	}{
		{
			name: "valid parameters",
			params: runtime.CreateParams{
				Name:       "test-workspace",
				SourcePath: tempSourcePath,
				Agent:      "test_agent",
			},
			expectError: false,
		},
		{
			name: "missing name",
			params: runtime.CreateParams{
				Name:       "",
				SourcePath: tempSourcePath,
				Agent:      "test_agent",
			},
			expectError: true,
			errorType:   runtime.ErrInvalidParams,
		},
		{
			name: "missing source path",
			params: runtime.CreateParams{
				Name:       "test-workspace",
				SourcePath: "",
				Agent:      "test_agent",
			},
			expectError: true,
			errorType:   runtime.ErrInvalidParams,
		},
		{
			name: "missing both",
			params: runtime.CreateParams{
				Agent: "test_agent",
			},
			expectError: true,
			errorType:   runtime.ErrInvalidParams,
		},
		{
			name: "valid mount - $SOURCES target within /workspace",
			params: runtime.CreateParams{
				Name:       "test-workspace",
				SourcePath: tempSourcePath,
				Agent:      "test_agent",
				WorkspaceConfig: &workspace.WorkspaceConfiguration{
					Mounts: &[]workspace.Mount{
						{Host: "$SOURCES/../sibling", Target: "$SOURCES/../sibling"},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &podmanRuntime{}
			err := p.validateCreateParams(tt.params)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("Expected error type %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestCreateInstanceDirectory(t *testing.T) {
	t.Parallel()

	t.Run("creates instance directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		p := &podmanRuntime{storageDir: storageDir}

		instanceDir, err := p.createInstanceDirectory("test-workspace")
		if err != nil {
			t.Fatalf("createInstanceDirectory() failed: %v", err)
		}

		expectedDir := filepath.Join(storageDir, "instances", "test-workspace")
		if instanceDir != expectedDir {
			t.Errorf("Expected instance directory %s, got %s", expectedDir, instanceDir)
		}

		// Verify directory exists
		info, err := os.Stat(instanceDir)
		if err != nil {
			t.Errorf("Instance directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Instance path is not a directory")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		p := &podmanRuntime{storageDir: storageDir}

		instanceDir, err := p.createInstanceDirectory("test-workspace")
		if err != nil {
			t.Fatalf("createInstanceDirectory() failed: %v", err)
		}

		// Verify both "instances" and "test-workspace" directories exist
		instancesDir := filepath.Join(storageDir, "instances")
		if _, err := os.Stat(instancesDir); err != nil {
			t.Errorf("Instances directory was not created: %v", err)
		}
		if _, err := os.Stat(instanceDir); err != nil {
			t.Errorf("Instance directory was not created: %v", err)
		}
	})
}

func TestCreateContainerfile(t *testing.T) {
	t.Parallel()

	t.Run("creates Containerfile with default configs", func(t *testing.T) {
		t.Parallel()

		instanceDir := t.TempDir()
		p := &podmanRuntime{}

		// Create default configs
		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{"which", "procps-ng"},
			Sudo:        []string{"/usr/bin/dnf"},
			RunCommands: []string{},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{"curl -fsSL https://claude.ai/install.sh | bash"},
			TerminalCommand: []string{"claude"},
		}

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil, nil)
		if err != nil {
			t.Fatalf("createContainerfile() failed: %v", err)
		}

		// Verify Containerfile exists and starts with expected FROM line
		containerfilePath := filepath.Join(instanceDir, "Containerfile")
		content, err := os.ReadFile(containerfilePath)
		if err != nil {
			t.Fatalf("Failed to read Containerfile: %v", err)
		}

		expectedFirstLine := "FROM registry.fedoraproject.org/fedora:latest\n"
		lines := strings.Split(string(content), "\n")
		if len(lines) == 0 || lines[0]+"\n" != expectedFirstLine {
			t.Errorf("Expected Containerfile to start with:\n%s\nGot:\n%s", expectedFirstLine, lines[0])
		}

		// Verify sudoers file exists
		sudoersPath := filepath.Join(instanceDir, "sudoers")
		sudoersContent, err := os.ReadFile(sudoersPath)
		if err != nil {
			t.Fatalf("Failed to read sudoers: %v", err)
		}

		// Verify sudoers has ALLOWED alias
		if !strings.Contains(string(sudoersContent), "Cmnd_Alias ALLOWED") {
			t.Error("Expected sudoers to contain 'Cmnd_Alias ALLOWED'")
		}
	})

	t.Run("creates Containerfile with custom configs", func(t *testing.T) {
		t.Parallel()

		instanceDir := t.TempDir()
		p := &podmanRuntime{}

		// Create custom configs
		imageConfig := &config.ImageConfig{
			Version:     "40",
			Packages:    []string{"custom-package"},
			Sudo:        []string{"/usr/bin/custom"},
			RunCommands: []string{"echo 'custom setup'"},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{"agent-package"},
			RunCommands:     []string{"echo 'agent setup'"},
			TerminalCommand: []string{"custom-agent"},
		}

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil, nil)
		if err != nil {
			t.Fatalf("createContainerfile() failed: %v", err)
		}

		// Verify Containerfile contains custom version
		containerfilePath := filepath.Join(instanceDir, "Containerfile")
		content, err := os.ReadFile(containerfilePath)
		if err != nil {
			t.Fatalf("Failed to read Containerfile: %v", err)
		}

		if !strings.Contains(string(content), "FROM registry.fedoraproject.org/fedora:40") {
			t.Error("Expected Containerfile to use custom Fedora version 40")
		}

		// Verify custom packages are installed
		if !strings.Contains(string(content), "custom-package") {
			t.Error("Expected Containerfile to contain custom package")
		}
		if !strings.Contains(string(content), "agent-package") {
			t.Error("Expected Containerfile to contain agent package")
		}

		// Verify custom RUN commands
		if !strings.Contains(string(content), "RUN echo 'custom setup'") {
			t.Error("Expected Containerfile to contain custom RUN command")
		}
		if !strings.Contains(string(content), "RUN echo 'agent setup'") {
			t.Error("Expected Containerfile to contain agent RUN command")
		}

		// Verify sudoers contains custom binary
		sudoersPath := filepath.Join(instanceDir, "sudoers")
		sudoersContent, err := os.ReadFile(sudoersPath)
		if err != nil {
			t.Fatalf("Failed to read sudoers: %v", err)
		}

		if !strings.Contains(string(sudoersContent), "/usr/bin/custom") {
			t.Error("Expected sudoers to contain custom binary")
		}
	})

	t.Run("writes agent settings files to build context", func(t *testing.T) {
		t.Parallel()

		instanceDir := t.TempDir()
		p := &podmanRuntime{}

		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{},
			Sudo:        []string{},
			RunCommands: []string{},
		}
		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{},
			TerminalCommand: []string{"claude"},
		}
		settings := map[string][]byte{
			".claude/settings.json": []byte(`{"theme":"dark"}`),
			".gitconfig":            []byte("[user]\n\tname = Agent\n"),
		}

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, settings, nil)
		if err != nil {
			t.Fatalf("createContainerfile() failed: %v", err)
		}

		// Verify agent-settings directory was created
		settingsDir := filepath.Join(instanceDir, "agent-settings")
		if _, err := os.Stat(settingsDir); os.IsNotExist(err) {
			t.Error("Expected agent-settings directory to be created")
		}

		// Verify nested file is written correctly
		claudeSettings := filepath.Join(settingsDir, ".claude", "settings.json")
		content, err := os.ReadFile(claudeSettings)
		if err != nil {
			t.Fatalf("Failed to read .claude/settings.json: %v", err)
		}
		if string(content) != `{"theme":"dark"}` {
			t.Errorf("Expected settings content %q, got %q", `{"theme":"dark"}`, string(content))
		}

		// Verify flat file is written correctly
		gitconfig := filepath.Join(settingsDir, ".gitconfig")
		content, err = os.ReadFile(gitconfig)
		if err != nil {
			t.Fatalf("Failed to read .gitconfig: %v", err)
		}
		if string(content) != "[user]\n\tname = Agent\n" {
			t.Errorf("Expected gitconfig content %q, got %q", "[user]\n\tname = Agent\n", string(content))
		}

		// Verify Containerfile contains the COPY instruction for agent settings
		containerfilePath := filepath.Join(instanceDir, "Containerfile")
		containerfileContent, err := os.ReadFile(containerfilePath)
		if err != nil {
			t.Fatalf("Failed to read Containerfile: %v", err)
		}
		if !strings.Contains(string(containerfileContent), "COPY --chown=agent:agent agent-settings/. /home/agent/") {
			t.Error("Expected Containerfile to contain COPY instruction for agent settings")
		}
	})

	t.Run("no agent-settings dir or COPY when settings is nil", func(t *testing.T) {
		t.Parallel()

		instanceDir := t.TempDir()
		p := &podmanRuntime{}

		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{},
			Sudo:        []string{},
			RunCommands: []string{},
		}
		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{},
			TerminalCommand: []string{"claude"},
		}

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil, nil)
		if err != nil {
			t.Fatalf("createContainerfile() failed: %v", err)
		}

		// Verify agent-settings directory was NOT created
		settingsDir := filepath.Join(instanceDir, "agent-settings")
		if _, err := os.Stat(settingsDir); !os.IsNotExist(err) {
			t.Error("Expected agent-settings directory to NOT be created when settings is nil")
		}

		// Verify Containerfile does not contain agent-settings COPY
		containerfilePath := filepath.Join(instanceDir, "Containerfile")
		containerfileContent, err := os.ReadFile(containerfilePath)
		if err != nil {
			t.Fatalf("Failed to read Containerfile: %v", err)
		}
		if strings.Contains(string(containerfileContent), "agent-settings") {
			t.Error("Expected Containerfile to NOT contain agent-settings when settings is nil")
		}
	})
}

func TestBuildContainerArgs(t *testing.T) {
	t.Parallel()

	t.Run("basic args without config", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		// Use t.TempDir() for cross-platform path handling
		sourcePath := t.TempDir()
		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		// Verify basic structure (includes --pod for single-pod architecture)
		expectedArgs := []string{
			"create",
			"--pod", "test-workspace",
			"--name", "test-workspace",
			"--device", "/dev/fuse",
			"-v", fmt.Sprintf("%s:/workspace/sources:Z", sourcePath),
			"-w", "/workspace/sources",
			"kdn-test-workspace",
			"sleep", "infinity",
		}

		if len(args) != len(expectedArgs) {
			t.Fatalf("Expected %d args, got %d\nExpected: %v\nGot: %v", len(expectedArgs), len(args), expectedArgs, args)
		}

		for i, expected := range expectedArgs {
			if args[i] != expected {
				t.Errorf("Arg %d: expected %q, got %q", i, expected, args[i])
			}
		}
	})

	t.Run("with environment variables", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		debugValue := "true"
		apiKeySecret := "github-token"
		emptyValue := ""

		envVars := []workspace.EnvironmentVariable{
			{Name: "DEBUG", Value: &debugValue},
			{Name: "API_KEY", Secret: &apiKeySecret},
			{Name: "EMPTY", Value: &emptyValue},
		}

		// Use t.TempDir() for cross-platform path handling
		sourcePath := t.TempDir()
		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Environment: &envVars,
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		// Check that environment variables are included
		argsStr := strings.Join(args, " ")

		if !strings.Contains(argsStr, "-e DEBUG=true") {
			t.Error("Expected DEBUG=true environment variable")
		}
		// Secrets should use --secret flag with type=env,target=ENV_VAR format
		if !strings.Contains(argsStr, "--secret github-token,type=env,target=API_KEY") {
			t.Error("Expected --secret github-token,type=env,target=API_KEY")
		}
		if !strings.Contains(argsStr, "-e EMPTY=") {
			t.Error("Expected EMPTY= environment variable")
		}
	})

	t.Run("with dependency mounts", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		// Create a real temp directory structure for cross-platform testing
		tempDir := t.TempDir()
		projectsDir := filepath.Join(tempDir, "projects")
		currentDir := filepath.Join(projectsDir, "current")
		mainDir := filepath.Join(projectsDir, "main")
		sharedDir := filepath.Join(projectsDir, "shared")

		os.MkdirAll(currentDir, 0755)
		os.MkdirAll(mainDir, 0755)
		os.MkdirAll(sharedDir, 0755)

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: currentDir,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Mounts: &[]workspace.Mount{
					{Host: "$SOURCES/../main", Target: "$SOURCES/../main"},
					{Host: "$SOURCES/../shared", Target: "$SOURCES/../shared"},
				},
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		argsStr := strings.Join(args, " ")

		// Build expected mount strings with cross-platform paths
		expectedMainMount := fmt.Sprintf("%s:/workspace/main:Z", mainDir)
		expectedSharedMount := fmt.Sprintf("%s:/workspace/shared:Z", sharedDir)

		if !strings.Contains(argsStr, expectedMainMount) {
			t.Errorf("Expected main dependency mount %q, got: %s", expectedMainMount, argsStr)
		}
		if !strings.Contains(argsStr, expectedSharedMount) {
			t.Errorf("Expected shared dependency mount %q, got: %s", expectedSharedMount, argsStr)
		}
	})

	t.Run("with config mounts", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: t.TempDir(),
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Mounts: &[]workspace.Mount{
					{Host: "$HOME/.claude", Target: "$HOME/.claude"},
					{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig"},
				},
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		// Get user home directory for verification
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}

		// Check that configs are mounted
		argsStr := strings.Join(args, " ")

		expectedClaude := filepath.Join(homeDir, ".claude") + ":/home/agent/.claude:Z"
		expectedGitconfig := filepath.Join(homeDir, ".gitconfig") + ":/home/agent/.gitconfig:Z"

		if !strings.Contains(argsStr, expectedClaude) {
			t.Errorf("Expected .claude config mount: %s", expectedClaude)
		}
		if !strings.Contains(argsStr, expectedGitconfig) {
			t.Errorf("Expected .gitconfig config mount: %s", expectedGitconfig)
		}
	})

	t.Run("with containerConfigArgs env vars and CA cert", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()
		caFile := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(caFile, []byte("cert-data"), 0644); err != nil {
			t.Fatalf("failed to write CA fixture: %v", err)
		}

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}
		imageName := "kdn-test-workspace"

		ccArgs := &containerConfigArgs{
			envVars: map[string]string{
				"HTTP_PROXY":  "http://proxy:8080",
				"HTTPS_PROXY": "https://proxy:8443",
			},
			caFilePath:      caFile,
			caContainerPath: "/etc/ssl/certs/onecli-ca.pem",
		}

		args, err := p.buildContainerArgs(params, imageName, ccArgs)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		argsStr := strings.Join(args, " ")

		// Verify OneCLI proxy env vars are present
		if !strings.Contains(argsStr, "-e HTTP_PROXY=http://proxy:8080") {
			t.Error("Expected HTTP_PROXY env var")
		}
		if !strings.Contains(argsStr, "-e HTTPS_PROXY=https://proxy:8443") {
			t.Error("Expected HTTPS_PROXY env var")
		}

		// Verify NO_PROXY is injected so local addresses bypass the proxy
		if !strings.Contains(argsStr, "-e NO_PROXY=localhost,127.0.0.1,host.containers.internal") {
			t.Errorf("Expected NO_PROXY env var in args: %s", argsStr)
		}
		if !strings.Contains(argsStr, "-e no_proxy=localhost,127.0.0.1,host.containers.internal") {
			t.Errorf("Expected no_proxy env var in args: %s", argsStr)
		}

		// Verify CA cert volume mount
		expectedMount := fmt.Sprintf("-v %s:/etc/ssl/certs/onecli-ca.pem:ro,Z", caFile)
		if !strings.Contains(argsStr, expectedMount) {
			t.Errorf("Expected CA cert mount %q in args: %s", expectedMount, argsStr)
		}
	})

	t.Run("no_proxy not injected when onecli already sets it", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}

		ccArgs := &containerConfigArgs{
			envVars: map[string]string{
				"HTTP_PROXY": "http://proxy:8080",
				"NO_PROXY":   "custom.internal",
				"no_proxy":   "custom.internal",
			},
		}

		args, err := p.buildContainerArgs(params, "kdn-test", ccArgs)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}
		argsStr := strings.Join(args, " ")

		// OneCLI's NO_PROXY must be preserved as-is; we must not inject a second one.
		if !strings.Contains(argsStr, "-e NO_PROXY=custom.internal") {
			t.Errorf("Expected OneCLI NO_PROXY in args: %s", argsStr)
		}
		if strings.Contains(argsStr, "-e NO_PROXY=localhost") || strings.Contains(argsStr, "-e no_proxy=localhost") {
			t.Errorf("Should not inject default NO_PROXY when OneCLI already provides it: %s", argsStr)
		}
	})

	t.Run("no_proxy not injected when workspace config sets it", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()
		noProxyVal := "my-internal-host"

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Environment: &[]workspace.EnvironmentVariable{
					{Name: "NO_PROXY", Value: &noProxyVal},
				},
			},
		}

		ccArgs := &containerConfigArgs{
			envVars: map[string]string{"HTTP_PROXY": "http://proxy:8080"},
		}

		args, err := p.buildContainerArgs(params, "kdn-test", ccArgs)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}
		argsStr := strings.Join(args, " ")

		if !strings.Contains(argsStr, "-e NO_PROXY=my-internal-host") {
			t.Errorf("Expected workspace NO_PROXY in args: %s", argsStr)
		}
		if strings.Contains(argsStr, "-e NO_PROXY=localhost") || strings.Contains(argsStr, "-e no_proxy=localhost") {
			t.Errorf("Should not inject default NO_PROXY when workspace config already sets it: %s", argsStr)
		}
	})

	t.Run("no_proxy not injected when ccArgs is nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}

		args, err := p.buildContainerArgs(params, "kdn-test", nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}
		argsStr := strings.Join(args, " ")

		if strings.Contains(argsStr, "-e NO_PROXY=") || strings.Contains(argsStr, "-e no_proxy=") {
			t.Errorf("Should not inject NO_PROXY when no OneCLI config: %s", argsStr)
		}
	})

	t.Run("onecli env vars override workspace env vars", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()

		proxyValue := "http://user-proxy:9090"
		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Environment: &[]workspace.EnvironmentVariable{
					{Name: "HTTP_PROXY", Value: &proxyValue},
				},
			},
		}
		imageName := "kdn-test-workspace"

		ccArgs := &containerConfigArgs{
			envVars: map[string]string{
				"HTTP_PROXY": "http://onecli-proxy:8080",
			},
		}

		args, err := p.buildContainerArgs(params, imageName, ccArgs)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		// Find the indices of both -e HTTP_PROXY entries
		onecliIdx, wsIdx := -1, -1
		for i, arg := range args {
			if arg == "-e" && i+1 < len(args) {
				if args[i+1] == "HTTP_PROXY=http://onecli-proxy:8080" {
					onecliIdx = i
				}
				if args[i+1] == "HTTP_PROXY=http://user-proxy:9090" {
					wsIdx = i
				}
			}
		}

		if onecliIdx == -1 {
			t.Fatal("OneCLI HTTP_PROXY not found in args")
		}
		if wsIdx == -1 {
			t.Fatal("Workspace HTTP_PROXY not found in args")
		}
		// OneCLI env var should come after workspace env var (later wins in podman)
		if onecliIdx <= wsIdx {
			t.Errorf("OneCLI HTTP_PROXY (index %d) should come after workspace HTTP_PROXY (index %d) for precedence", onecliIdx, wsIdx)
		}
	})

	t.Run("with all options combined", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		debugValue := "true"
		envVars := []workspace.EnvironmentVariable{
			{Name: "DEBUG", Value: &debugValue},
		}

		// Create a real temp directory structure for cross-platform testing
		tempDir := t.TempDir()
		projectsDir := filepath.Join(tempDir, "projects")
		currentDir := filepath.Join(projectsDir, "current")
		mainDir := filepath.Join(projectsDir, "main")

		os.MkdirAll(currentDir, 0755)
		os.MkdirAll(mainDir, 0755)

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: currentDir,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Environment: &envVars,
				Mounts: &[]workspace.Mount{
					{Host: "$SOURCES/../main", Target: "$SOURCES/../main"},
					{Host: "$HOME/.claude", Target: "$HOME/.claude"},
				},
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		// Verify all components are present
		argsStr := strings.Join(args, " ")

		// Check structure
		if !strings.Contains(argsStr, "create") {
			t.Error("Expected 'create' command")
		}
		if !strings.Contains(argsStr, "--name test-workspace") {
			t.Error("Expected container name")
		}
		if !strings.Contains(argsStr, "-e DEBUG=true") {
			t.Error("Expected environment variable")
		}

		// Build expected mount strings with cross-platform paths
		expectedSourceMount := fmt.Sprintf("%s:/workspace/sources:Z", currentDir)
		expectedMainMount := fmt.Sprintf("%s:/workspace/main:Z", mainDir)

		if !strings.Contains(argsStr, expectedSourceMount) {
			t.Errorf("Expected source mount %q", expectedSourceMount)
		}
		if !strings.Contains(argsStr, expectedMainMount) {
			t.Errorf("Expected dependency mount %q", expectedMainMount)
		}
		if !strings.Contains(argsStr, ":/home/agent/.claude:Z") {
			t.Error("Expected config mount")
		}
		if !strings.Contains(argsStr, "-w /workspace/sources") {
			t.Error("Expected working directory")
		}
		if !strings.Contains(argsStr, imageName) {
			t.Error("Expected image name")
		}
		if !strings.Contains(argsStr, "sleep infinity") {
			t.Error("Expected sleep infinity command")
		}
	})

	t.Run("with secret env vars", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()
		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			SecretEnvVars: map[string]string{
				"GH_TOKEN":     "placeholder",
				"GITHUB_TOKEN": "placeholder",
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "-e GH_TOKEN=placeholder") {
			t.Error("Expected GH_TOKEN=placeholder environment variable")
		}
		if !strings.Contains(argsStr, "-e GITHUB_TOKEN=placeholder") {
			t.Error("Expected GITHUB_TOKEN=placeholder environment variable")
		}
	})

	t.Run("secret env vars skip workspace-defined vars", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		sourcePath := t.TempDir()

		customToken := "my-real-token"
		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Environment: &[]workspace.EnvironmentVariable{
					{Name: "GH_TOKEN", Value: &customToken},
				},
			},
			SecretEnvVars: map[string]string{
				"GH_TOKEN":     "placeholder",
				"GITHUB_TOKEN": "placeholder",
			},
		}
		imageName := "kdn-test-workspace"

		args, err := p.buildContainerArgs(params, imageName, nil)
		if err != nil {
			t.Fatalf("buildContainerArgs() failed: %v", err)
		}

		argsStr := strings.Join(args, " ")

		if strings.Contains(argsStr, "GH_TOKEN=placeholder") {
			t.Error("Secret env var GH_TOKEN should not override workspace-defined value")
		}
		if !strings.Contains(argsStr, "GH_TOKEN=my-real-token") {
			t.Error("Expected workspace GH_TOKEN=my-real-token")
		}
		if !strings.Contains(argsStr, "GITHUB_TOKEN=placeholder") {
			t.Error("Expected GITHUB_TOKEN=placeholder")
		}
	})
}

func TestCreate_StepLogger_Success(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()
	onecliServer := newOnecliTestServer(t)

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return nil
	}
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("container-id-123"), nil
	}

	p := &podmanRuntime{
		system:          &fakeSystem{},
		executor:        fakeExec,
		storageDir:      storageDir,
		config:          &fakeConfig{},
		onecliBaseURLFn: func(_ int) string { return onecliServer.URL },
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	params := runtime.CreateParams{
		Name:       "test-workspace",
		SourcePath: sourcePath,
		Agent:      "test_agent",
	}

	_, err := p.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify Complete was called once (deferred call)
	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// Verify no Fail calls
	if len(fakeLogger.failCalls) != 0 {
		t.Errorf("Expected no Fail() calls, got %d", len(fakeLogger.failCalls))
	}

	// Verify the full step sequence including OneCLI setup (always runs to inject proxy env vars)
	expectedSteps := []stepCall{
		{inProgress: "Creating temporary build directory", completed: "Temporary build directory created"},
		{inProgress: "Generating Containerfile", completed: "Containerfile generated"},
		{inProgress: "Building container image: kdn-test-workspace", completed: "Container image built"},
		{inProgress: "Creating onecli services", completed: "Onecli services created"},
		{inProgress: "Starting postgres", completed: "Postgres started"},
		{inProgress: "Waiting for postgres readiness", completed: "Postgres ready"},
		{inProgress: "Starting OneCLI", completed: "OneCLI started"},
		{inProgress: "Waiting for OneCLI readiness", completed: "OneCLI ready"},
		{inProgress: "Retrieving OneCLI container config", completed: "Container config retrieved"},
		{inProgress: "Stopping OneCLI services", completed: "OneCLI services stopped"},
		{inProgress: "Creating workspace container: test-workspace", completed: "Workspace container created"},
	}

	if len(fakeLogger.startCalls) != len(expectedSteps) {
		t.Fatalf("Expected %d Start() calls, got %d", len(expectedSteps), len(fakeLogger.startCalls))
	}

	for i, expected := range expectedSteps {
		actual := fakeLogger.startCalls[i]
		if actual.inProgress != expected.inProgress {
			t.Errorf("Step %d: expected inProgress %q, got %q", i, expected.inProgress, actual.inProgress)
		}
		if actual.completed != expected.completed {
			t.Errorf("Step %d: expected completed %q, got %q", i, expected.completed, actual.completed)
		}
	}
}

func TestCreate_StepLogger_FailOnCreateInstanceDirectory(t *testing.T) {
	t.Parallel()

	// Use a file as storage dir to cause createInstanceDirectory to fail
	storageDir := t.TempDir()
	notADir := filepath.Join(storageDir, "file")
	os.WriteFile(notADir, []byte("test"), 0644)

	sourcePath := t.TempDir()

	p := &podmanRuntime{
		system:     &fakeSystem{},
		executor:   exec.NewFake(),
		storageDir: notADir, // Will fail when trying to create subdirectory
		config:     &fakeConfig{},
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	params := runtime.CreateParams{
		Name:       "test-workspace",
		SourcePath: sourcePath,
		Agent:      "test_agent",
	}

	_, err := p.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected Create() to fail, got nil")
	}

	// Verify Complete was called once (deferred call)
	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// Verify Start was called for the first step
	if len(fakeLogger.startCalls) != 1 {
		t.Fatalf("Expected 1 Start() call, got %d", len(fakeLogger.startCalls))
	}

	if fakeLogger.startCalls[0].inProgress != "Creating temporary build directory" {
		t.Errorf("Expected first step to be 'Creating temporary build directory', got %q", fakeLogger.startCalls[0].inProgress)
	}

	// Verify Fail was called once with the error
	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}

	if fakeLogger.failCalls[0] == nil {
		t.Error("Expected Fail() to be called with non-nil error")
	}
}

func TestCreate_StepLogger_FailOnCreateContainerfile(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()

	// Create instance directory and a directory named "Containerfile" to cause path collision
	instanceDir := filepath.Join(storageDir, "instances", "test-workspace")
	os.MkdirAll(instanceDir, 0755)
	containerfileDir := filepath.Join(instanceDir, "Containerfile")
	os.Mkdir(containerfileDir, 0755) // This will cause os.WriteFile to fail
	defer os.RemoveAll(containerfileDir)

	p := &podmanRuntime{
		system:     &fakeSystem{},
		executor:   exec.NewFake(),
		storageDir: storageDir,
		config:     &fakeConfig{},
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	params := runtime.CreateParams{
		Name:       "test-workspace",
		SourcePath: sourcePath,
		Agent:      "test_agent",
	}

	_, err := p.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected Create() to fail, got nil")
	}

	// Verify Complete was called once (deferred call)
	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// Verify Start was called twice (create dir, then create containerfile)
	if len(fakeLogger.startCalls) != 2 {
		t.Fatalf("Expected 2 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Creating temporary build directory",
		"Generating Containerfile",
	}

	for i, expected := range expectedSteps {
		if fakeLogger.startCalls[i].inProgress != expected {
			t.Errorf("Step %d: expected %q, got %q", i, expected, fakeLogger.startCalls[i].inProgress)
		}
	}

	// Verify Fail was called once
	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}

	if fakeLogger.failCalls[0] == nil {
		t.Error("Expected Fail() to be called with non-nil error")
	}
}

func TestCreate_StepLogger_FailOnBuildImage(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()

	fakeExec := exec.NewFake()
	// Make Run fail on build command
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		if len(args) > 0 && args[0] == "build" {
			return fmt.Errorf("build failed")
		}
		return nil
	}

	p := &podmanRuntime{
		system:     &fakeSystem{},
		executor:   fakeExec,
		storageDir: storageDir,
		config:     &fakeConfig{},
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	params := runtime.CreateParams{
		Name:       "test-workspace",
		SourcePath: sourcePath,
		Agent:      "test_agent",
	}

	_, err := p.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected Create() to fail, got nil")
	}

	// Verify Complete was called once (deferred call)
	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// Verify Start was called 3 times (create dir, containerfile, build image)
	if len(fakeLogger.startCalls) != 3 {
		t.Fatalf("Expected 3 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Creating temporary build directory",
		"Generating Containerfile",
		"Building container image: kdn-test-workspace",
	}

	for i, expected := range expectedSteps {
		if fakeLogger.startCalls[i].inProgress != expected {
			t.Errorf("Step %d: expected %q, got %q", i, expected, fakeLogger.startCalls[i].inProgress)
		}
	}

	// Verify Fail was called once
	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}

	if fakeLogger.failCalls[0] == nil {
		t.Error("Expected Fail() to be called with non-nil error")
	}
}

func TestCreate_StepLogger_FailOnCreateContainer(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()
	onecliServer := newOnecliTestServer(t)

	fakeExec := exec.NewFake()
	// Make Run succeed for build, but Output fail for create
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return nil
	}
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return nil, fmt.Errorf("create container failed")
		}
		return []byte("output"), nil
	}

	p := &podmanRuntime{
		system:          &fakeSystem{},
		executor:        fakeExec,
		storageDir:      storageDir,
		config:          &fakeConfig{},
		onecliBaseURLFn: func(_ int) string { return onecliServer.URL },
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	params := runtime.CreateParams{
		Name:       "test-workspace",
		SourcePath: sourcePath,
		Agent:      "test_agent",
	}

	_, err := p.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected Create() to fail, got nil")
	}

	// Verify Complete was called once (deferred call)
	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// OneCLI setup always runs; workspace container creation is the last step (which fails)
	expectedSteps := []string{
		"Creating temporary build directory",
		"Generating Containerfile",
		"Building container image: kdn-test-workspace",
		"Creating onecli services",
		"Starting postgres",
		"Waiting for postgres readiness",
		"Starting OneCLI",
		"Waiting for OneCLI readiness",
		"Retrieving OneCLI container config",
		"Stopping OneCLI services",
		"Creating workspace container: test-workspace",
	}

	if len(fakeLogger.startCalls) != len(expectedSteps) {
		t.Fatalf("Expected %d Start() calls, got %d", len(expectedSteps), len(fakeLogger.startCalls))
	}

	for i, expected := range expectedSteps {
		if fakeLogger.startCalls[i].inProgress != expected {
			t.Errorf("Step %d: expected %q, got %q", i, expected, fakeLogger.startCalls[i].inProgress)
		}
	}

	// Verify Fail was called once
	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}

	if fakeLogger.failCalls[0] == nil {
		t.Error("Expected Fail() to be called with non-nil error")
	}
}

func TestCreate_StepLogger_FailOnPrepareFeatures(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return nil
	}
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("container-id-123"), nil
	}

	p := &podmanRuntime{
		system:     &fakeSystem{},
		executor:   fakeExec,
		storageDir: storageDir,
		config:     &fakeConfig{},
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	// Reference a non-existent local feature to cause Download to fail.
	featuresMap := map[string]map[string]interface{}{
		"./nonexistent-feature": {},
	}
	params := runtime.CreateParams{
		Name:               "test-workspace",
		SourcePath:         sourcePath,
		Agent:              "test_agent",
		WorkspaceConfigDir: sourcePath,
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Features: &featuresMap,
		},
	}

	_, err := p.Create(ctx, params)
	if err == nil {
		t.Fatal("Expected Create() to fail, got nil")
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	// Exactly two Start calls: create dir, then download features.
	if len(fakeLogger.startCalls) != 2 {
		t.Fatalf("Expected 2 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Creating temporary build directory",
		"Downloading devcontainer features",
	}
	for i, expected := range expectedSteps {
		if fakeLogger.startCalls[i].inProgress != expected {
			t.Errorf("Step %d: expected %q, got %q", i, expected, fakeLogger.startCalls[i].inProgress)
		}
	}

	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}
	if fakeLogger.failCalls[0] == nil {
		t.Error("Expected Fail() to be called with non-nil error")
	}
}

func TestCreate_StepLogger_Success_WithFeatures(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	sourcePath := t.TempDir()
	onecliServer := newOnecliTestServer(t)

	// Set up a minimal local feature.
	configDir := t.TempDir()
	featureDir := filepath.Join(configDir, "my-feature")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("failed to create feature dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(featureDir, "devcontainer-feature.json"),
		[]byte(`{"id":"my-feature","version":"1.0.0"}`),
		0644,
	); err != nil {
		t.Fatalf("failed to write devcontainer-feature.json: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(featureDir, "install.sh"),
		[]byte("#!/bin/bash\necho installed"),
		0755,
	); err != nil {
		t.Fatalf("failed to write install.sh: %v", err)
	}

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return nil
	}
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("container-id-123"), nil
	}

	p := &podmanRuntime{
		system:          &fakeSystem{},
		executor:        fakeExec,
		storageDir:      storageDir,
		config:          &fakeConfig{},
		onecliBaseURLFn: func(_ int) string { return onecliServer.URL },
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	featuresMap := map[string]map[string]interface{}{
		"./my-feature": {},
	}
	params := runtime.CreateParams{
		Name:               "test-workspace",
		SourcePath:         sourcePath,
		Agent:              "test_agent",
		WorkspaceConfigDir: configDir,
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Features: &featuresMap,
		},
	}

	_, err := p.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}
	if len(fakeLogger.failCalls) != 0 {
		t.Errorf("Expected no Fail() calls, got %d", len(fakeLogger.failCalls))
	}

	expectedSteps := []stepCall{
		{inProgress: "Creating temporary build directory", completed: "Temporary build directory created"},
		{inProgress: "Downloading devcontainer features", completed: "Devcontainer features downloaded"},
		{inProgress: "Generating Containerfile", completed: "Containerfile generated"},
		{inProgress: "Building container image: kdn-test-workspace", completed: "Container image built"},
		{inProgress: "Creating onecli services", completed: "Onecli services created"},
		{inProgress: "Starting postgres", completed: "Postgres started"},
		{inProgress: "Waiting for postgres readiness", completed: "Postgres ready"},
		{inProgress: "Starting OneCLI", completed: "OneCLI started"},
		{inProgress: "Waiting for OneCLI readiness", completed: "OneCLI ready"},
		{inProgress: "Retrieving OneCLI container config", completed: "Container config retrieved"},
		{inProgress: "Stopping OneCLI services", completed: "OneCLI services stopped"},
		{inProgress: "Creating workspace container: test-workspace", completed: "Workspace container created"},
	}

	if len(fakeLogger.startCalls) != len(expectedSteps) {
		t.Fatalf("Expected %d Start() calls, got %d", len(expectedSteps), len(fakeLogger.startCalls))
	}
	for i, expected := range expectedSteps {
		actual := fakeLogger.startCalls[i]
		if actual.inProgress != expected.inProgress {
			t.Errorf("Step %d: expected inProgress %q, got %q", i, expected.inProgress, actual.inProgress)
		}
		if actual.completed != expected.completed {
			t.Errorf("Step %d: expected completed %q, got %q", i, expected.completed, actual.completed)
		}
	}
}

func TestCreate_CleansUpInstanceDirectory(t *testing.T) {
	t.Parallel()

	t.Run("removes instance directory after successful create", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcePath := t.TempDir()
		onecliServer := newOnecliTestServer(t)

		// Create a fake executor that simulates successful operations
		fakeExec := &fakeExecutor{
			runErr:    nil,
			outputErr: nil,
			output:    []byte("container123"),
		}

		p := &podmanRuntime{
			system:          &fakeSystem{},
			executor:        fakeExec,
			storageDir:      storageDir,
			config:          &fakeConfig{},
			onecliBaseURLFn: func(_ int) string { return onecliServer.URL },
		}

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}

		// Before Create, verify instances directory doesn't exist yet
		instancesDir := filepath.Join(storageDir, "instances")

		// Call Create
		_, err := p.Create(context.Background(), params)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// After Create, verify the instance directory was cleaned up
		// On Windows, file locks may delay cleanup, so retry with a timeout
		instanceDir := filepath.Join(instancesDir, "test-workspace")
		assertDirectoryRemoved(t, instanceDir)
	})

	t.Run("removes instance directory even on build failure", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcePath := t.TempDir()

		// Create a fake executor that simulates build failure
		fakeExec := &fakeExecutor{
			runErr:    fmt.Errorf("image build failed"),
			outputErr: nil,
			output:    nil,
		}

		p := &podmanRuntime{
			system:     &fakeSystem{},
			executor:   fakeExec,
			storageDir: storageDir,
			config:     &fakeConfig{},
		}

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
		}

		instancesDir := filepath.Join(storageDir, "instances")

		// Call Create (should fail on build)
		_, err := p.Create(context.Background(), params)
		if err == nil {
			t.Fatal("Expected Create() to fail, but it succeeded")
		}

		// Even after failure, verify the instance directory was cleaned up
		// On Windows, file locks may delay cleanup, so retry with a timeout
		instanceDir := filepath.Join(instancesDir, "test-workspace")
		assertDirectoryRemoved(t, instanceDir)
	})
}

// fakeExecutor is a test double for the exec.Executor interface
// newOnecliTestServer starts an httptest server that handles the OneCLI endpoints
// invoked during Create() (health, api-key, container-config). Use together with
// podmanRuntime.onecliBaseURLFn to avoid dialling a real localhost port in tests.
func newOnecliTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/container-config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"env":                        map[string]string{},
				"caCertificate":              "",
				"caCertificateContainerPath": "",
			})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

type fakeExecutor struct {
	runErr    error
	outputErr error
	output    []byte
}

func (f *fakeExecutor) Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	return f.runErr
}

func (f *fakeExecutor) Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error) {
	if f.outputErr != nil {
		return nil, f.outputErr
	}
	return f.output, nil
}

func (f *fakeExecutor) RunInteractive(ctx context.Context, args ...string) error {
	return f.runErr
}

func TestPrepareFeatures(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when WorkspaceConfig is nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		instanceDir := t.TempDir()
		params := runtime.CreateParams{}

		infos, err := p.prepareFeatures(context.Background(), instanceDir, params)
		if err != nil {
			t.Fatalf("prepareFeatures() returned unexpected error: %v", err)
		}
		if infos != nil {
			t.Errorf("Expected nil infos, got %v", infos)
		}
	})

	t.Run("returns nil when Features map is nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		instanceDir := t.TempDir()
		params := runtime.CreateParams{
			WorkspaceConfig: &workspace.WorkspaceConfiguration{},
		}

		infos, err := p.prepareFeatures(context.Background(), instanceDir, params)
		if err != nil {
			t.Fatalf("prepareFeatures() returned unexpected error: %v", err)
		}
		if infos != nil {
			t.Errorf("Expected nil infos, got %v", infos)
		}
	})

	t.Run("returns nil when Features map is empty", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		instanceDir := t.TempDir()
		empty := map[string]map[string]interface{}{}
		params := runtime.CreateParams{
			WorkspaceConfig: &workspace.WorkspaceConfiguration{Features: &empty},
		}

		infos, err := p.prepareFeatures(context.Background(), instanceDir, params)
		if err != nil {
			t.Fatalf("prepareFeatures() returned unexpected error: %v", err)
		}
		if infos != nil {
			t.Errorf("Expected nil infos, got %v", infos)
		}
	})

	t.Run("downloads local feature and returns install info", func(t *testing.T) {
		t.Parallel()

		// Create a local feature directory with required files.
		configDir := t.TempDir()
		featureDir := filepath.Join(configDir, "my-feature")
		if err := os.MkdirAll(featureDir, 0755); err != nil {
			t.Fatalf("failed to create feature dir: %v", err)
		}

		featureJSON := `{
			"id": "my-feature",
			"version": "1.0.0",
			"options": {
				"version": {"type": "string", "default": "latest"}
			},
			"containerEnv": {
				"MY_FEATURE_HOME": "/opt/my-feature"
			}
		}`
		if err := os.WriteFile(filepath.Join(featureDir, "devcontainer-feature.json"), []byte(featureJSON), 0644); err != nil {
			t.Fatalf("failed to write devcontainer-feature.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(featureDir, "install.sh"), []byte("#!/usr/bin/env bash\necho installed"), 0755); err != nil {
			t.Fatalf("failed to write install.sh: %v", err)
		}

		instanceDir := t.TempDir()
		featureID := "./my-feature"
		featuresMap := map[string]map[string]interface{}{
			featureID: {"version": "1.21"},
		}
		params := runtime.CreateParams{
			WorkspaceConfigDir: configDir,
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Features: &featuresMap,
			},
		}

		p := &podmanRuntime{}
		infos, err := p.prepareFeatures(context.Background(), instanceDir, params)
		if err != nil {
			t.Fatalf("prepareFeatures() failed: %v", err)
		}

		if len(infos) != 1 {
			t.Fatalf("Expected 1 featureInstallInfo, got %d", len(infos))
		}

		info := infos[0]
		if info.dirName == "" {
			t.Error("Expected non-empty dirName")
		}

		// Verify the feature directory was created in the build context.
		featureBuildDir := filepath.Join(instanceDir, "features", info.dirName)
		if _, err := os.Stat(featureBuildDir); os.IsNotExist(err) {
			t.Errorf("Expected feature build directory %s to exist", featureBuildDir)
		}

		// Verify install.sh was copied.
		installSh := filepath.Join(featureBuildDir, "install.sh")
		if _, err := os.Stat(installSh); os.IsNotExist(err) {
			t.Error("Expected install.sh to be copied into build context")
		}

		// Verify user-supplied option was merged.
		if info.options["VERSION"] != "1.21" {
			t.Errorf("Expected VERSION=1.21, got %q", info.options["VERSION"])
		}

		// Verify containerEnv was returned.
		if info.envVars["MY_FEATURE_HOME"] != "/opt/my-feature" {
			t.Errorf("Expected MY_FEATURE_HOME=/opt/my-feature, got %q", info.envVars["MY_FEATURE_HOME"])
		}
	})
}

// fakeCredentialForDetect is a configurable test double for credential.Credential.
// Only Detect and FakeFile behaviour can be controlled; Configure is a no-op.
type fakeCredentialForDetect struct {
	name        string
	detectPath  string // returned as hostFilePath by Detect; "" means not detected
	intercepted *workspace.Mount
	fakeContent []byte
	fakeFileErr error
}

var _ credential.Credential = (*fakeCredentialForDetect)(nil)

func (f *fakeCredentialForDetect) Name() string              { return f.name }
func (f *fakeCredentialForDetect) ContainerFilePath() string { return "/fake/" + f.name }
func (f *fakeCredentialForDetect) Detect(_ []workspace.Mount, _ string) (string, *workspace.Mount) {
	return f.detectPath, f.intercepted
}
func (f *fakeCredentialForDetect) FakeFile(_ string) ([]byte, error) {
	return f.fakeContent, f.fakeFileErr
}
func (f *fakeCredentialForDetect) Configure(_ context.Context, _ onecli.Client, _ string) error {
	return nil
}
func (f *fakeCredentialForDetect) HostPatterns(_ string) []string { return nil }

func TestCreate_WithActiveCredentials(t *testing.T) {
	t.Parallel()

	t.Run("fake credential file mounted and intercepted mount suppressed", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcePath := t.TempDir()
		onecliServer := newOnecliTestServer(t)

		// Mount the gcloud directory; the credential will intercept it.
		interceptedMount := workspace.Mount{
			Host:   "$HOME/.config/gcloud",
			Target: "$HOME/.config/gcloud",
		}
		mounts := []workspace.Mount{interceptedMount}

		// Create a real credential file so os.Stat in detectCredentials succeeds.
		realCredFile := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realCredFile, []byte(`{"type":"authorized_user"}`), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}

		cred := &fakeCredentialForDetect{
			name:        "mycred",
			detectPath:  realCredFile,
			intercepted: &mounts[0],
			fakeContent: []byte(`{"fake":true}`),
		}
		reg := credential.NewRegistry()
		_ = reg.Register(cred)

		fakeExec := exec.NewFake()
		fakeExec.RunFunc = func(_ context.Context, args ...string) error { return nil }
		fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
			return []byte("container-id-123"), nil
		}

		p := &podmanRuntime{
			system:             &fakeSystem{},
			executor:           fakeExec,
			storageDir:         storageDir,
			config:             &fakeConfig{},
			onecliBaseURLFn:    func(_ int) string { return onecliServer.URL },
			credentialRegistry: reg,
		}

		params := runtime.CreateParams{
			Name:       "test-workspace",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Mounts: &mounts,
			},
		}

		ctx := steplogger.WithLogger(context.Background(), &fakeStepLogger{})
		_, err := p.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Find the podman "create" call to inspect the container args.
		var createArgs []string
		for _, call := range fakeExec.OutputCalls {
			if len(call) > 0 && call[0] == "create" {
				createArgs = call
				break
			}
		}
		if createArgs == nil {
			t.Fatal("Expected 'create' command to be called")
		}
		argsStr := strings.Join(createArgs, " ")

		// Fake credential file must be mounted at the credential's container path.
		fakePath := filepath.Join(storageDir, "credentials", "test-workspace", "mycred", "credential")
		expectedCredMount := fmt.Sprintf("-v %s:/fake/mycred:ro,Z", fakePath)
		if !strings.Contains(argsStr, expectedCredMount) {
			t.Errorf("Expected credential mount %q in args:\n%s", expectedCredMount, argsStr)
		}

		// The intercepted mount's container path must NOT appear (mount was suppressed).
		if strings.Contains(argsStr, "/home/agent/.config/gcloud") {
			t.Errorf("Intercepted mount container path should be absent from args:\n%s", argsStr)
		}
	})

	t.Run("ccArgs initialised when OneCLI returns no container config", func(t *testing.T) {
		t.Parallel()

		// When the OneCLI server returns an empty container config (no env, no CA cert),
		// containerConfig is non-nil but ccArgs starts with only envVars set (to an
		// empty map). The credential block must still populate credMounts correctly.
		storageDir := t.TempDir()
		sourcePath := t.TempDir()
		onecliServer := newOnecliTestServer(t)

		// Create a real credential file so os.Stat in detectCredentials succeeds.
		realCredFile2 := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realCredFile2, []byte(`{"type":"authorized_user"}`), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}

		mounts := []workspace.Mount{{Host: "/host/cred", Target: "/host/cred"}}
		cred := &fakeCredentialForDetect{
			name:        "cred2",
			detectPath:  realCredFile2,
			intercepted: &mounts[0],
			fakeContent: []byte("placeholder"),
		}
		reg := credential.NewRegistry()
		_ = reg.Register(cred)

		fakeExec := exec.NewFake()
		fakeExec.RunFunc = func(_ context.Context, args ...string) error { return nil }
		fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
			return []byte("container-id-456"), nil
		}

		p := &podmanRuntime{
			system:             &fakeSystem{},
			executor:           fakeExec,
			storageDir:         storageDir,
			config:             &fakeConfig{},
			onecliBaseURLFn:    func(_ int) string { return onecliServer.URL },
			credentialRegistry: reg,
		}

		params := runtime.CreateParams{
			Name:       "ws2",
			SourcePath: sourcePath,
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Mounts: &mounts,
			},
		}

		ctx := steplogger.WithLogger(context.Background(), &fakeStepLogger{})
		_, err := p.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		var createArgs []string
		for _, call := range fakeExec.OutputCalls {
			if len(call) > 0 && call[0] == "create" {
				createArgs = call
				break
			}
		}
		if createArgs == nil {
			t.Fatal("Expected 'create' command to be called")
		}

		fakePath := filepath.Join(storageDir, "credentials", "ws2", "cred2", "credential")
		expectedMount := fmt.Sprintf("-v %s:/fake/cred2:ro,Z", fakePath)
		if !strings.Contains(strings.Join(createArgs, " "), expectedMount) {
			t.Errorf("Expected credential mount %q in args: %v", expectedMount, createArgs)
		}
	})
}

func TestDetectCredentials(t *testing.T) {
	t.Parallel()

	emptyMounts := []workspace.Mount{}
	configWithMounts := func(mounts []workspace.Mount) *workspace.WorkspaceConfiguration {
		return &workspace.WorkspaceConfiguration{Mounts: &mounts}
	}

	t.Run("nil registry returns nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("nil WorkspaceConfig returns nil", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		p := &podmanRuntime{credentialRegistry: reg}
		result := p.detectCredentials(runtime.CreateParams{Name: "ws"}, "/home/user")
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("nil Mounts returns nil", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		p := &podmanRuntime{credentialRegistry: reg}
		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{},
		}, "/home/user")
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("credential not detected is skipped", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{name: "unmatched", detectPath: ""})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: t.TempDir()}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if len(result) != 0 {
			t.Errorf("Expected empty result for undetected credential, got %v", result)
		}
	})

	t.Run("host file missing on host skips credential", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "gcloud",
			detectPath:  "/nonexistent/path/adc.json",
			fakeContent: []byte(`{"fake":true}`),
		})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: t.TempDir()}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if len(result) != 0 {
			t.Errorf("Expected empty result when host file is missing, got %v", result)
		}
	})

	t.Run("FakeFile error skips credential", func(t *testing.T) {
		t.Parallel()

		// Create a real file so os.Stat succeeds; FakeFile error is still exercised.
		realFile := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realFile, []byte("{}"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "broken",
			detectPath:  realFile,
			fakeFileErr: errors.New("cannot generate placeholder"),
		})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: t.TempDir()}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if len(result) != 0 {
			t.Errorf("Expected empty result when FakeFile fails, got %v", result)
		}
	})

	t.Run("MkdirAll error skips credential", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		// Block MkdirAll by placing a regular file where the credentials dir would be.
		if err := os.WriteFile(filepath.Join(storageDir, "credentials"), []byte("not a dir"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		// Create a real file so os.Stat succeeds; MkdirAll error is still exercised.
		realFile := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realFile, []byte("{}"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "test",
			detectPath:  realFile,
			fakeContent: []byte("fake"),
		})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if len(result) != 0 {
			t.Errorf("Expected empty result when MkdirAll fails, got %v", result)
		}
	})

	t.Run("WriteFile error skips credential", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		// Pre-create the credential dir but make the "credential" leaf a directory
		// so os.WriteFile cannot overwrite it.
		credDir := filepath.Join(storageDir, "credentials", "ws", "test")
		if err := os.MkdirAll(filepath.Join(credDir, "credential"), 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		// Create a real file so os.Stat succeeds; WriteFile error is still exercised.
		realFile := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realFile, []byte("{}"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "test",
			detectPath:  realFile,
			fakeContent: []byte("fake"),
		})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")
		if len(result) != 0 {
			t.Errorf("Expected empty result when WriteFile fails, got %v", result)
		}
	})

	t.Run("successful detection returns active credential", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		interceptedMount := &workspace.Mount{Host: "/real/path", Target: "/container/cred"}

		// Create a real credential file so os.Stat succeeds.
		realCredFile := filepath.Join(t.TempDir(), "adc.json")
		if err := os.WriteFile(realCredFile, []byte(`{"type":"authorized_user"}`), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}

		reg := credential.NewRegistry()
		cred := &fakeCredentialForDetect{
			name:        "mycred",
			detectPath:  realCredFile,
			intercepted: interceptedMount,
			fakeContent: []byte(`{"token":"placeholder"}`),
		}
		_ = reg.Register(cred)
		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}

		mounts := []workspace.Mount{{Host: "/real/path", Target: "/container/cred"}}
		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(mounts),
		}, "/home/user")

		if len(result) != 1 {
			t.Fatalf("Expected 1 active credential, got %d", len(result))
		}
		if result[0].hostPath != realCredFile {
			t.Errorf("hostPath = %q, want %q", result[0].hostPath, realCredFile)
		}
		if result[0].intercepted != interceptedMount {
			t.Errorf("intercepted mount not set correctly")
		}

		fakePath := filepath.Join(storageDir, "credentials", "ws", "mycred", "credential")
		content, err := os.ReadFile(fakePath)
		if err != nil {
			t.Fatalf("Expected fake credential file at %s: %v", fakePath, err)
		}
		if string(content) != `{"token":"placeholder"}` {
			t.Errorf("fake file content = %q, want %q", string(content), `{"token":"placeholder"}`)
		}
	})

	t.Run("only detected credentials are returned", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// Create a real file for the credential that should be detected.
		realCredFile := filepath.Join(t.TempDir(), "cred.json")
		if err := os.WriteFile(realCredFile, []byte(`{}`), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{name: "missing", detectPath: ""})
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "present",
			detectPath:  realCredFile,
			fakeContent: []byte("ok"),
		})
		_ = reg.Register(&fakeCredentialForDetect{name: "also-missing", detectPath: ""})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}

		result := p.detectCredentials(runtime.CreateParams{
			Name:            "ws",
			WorkspaceConfig: configWithMounts(emptyMounts),
		}, "/home/user")

		if len(result) != 1 {
			t.Fatalf("Expected 1 active credential, got %d", len(result))
		}
		if result[0].cred.Name() != "present" {
			t.Errorf("Expected 'present' credential, got %q", result[0].cred.Name())
		}
	})
}

// assertDirectoryRemoved checks that a directory has been removed.
// On Windows, file locks may delay cleanup, so this retries with a timeout.
func assertDirectoryRemoved(t *testing.T, dir string) {
	t.Helper()

	// Retry for up to 1 second with 50ms intervals (Windows file lock workaround)
	maxAttempts := 20
	interval := 50 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			// Directory successfully removed
			return
		}

		if attempt < maxAttempts-1 {
			time.Sleep(interval)
		}
	}

	// Final check after all retries
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Expected instance directory to be removed, but it still exists: %s", dir)

		// List contents for debugging
		if err == nil {
			entries, _ := os.ReadDir(dir)
			t.Logf("Instance directory contents: %v", entries)
		}
	}
}

func TestRenderPodYAML_Ports(t *testing.T) {
	t.Parallel()

	t.Run("renders port declarations in pod YAML when forwards are set", func(t *testing.T) {
		t.Parallel()

		data := podTemplateData{
			Name:               "port-workspace",
			OnecliWebPort:      11000,
			OnecliVersion:      "1.17",
			AgentUID:           1000,
			BaseImageRegistry:  "registry.example.com/base",
			BaseImageVersion:   "latest",
			SourcePath:         "/workspace/sources",
			ApprovalHandlerDir: "/tmp/approval",
			Forwards: []api.WorkspaceForward{
				{Bind: "127.0.0.1", Port: 54321, Target: 8080},
				{Bind: "127.0.0.1", Port: 54322, Target: 3000},
			},
		}

		rendered, err := renderPodYAML(data)
		if err != nil {
			t.Fatalf("renderPodYAML() failed: %v", err)
		}

		yaml := string(rendered)
		if !strings.Contains(yaml, "containerPort: 8080") {
			t.Errorf("Expected containerPort 8080 in YAML:\n%s", yaml)
		}
		if !strings.Contains(yaml, "hostPort: 54321") {
			t.Errorf("Expected hostPort 54321 in YAML:\n%s", yaml)
		}
		if !strings.Contains(yaml, "containerPort: 3000") {
			t.Errorf("Expected containerPort 3000 in YAML:\n%s", yaml)
		}
		if !strings.Contains(yaml, "hostPort: 54322") {
			t.Errorf("Expected hostPort 54322 in YAML:\n%s", yaml)
		}
		if !strings.Contains(yaml, `hostIP: "127.0.0.1"`) {
			t.Errorf("Expected hostIP 127.0.0.1 in YAML:\n%s", yaml)
		}
	})

	t.Run("omits ports section when no forwards configured", func(t *testing.T) {
		t.Parallel()

		data := podTemplateData{
			Name:               "no-port-workspace",
			OnecliWebPort:      11001,
			OnecliVersion:      "1.17",
			AgentUID:           1000,
			BaseImageRegistry:  "registry.example.com/base",
			BaseImageVersion:   "latest",
			SourcePath:         "/workspace/sources",
			ApprovalHandlerDir: "/tmp/approval",
		}

		rendered, err := renderPodYAML(data)
		if err != nil {
			t.Fatalf("renderPodYAML() failed: %v", err)
		}

		yaml := string(rendered)
		// The only ports section should be for OneCLI (containerPort: 10254), not user ports
		if strings.Contains(yaml, "containerPort: 8080") {
			t.Errorf("Expected no user containerPort in YAML:\n%s", yaml)
		}
	})
}

func TestBuildForwards(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no workspace config", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		params := runtime.CreateParams{}

		forwards, err := p.buildForwards(params)
		if err != nil {
			t.Fatalf("buildForwards() failed: %v", err)
		}
		if forwards != nil {
			t.Errorf("Expected nil forwards, got %v", forwards)
		}
	})

	t.Run("returns nil when ports is nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		params := runtime.CreateParams{
			WorkspaceConfig: &workspace.WorkspaceConfiguration{},
		}

		forwards, err := p.buildForwards(params)
		if err != nil {
			t.Fatalf("buildForwards() failed: %v", err)
		}
		if forwards != nil {
			t.Errorf("Expected nil forwards, got %v", forwards)
		}
	})

	t.Run("allocates host ports for each container port", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}
		ports := []int{8080, 3000}
		params := runtime.CreateParams{
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Ports: &ports,
			},
		}

		forwards, err := p.buildForwards(params)
		if err != nil {
			t.Fatalf("buildForwards() failed: %v", err)
		}
		if len(forwards) != 2 {
			t.Fatalf("Expected 2 forwards, got %d", len(forwards))
		}
		for i, fwd := range forwards {
			if fwd.Bind != "127.0.0.1" {
				t.Errorf("Forward %d: expected Bind '127.0.0.1', got '%s'", i, fwd.Bind)
			}
			if fwd.Target != ports[i] {
				t.Errorf("Forward %d: expected Target %d, got %d", i, ports[i], fwd.Target)
			}
			if fwd.Port <= 0 || fwd.Port > 65535 {
				t.Errorf("Forward %d: Port %d is out of valid range", i, fwd.Port)
			}
		}
	})
}

func TestCreate_ForwardsInRuntimeInfo(t *testing.T) {
	t.Parallel()

	newRuntime := func(t *testing.T) (*podmanRuntime, context.Context) {
		t.Helper()
		storageDir := t.TempDir()
		sourcePath := t.TempDir()
		onecliServer := newOnecliTestServer(t)

		fakeExec := exec.NewFake()
		fakeExec.RunFunc = func(ctx context.Context, args ...string) error { return nil }
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte("container-id-123"), nil
		}

		p := &podmanRuntime{
			system:          &fakeSystem{},
			executor:        fakeExec,
			storageDir:      storageDir,
			config:          &fakeConfig{},
			onecliBaseURLFn: func(_ int) string { return onecliServer.URL },
		}
		ctx := steplogger.WithLogger(context.Background(), &fakeStepLogger{})
		_ = sourcePath
		return p, ctx
	}

	t.Run("forwards written to RuntimeInfo when ports configured", func(t *testing.T) {
		t.Parallel()

		p, ctx := newRuntime(t)
		ports := []int{8080, 3000}
		params := runtime.CreateParams{
			Name:       "fwd-workspace",
			SourcePath: t.TempDir(),
			Agent:      "test_agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Ports: &ports,
			},
		}

		info, err := p.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		forwardsJSON, ok := info.Info["forwards"]
		if !ok {
			t.Fatal("Expected 'forwards' key in RuntimeInfo.Info")
		}

		var forwards []api.WorkspaceForward
		if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err != nil {
			t.Fatalf("Failed to unmarshal forwards JSON: %v", err)
		}
		if len(forwards) != 2 {
			t.Fatalf("Expected 2 forwards, got %d", len(forwards))
		}
		targets := map[int]bool{}
		for _, fwd := range forwards {
			targets[fwd.Target] = true
			if fwd.Bind != "127.0.0.1" {
				t.Errorf("Expected Bind '127.0.0.1', got '%s'", fwd.Bind)
			}
			if fwd.Port <= 0 || fwd.Port > 65535 {
				t.Errorf("Port %d out of valid range", fwd.Port)
			}
		}
		for _, want := range ports {
			if !targets[want] {
				t.Errorf("Expected target port %d in forwards, got %v", want, forwards)
			}
		}
	})

	t.Run("forwards absent from RuntimeInfo when no ports configured", func(t *testing.T) {
		t.Parallel()

		p, ctx := newRuntime(t)
		params := runtime.CreateParams{
			Name:       "no-fwd-workspace",
			SourcePath: t.TempDir(),
			Agent:      "test_agent",
		}

		info, err := p.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		if _, ok := info.Info["forwards"]; ok {
			t.Errorf("Expected no 'forwards' key in RuntimeInfo.Info when no ports configured")
		}
	})
}
