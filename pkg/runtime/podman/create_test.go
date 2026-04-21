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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
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

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil)
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

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil)
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

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, settings)
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

		err := p.createContainerfile(instanceDir, imageConfig, agentConfig, nil)
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

		// Verify CA cert volume mount
		expectedMount := fmt.Sprintf("-v %s:/etc/ssl/certs/onecli-ca.pem:ro,Z", caFile)
		if !strings.Contains(argsStr, expectedMount) {
			t.Errorf("Expected CA cert mount %q in args: %s", expectedMount, argsStr)
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

	// Verify Start was called 5 times with correct messages
	expectedSteps := []stepCall{
		{
			inProgress: "Creating temporary build directory",
			completed:  "Temporary build directory created",
		},
		{
			inProgress: "Generating Containerfile",
			completed:  "Containerfile generated",
		},
		{
			inProgress: "Building container image: kdn-test-workspace",
			completed:  "Container image built",
		},
		{
			inProgress: "Creating onecli services",
			completed:  "Onecli services created",
		},
		{
			inProgress: "Creating workspace container: test-workspace",
			completed:  "Workspace container created",
		},
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

	// Verify Start was called 5 times (all steps through workspace container creation)
	if len(fakeLogger.startCalls) != 5 {
		t.Fatalf("Expected 5 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Creating temporary build directory",
		"Generating Containerfile",
		"Building container image: kdn-test-workspace",
		"Creating onecli services",
		"Creating workspace container: test-workspace",
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

func TestCreate_CleansUpInstanceDirectory(t *testing.T) {
	t.Parallel()

	t.Run("removes instance directory after successful create", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcePath := t.TempDir()

		// Create a fake executor that simulates successful operations
		fakeExec := &fakeExecutor{
			runErr:    nil,
			outputErr: nil,
			output:    []byte("container123"),
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
