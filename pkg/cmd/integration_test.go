//go:build integration

/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	gokeyring "github.com/zalando/go-keyring"
)

// initMu serializes "kdn init" calls so that only one podman build runs at a
// time. Podman's rootless storage driver uses file locks that are unreliable
// under concurrent builds, causing sporadic "exit status 125" failures.
var initMu sync.Mutex

// containerSourcesPath is the mount point for project sources inside the container.
const containerSourcesPath = "/workspace/sources"

// warmupImages tracks images built during cache warmup for cleanup after tests.
var warmupImages []string

func TestMain(m *testing.M) {
	// Use an in-memory keyring so integration tests never touch the real system
	// keychain, which may not be available in CI environments.
	gokeyring.MockInit()
	if podmanAvailable() {
		warmupBuildCache()
	}
	code := m.Run()
	for _, img := range warmupImages {
		_ = exec.Command("podman", "rmi", "-f", img).Run()
	}
	os.Exit(code)
}

func podmanAvailable() bool {
	if _, err := exec.LookPath("podman"); err != nil {
		return false
	}
	out, err := exec.Command("podman", "info", "--format", "{{.Host.OCIRuntime.Name}}").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// warmupBuildCache builds one image per agent type to populate Podman's layer cache.
// Subsequent builds in parallel tests reuse cached layers and complete much faster.
func warmupBuildCache() {
	for _, agent := range []string{"claude", "goose"} {
		storageDir, err := os.MkdirTemp("", "kdn-warmup-*")
		if err != nil {
			continue
		}
		sourcesDir, err := os.MkdirTemp("", "kdn-warmup-src-*")
		if err != nil {
			os.RemoveAll(storageDir)
			continue
		}

		name := fmt.Sprintf("warmup-%s", agent)
		imageName := fmt.Sprintf("kdn-%s", name)

		rootCmd := NewRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{
			"--storage", storageDir,
			"init", sourcesDir,
			"--runtime", "podman",
			"--agent", agent,
			"-n", name,
		})

		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "warmup: build failed for agent %s: %v\n", agent, err)
			os.RemoveAll(storageDir)
			os.RemoveAll(sourcesDir)
			continue
		}

		warmupImages = append(warmupImages, imageName)

		cleanCmd := NewRootCmd()
		cleanCmd.SetOut(new(bytes.Buffer))
		cleanCmd.SetErr(new(bytes.Buffer))
		cleanCmd.SilenceErrors = true
		cleanCmd.SetArgs([]string{"--storage", storageDir, "remove", name, "--force"})
		_ = cleanCmd.Execute()

		os.RemoveAll(storageDir)
		os.RemoveAll(sourcesDir)
	}
}

func skipIfNoPodman(t *testing.T) {
	t.Helper()
	if !podmanAvailable() {
		t.Skip("podman not available, skipping integration test")
	}
}

// integrationInit runs "kdn init" with the given args and returns the parsed workspace and ID.
// It registers a t.Cleanup that force-removes the workspace and its image.
func integrationInit(t *testing.T, storageDir, sourcesDir, name, agent string, extraArgs ...string) (api.Workspace, string) {
	t.Helper()

	args := []string{
		"--storage", storageDir,
		"init", sourcesDir,
		"--runtime", "podman",
		"--agent", agent,
		"-n", name,
		"--output", "json",
		"-v",
	}
	args = append(args, extraArgs...)

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetArgs(args)

	initMu.Lock()
	err := rootCmd.Execute()
	initMu.Unlock()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	var ws api.Workspace
	if err := json.Unmarshal(stdout.Bytes(), &ws); err != nil {
		t.Fatalf("Failed to parse init JSON: %v\nOutput: %s", err, stdout.String())
	}

	if ws.Id == "" {
		t.Fatal("Expected workspace ID in init output")
	}

	imageName := fmt.Sprintf("kdn-%s", name)
	t.Cleanup(func() {
		cleanCmd := NewRootCmd()
		cleanCmd.SetOut(new(bytes.Buffer))
		cleanCmd.SetErr(new(bytes.Buffer))
		cleanCmd.SilenceErrors = true
		cleanCmd.SetArgs([]string{"--storage", storageDir, "remove", ws.Id, "--force"})
		_ = cleanCmd.Execute()
		_ = exec.Command("podman", "rmi", "-f", imageName).Run()
	})

	return ws, ws.Id
}

// integrationInitTextMode runs "kdn init" without JSON output (for --show-logs).
// It registers a t.Cleanup and returns the captured stdout and stderr.
func integrationInitTextMode(t *testing.T, storageDir, sourcesDir, name, agent string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()

	args := []string{
		"--storage", storageDir,
		"init", sourcesDir,
		"--runtime", "podman",
		"--agent", agent,
		"-n", name,
	}
	args = append(args, extraArgs...)

	rootCmd := NewRootCmd()
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	rootCmd.SetOut(stdoutBuf)
	rootCmd.SetErr(stderrBuf)
	rootCmd.SetArgs(args)

	initMu.Lock()
	err := rootCmd.Execute()
	initMu.Unlock()
	if err != nil {
		t.Fatalf("init failed: %v\nStderr: %s", err, stderrBuf.String())
	}

	imageName := fmt.Sprintf("kdn-%s", name)
	t.Cleanup(func() {
		cleanCmd := NewRootCmd()
		cleanCmd.SetOut(new(bytes.Buffer))
		cleanCmd.SetErr(new(bytes.Buffer))
		cleanCmd.SilenceErrors = true
		cleanCmd.SetArgs([]string{"--storage", storageDir, "remove", name, "--force"})
		_ = cleanCmd.Execute()
		_ = exec.Command("podman", "rmi", "-f", imageName).Run()
	})

	return stdoutBuf.String(), stderrBuf.String()
}

// integrationExecCmd creates a NewRootCmd, sets stdout/stderr buffers, runs the given args,
// and returns the stdout buffer content. Fatals on error, including stderr in the message.
func integrationExecCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewRootCmd()
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(stderrBuf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\nStderr: %s", args, err, stderrBuf.String())
	}
	return stdoutBuf.String()
}

// integrationExecCmdExpectError runs a command and expects it to fail.
// Returns the stderr content and error.
func integrationExecCmdExpectError(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	stderrBuf := new(bytes.Buffer)
	cmd.SetErr(stderrBuf)
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected command %v to fail, but it succeeded", args)
	}
	return stderrBuf.String(), err
}

// integrationListWorkspaces returns the parsed workspace list for a storage dir.
func integrationListWorkspaces(t *testing.T, storageDir string) api.WorkspacesList {
	t.Helper()
	out := integrationExecCmd(t, "--storage", storageDir, "list", "--output", "json")
	var result api.WorkspacesList
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("Failed to parse list JSON: %v\nOutput: %s", err, out)
	}
	return result
}

func TestIntegration_CreateStartStopRemove(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	ws, wsID := integrationInit(t, storageDir, sourcesDir, "create-start-stop-remove", "claude")

	if ws.Name != "create-start-stop-remove" {
		t.Errorf("Expected name 'create-start-stop-remove', got %q", ws.Name)
	}

	// list shows stopped
	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 1 {
		t.Fatalf("Expected 1 workspace, got %d", len(listResult.Items))
	}
	if listResult.Items[0].State != "stopped" {
		t.Errorf("Expected state 'stopped', got %q", listResult.Items[0].State)
	}

	// start
	startOut := integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")
	var startResult api.WorkspaceId
	if err := json.Unmarshal([]byte(startOut), &startResult); err != nil {
		t.Fatalf("Failed to parse start JSON: %v", err)
	}
	if startResult.Id != wsID {
		t.Errorf("Expected start to return ID %q, got %q", wsID, startResult.Id)
	}

	// list shows running
	listResult2 := integrationListWorkspaces(t, storageDir)
	if listResult2.Items[0].State != "running" {
		t.Errorf("Expected state 'running', got %q", listResult2.Items[0].State)
	}

	// verify container is visible via podman ps
	psOut, err := exec.Command("podman", "ps", "--filter", "name=create-start-stop-remove", "--format", "{{.Names}}").Output()
	if err != nil {
		t.Fatalf("podman ps failed: %v", err)
	}
	if !strings.Contains(string(psOut), "create-start-stop-remove") {
		t.Errorf("Expected container 'create-start-stop-remove' in podman ps, got: %s", string(psOut))
	}

	// idempotent start
	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"--storage", storageDir, "start", wsID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Expected idempotent start to succeed, got: %v", err)
	}

	// stop
	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")

	// idempotent stop
	cmd2 := NewRootCmd()
	cmd2.SetOut(new(bytes.Buffer))
	cmd2.SetArgs([]string{"--storage", storageDir, "stop", wsID})
	if err := cmd2.Execute(); err != nil {
		t.Errorf("Expected idempotent stop to succeed, got: %v", err)
	}

	// remove
	integrationExecCmd(t, "--storage", storageDir, "remove", wsID, "--output", "json")

	// verify container is gone
	psOut2, _ := exec.Command("podman", "ps", "-a", "--filter", "name=create-start-stop-remove", "--format", "{{.Names}}").Output()
	if strings.Contains(string(psOut2), "create-start-stop-remove") {
		t.Errorf("Expected container removed, but still visible in podman ps -a")
	}

	// list shows empty
	listResult3 := integrationListWorkspaces(t, storageDir)
	if len(listResult3.Items) != 0 {
		t.Errorf("Expected 0 workspaces after removal, got %d", len(listResult3.Items))
	}
}

func TestIntegration_ExecCommandInWorkspace(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	_, wsID := integrationInit(t, storageDir, sourcesDir, "exec-command-in-workspace", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Exec a command inside the running container via podman directly
	// (kdn terminal uses -it which requires a TTY, so we use podman exec without -it)
	out, err := exec.Command("podman", "exec", "exec-command-in-workspace", "echo", "hello-from-container").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "hello-from-container") {
		t.Errorf("Expected 'hello-from-container' in exec output, got: %s", string(out))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_CreateAndAutoStart(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	ws, _ := integrationInit(t, storageDir, sourcesDir, "create-and-auto-start", "claude", "--start")

	if ws.Name != "create-and-auto-start" {
		t.Errorf("Expected name 'create-and-auto-start', got %q", ws.Name)
	}

	// Workspace should already be running (no explicit start needed)
	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 1 {
		t.Fatalf("Expected 1 workspace, got %d", len(listResult.Items))
	}
	if listResult.Items[0].State != "running" {
		t.Errorf("Expected state 'running' after init --start, got %q", listResult.Items[0].State)
	}

	// Verify container is visible via podman ps
	psOut, err := exec.Command("podman", "ps", "--filter", "name=create-and-auto-start", "--format", "{{.Names}}").Output()
	if err != nil {
		t.Fatalf("podman ps failed: %v", err)
	}
	if !strings.Contains(string(psOut), "create-and-auto-start") {
		t.Errorf("Expected container 'create-and-auto-start' in podman ps, got: %s", string(psOut))
	}
}

func TestIntegration_ProjectFilesAccessible(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	markerContent := "integration-test-marker-9f3a7b2e"
	if err := os.WriteFile(filepath.Join(sourcesDir, "marker.txt"), []byte(markerContent), 0644); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	_, wsID := integrationInit(t, storageDir, sourcesDir, "project-files-accessible", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Read the marker file from inside the container
	markerPath := containerSourcesPath + "/marker.txt"
	out, err := exec.Command("podman", "exec", "project-files-accessible", "cat", markerPath).CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec cat failed: %v\nOutput: %s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != markerContent {
		t.Errorf("Expected marker content %q, got %q", markerContent, strings.TrimSpace(string(out)))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_ContainerImageCreated(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	integrationInit(t, storageDir, sourcesDir, "container-image-created", "claude")

	// Verify the image was created with the expected name pattern: kdn-<workspace-name>
	out, err := exec.Command("podman", "images", "--format", "{{.Repository}}", "--filter", "reference=kdn-container-image-created").Output()
	if err != nil {
		t.Fatalf("podman images failed: %v", err)
	}
	if !strings.Contains(string(out), "kdn-container-image-created") {
		t.Errorf("Expected image 'kdn-container-image-created' in podman images, got: %s", string(out))
	}
}

func TestIntegration_ForceRemoveRunningWorkspace(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	_, wsID := integrationInit(t, storageDir, sourcesDir, "force-remove-running", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Confirm workspace is running
	listResult := integrationListWorkspaces(t, storageDir)
	if listResult.Items[0].State != "running" {
		t.Fatalf("Expected state 'running', got %q", listResult.Items[0].State)
	}

	// Remove while running with --force
	integrationExecCmd(t, "--storage", storageDir, "remove", wsID, "--force", "--output", "json")

	// Verify container is gone from podman
	psOut, _ := exec.Command("podman", "ps", "-a", "--filter", "name=force-remove-running", "--format", "{{.Names}}").Output()
	if strings.Contains(string(psOut), "force-remove-running") {
		t.Errorf("Expected container removed after --force, but still visible in podman ps -a")
	}

	// Verify workspace list is empty
	listResult2 := integrationListWorkspaces(t, storageDir)
	if len(listResult2.Items) != 0 {
		t.Errorf("Expected 0 workspaces after force removal, got %d", len(listResult2.Items))
	}
}

func TestIntegration_RemoveRunningWithoutForceFails(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	_, wsID := integrationInit(t, storageDir, sourcesDir, "remove-without-force", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Attempt to remove without --force should fail with a clear error
	_, err := integrationExecCmdExpectError(t, "--storage", storageDir, "remove", wsID)
	if !strings.Contains(err.Error(), "running") && !strings.Contains(err.Error(), "force") {
		t.Errorf("Expected error mentioning 'running' or 'force', got: %v", err)
	}

	// Workspace should still be running
	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 1 {
		t.Fatalf("Expected workspace to still exist, got %d items", len(listResult.Items))
	}
	if listResult.Items[0].State != "running" {
		t.Errorf("Expected workspace to still be running, got %q", listResult.Items[0].State)
	}
}

func TestIntegration_EnvVarsFromConfig(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	// Create workspace configuration with an environment variable
	configDir := filepath.Join(sourcesDir, ".kaiden")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	workspaceJSON := `{"environment": [{"name": "KDN_TEST_VAR", "value": "integration123"}]}`
	if err := os.WriteFile(filepath.Join(configDir, "workspace.json"), []byte(workspaceJSON), 0644); err != nil {
		t.Fatalf("Failed to write workspace.json: %v", err)
	}

	_, wsID := integrationInit(t, storageDir, sourcesDir, "env-vars-from-config", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Verify the environment variable is set inside the container
	out, err := exec.Command("podman", "exec", "env-vars-from-config", "printenv", "KDN_TEST_VAR").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec printenv failed: %v\nOutput: %s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "integration123" {
		t.Errorf("Expected KDN_TEST_VAR='integration123', got %q", strings.TrimSpace(string(out)))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_BuildOutputVisible(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	// --show-logs cannot combine with --output json, so we use the text-mode helper
	stdout, stderr := integrationInitTextMode(t, storageDir, sourcesDir, "build-output-visible", "claude", "--show-logs")

	// Podman build output typically contains "STEP" directives
	combined := stdout + stderr
	if combined == "" {
		t.Error("Expected non-empty output with --show-logs, got empty")
	}
	hasStep := strings.Contains(strings.ToUpper(combined), "STEP")
	hasFrom := strings.Contains(strings.ToUpper(combined), "FROM")
	if !hasStep && !hasFrom {
		t.Errorf("Expected build output containing STEP or FROM with --show-logs, got:\n%s", combined)
	}
}

func TestIntegration_GooseAgentWorkspace(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	ws, wsID := integrationInit(t, storageDir, sourcesDir, "goose-agent-workspace", "goose")

	if ws.Agent != "goose" {
		t.Errorf("Expected agent 'goose', got %q", ws.Agent)
	}

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Verify running via list
	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 1 {
		t.Fatalf("Expected 1 workspace, got %d", len(listResult.Items))
	}
	if listResult.Items[0].State != "running" {
		t.Errorf("Expected state 'running', got %q", listResult.Items[0].State)
	}
	if listResult.Items[0].Agent != "goose" {
		t.Errorf("Expected agent 'goose' in list, got %q", listResult.Items[0].Agent)
	}

	// Verify container is functional
	out, err := exec.Command("podman", "exec", "goose-agent-workspace", "echo", "goose-ok").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "goose-ok") {
		t.Errorf("Expected 'goose-ok' in exec output, got: %s", string(out))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_ManageWorkspaceByName(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	ws, _ := integrationInit(t, storageDir, sourcesDir, "manage-by-name", "claude")
	wsName := ws.Name

	if wsName != "manage-by-name" {
		t.Fatalf("Expected name 'manage-by-name', got %q", wsName)
	}

	// All operations by name instead of ID
	integrationExecCmd(t, "--storage", storageDir, "start", wsName, "--output", "json")

	listResult := integrationListWorkspaces(t, storageDir)
	if listResult.Items[0].State != "running" {
		t.Errorf("Expected state 'running' after start by name, got %q", listResult.Items[0].State)
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsName, "--output", "json")

	listResult2 := integrationListWorkspaces(t, storageDir)
	if listResult2.Items[0].State != "stopped" {
		t.Errorf("Expected state 'stopped' after stop by name, got %q", listResult2.Items[0].State)
	}

	integrationExecCmd(t, "--storage", storageDir, "remove", wsName, "--output", "json")

	listResult3 := integrationListWorkspaces(t, storageDir)
	if len(listResult3.Items) != 0 {
		t.Errorf("Expected 0 workspaces after remove by name, got %d", len(listResult3.Items))
	}
}

func TestIntegration_AgentWritesAppearOnHost(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	// Make sources dir world-writable so the container's agent user (UID 1000)
	// can write to it even when the host runner has a different UID (e.g. 1001 on GHA).
	if err := os.Chmod(sourcesDir, 0777); err != nil {
		t.Fatalf("Failed to chmod sources dir: %v", err)
	}

	_, wsID := integrationInit(t, storageDir, sourcesDir, "agent-writes-on-host", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Write a file inside the container at the sources mount
	content := "written-by-agent-inside-container"
	outputPath := containerSourcesPath + "/agent-output.txt"
	out, err := exec.Command("podman", "exec", "agent-writes-on-host",
		"sh", "-c", fmt.Sprintf("echo -n '%s' > %s", content, outputPath)).CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec write failed: %v\nOutput: %s", err, string(out))
	}

	// Verify the file appeared on the host filesystem
	hostFile := filepath.Join(sourcesDir, "agent-output.txt")
	data, err := os.ReadFile(hostFile)
	if err != nil {
		t.Fatalf("Failed to read file written by container on host: %v", err)
	}
	if string(data) != content {
		t.Errorf("Expected host file content %q, got %q", content, string(data))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_StartsInProjectDirectory(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	_, wsID := integrationInit(t, storageDir, sourcesDir, "starts-in-project-dir", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Verify the container's working directory
	out, err := exec.Command("podman", "exec", "starts-in-project-dir", "pwd").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec pwd failed: %v\nOutput: %s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != containerSourcesPath {
		t.Errorf("Expected working directory %q, got %q", containerSourcesPath, strings.TrimSpace(string(out)))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_MountExtraDirectories(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	// Create a directory on the host to mount as an additional volume
	extraDir := t.TempDir()
	markerContent := "extra-mount-marker-content"
	if err := os.WriteFile(filepath.Join(extraDir, "extra.txt"), []byte(markerContent), 0644); err != nil {
		t.Fatalf("Failed to create extra marker file: %v", err)
	}

	// Create workspace config with an additional mount
	configDir := filepath.Join(sourcesDir, ".kaiden")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	workspaceJSON := fmt.Sprintf(`{"mounts": [{"host": %q, "target": "/workspace/extra", "ro": true}]}`, extraDir)
	if err := os.WriteFile(filepath.Join(configDir, "workspace.json"), []byte(workspaceJSON), 0644); err != nil {
		t.Fatalf("Failed to write workspace.json: %v", err)
	}

	_, wsID := integrationInit(t, storageDir, sourcesDir, "mount-extra-dirs", "claude")

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Verify the additional mount is accessible inside the container
	out, err := exec.Command("podman", "exec", "mount-extra-dirs", "cat", "/workspace/extra/extra.txt").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec cat extra mount failed: %v\nOutput: %s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != markerContent {
		t.Errorf("Expected extra mount content %q, got %q", markerContent, strings.TrimSpace(string(out)))
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}

func TestIntegration_DuplicateNameGetsIncrement(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir1 := t.TempDir()
	sourcesDir2 := t.TempDir()

	ws1, _ := integrationInit(t, storageDir, sourcesDir1, "dup-name", "claude")
	ws2, _ := integrationInit(t, storageDir, sourcesDir2, "dup-name", "claude")

	if ws1.Name == ws2.Name {
		t.Errorf("Expected unique names for duplicate init, both got %q", ws1.Name)
	}

	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 2 {
		t.Fatalf("Expected 2 workspaces, got %d", len(listResult.Items))
	}
}

func TestIntegration_OperationsOnNonExistentWorkspace(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()

	_, err := integrationExecCmdExpectError(t, "--storage", storageDir, "start", "nonexistent-workspace")
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error for start, got: %v", err)
	}

	_, err = integrationExecCmdExpectError(t, "--storage", storageDir, "stop", "nonexistent-workspace")
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error for stop, got: %v", err)
	}

	_, err = integrationExecCmdExpectError(t, "--storage", storageDir, "remove", "nonexistent-workspace")
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error for remove, got: %v", err)
	}
}

func TestIntegration_RemoveWithSharedImageRunning(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	_, wsID := integrationInit(t, storageDir, sourcesDir, "rm-shared-img", "claude")
	imageName := "kdn-rm-shared-img"

	// Start an external container from the workspace image so the image has a
	// running dependent. This reproduces the CI failure where parallel workspaces
	// share the same image hash and `podman image rm` refuses to delete it.
	externalContainer := "rm-shared-img-external"
	out, err := exec.Command("podman", "run", "-d", "--name", externalContainer, imageName, "sleep", "300").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start external container: %v\nOutput: %s", err, string(out))
	}
	t.Cleanup(func() {
		_ = exec.Command("podman", "rm", "-f", externalContainer).Run()
	})

	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	// Force-remove the workspace while the external container still uses the image.
	// If image cleanup uses `podman image rm` without --force, it will fail with
	// "image is in use by a container".
	integrationExecCmd(t, "--storage", storageDir, "remove", wsID, "--force", "--output", "json")

	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 0 {
		t.Errorf("Expected 0 workspaces after removal, got %d", len(listResult.Items))
	}

	// External container must still be functional
	out2, err := exec.Command("podman", "exec", externalContainer, "echo", "still-alive").CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec on external container failed: %v\nOutput: %s", err, string(out2))
	}
	if !strings.Contains(string(out2), "still-alive") {
		t.Errorf("Expected 'still-alive' from external container, got: %s", string(out2))
	}
}

func TestIntegration_MultipleWorkspacesIsolated(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir1 := t.TempDir()
	sourcesDir2 := t.TempDir()

	// Create distinct marker files in each sources dir
	if err := os.WriteFile(filepath.Join(sourcesDir1, "id.txt"), []byte("workspace-one"), 0644); err != nil {
		t.Fatalf("Failed to create marker in sources1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcesDir2, "id.txt"), []byte("workspace-two"), 0644); err != nil {
		t.Fatalf("Failed to create marker in sources2: %v", err)
	}

	_, wsID1 := integrationInit(t, storageDir, sourcesDir1, "multi-isolated-one", "claude")
	_, wsID2 := integrationInit(t, storageDir, sourcesDir2, "multi-isolated-two", "claude")

	// Start both
	integrationExecCmd(t, "--storage", storageDir, "start", wsID1, "--output", "json")
	integrationExecCmd(t, "--storage", storageDir, "start", wsID2, "--output", "json")

	// Verify both are running
	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 2 {
		t.Fatalf("Expected 2 workspaces, got %d", len(listResult.Items))
	}
	for _, item := range listResult.Items {
		if item.State != "running" {
			t.Errorf("Expected workspace %q to be running, got %q", item.Name, item.State)
		}
	}

	// Verify each container sees its own sources (no cross-contamination)
	idPath := containerSourcesPath + "/id.txt"
	out1, err := exec.Command("podman", "exec", "multi-isolated-one", "cat", idPath).CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec on multi-isolated-one failed: %v\nOutput: %s", err, string(out1))
	}
	if strings.TrimSpace(string(out1)) != "workspace-one" {
		t.Errorf("Expected multi-isolated-one to see 'workspace-one', got %q", strings.TrimSpace(string(out1)))
	}

	out2, err := exec.Command("podman", "exec", "multi-isolated-two", "cat", idPath).CombinedOutput()
	if err != nil {
		t.Fatalf("podman exec on multi-isolated-two failed: %v\nOutput: %s", err, string(out2))
	}
	if strings.TrimSpace(string(out2)) != "workspace-two" {
		t.Errorf("Expected multi-isolated-two to see 'workspace-two', got %q", strings.TrimSpace(string(out2)))
	}

	// Stop both
	integrationExecCmd(t, "--storage", storageDir, "stop", wsID1, "--output", "json")
	integrationExecCmd(t, "--storage", storageDir, "stop", wsID2, "--output", "json")
}

func TestIntegration_WorkspaceWithSecret(t *testing.T) {
	skipIfNoPodman(t)
	t.Parallel()

	storageDir := t.TempDir()
	sourcesDir := t.TempDir()

	// Create a secret using kdn secret create. Both the secret command and the
	// workspace manager use the same storageDir, so the metadata file and the
	// in-memory mock keyring are shared across all commands in this test.
	integrationExecCmd(t,
		"--storage", storageDir,
		"secret", "create", "integration-test-token",
		"--type", "other",
		"--value", "supersecretvalue",
		"--host", "api.example.com",
		"--header", "Authorization",
		"--output", "json",
	)

	// Create workspace configuration that references the secret.
	configDir := filepath.Join(sourcesDir, ".kaiden")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	workspaceJSON := `{"secrets": ["integration-test-token"]}`
	if err := os.WriteFile(filepath.Join(configDir, "workspace.json"), []byte(workspaceJSON), 0644); err != nil {
		t.Fatalf("Failed to write workspace.json: %v", err)
	}

	// Create the workspace; this reads the secret from the store and sets up the
	// onecli proxy so the secret can be injected at runtime.
	_, wsID := integrationInit(t, storageDir, sourcesDir, "workspace-with-secret", "claude")

	// Start the workspace and verify it reaches the running state.
	integrationExecCmd(t, "--storage", storageDir, "start", wsID, "--output", "json")

	listResult := integrationListWorkspaces(t, storageDir)
	if len(listResult.Items) != 1 {
		t.Fatalf("Expected 1 workspace, got %d", len(listResult.Items))
	}
	if listResult.Items[0].State != "running" {
		t.Errorf("Expected state 'running', got %q", listResult.Items[0].State)
	}

	integrationExecCmd(t, "--storage", storageDir, "stop", wsID, "--output", "json")
}
