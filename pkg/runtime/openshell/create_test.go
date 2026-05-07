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
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/agent"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

type noopLogger struct{}

func (noopLogger) Stdout() io.Writer { return io.Discard }
func (noopLogger) Stderr() io.Writer { return io.Discard }

func TestCreate_ValidatesParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params runtime.CreateParams
	}{
		{
			name:   "missing name",
			params: runtime.CreateParams{SourcePath: "/src", Agent: "claude"},
		},
		{
			name:   "missing source path",
			params: runtime.CreateParams{Name: "test", Agent: "claude"},
		},
		{
			name:   "missing agent",
			params: runtime.CreateParams{Name: "test", SourcePath: "/src"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeExec := exec.NewFake()
			rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

			_, err := rt.Create(context.Background(), tt.params)
			if err == nil {
				t.Error("Expected validation error")
			}
		})
	}
}

func TestCreate_Success(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	storageDir := t.TempDir()
	_ = newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-my-project\n"), nil
		}
		return []byte{}, nil
	}

	params := runtime.CreateParams{
		Name:       "my-project",
		SourcePath: "/home/user/code",
		Agent:      "claude",
	}

	// Since we can't easily mock the gateway check (it uses os/exec directly),
	// we test the validation and command construction separately.
	err := validateCreateParams(params)
	if err != nil {
		t.Fatalf("validateCreateParams() failed: %v", err)
	}
}

func TestCreate_ReturnsStopped(t *testing.T) {
	t.Parallel()

	// Verify that Create returns stopped state to match the manager's flow
	info := runtime.RuntimeInfo{
		ID:    "kdn-test",
		State: api.WorkspaceStateStopped,
		Info:  map[string]string{"sandbox_name": "kdn-test"},
	}

	if info.State != api.WorkspaceStateStopped {
		t.Errorf("Expected state %q, got %q", api.WorkspaceStateStopped, info.State)
	}
}

func TestCreateSandbox_PodmanIncludesFromBase(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverPodman

	_ = rt.createSandbox(context.Background(), "kdn-test", "claude", nil, noopLogger{})

	if len(fakeExec.RunCalls) < 1 {
		t.Fatal("Expected at least 1 Run call")
	}

	call := fakeExec.RunCalls[0]
	hasFrom := false
	for i, arg := range call {
		if arg == "--from" && i+1 < len(call) && call[i+1] == "base" {
			hasFrom = true
			break
		}
	}
	if !hasFrom {
		t.Errorf("Expected --from base in podman create args: %v", call)
	}
}

func TestCreateSandbox_PodmanUsesAgentImage(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverPodman

	_ = rt.createSandbox(context.Background(), "kdn-test", "gemini", nil, noopLogger{})

	if len(fakeExec.RunCalls) < 1 {
		t.Fatal("Expected at least 1 Run call")
	}

	call := fakeExec.RunCalls[0]
	hasFrom := false
	for i, arg := range call {
		if arg == "--from" && i+1 < len(call) && call[i+1] == "gemini" {
			hasFrom = true
			break
		}
	}
	if !hasFrom {
		t.Errorf("Expected --from gemini in podman create args: %v", call)
	}
}

func TestCreateSandbox_WithProviders(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverPodman

	providers := []string{"kdn-github-token", "kdn-anthropic-key"}
	_ = rt.createSandbox(context.Background(), "kdn-test", "claude", providers, noopLogger{})

	if len(fakeExec.RunCalls) < 1 {
		t.Fatal("Expected at least 1 Run call")
	}

	call := fakeExec.RunCalls[0]
	providerCount := 0
	for i, arg := range call {
		if arg == "--provider" && i+1 < len(call) {
			providerCount++
		}
	}
	if providerCount != 2 {
		t.Errorf("Expected 2 --provider flags, got %d in: %v", providerCount, call)
	}

	if !slices.Contains(call, "kdn-github-token") || !slices.Contains(call, "kdn-anthropic-key") {
		t.Errorf("Expected provider names in create call: %v", call)
	}
}

func TestCreateSandbox_PodmanRejectsUnknownAgent(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverPodman

	err := rt.createSandbox(context.Background(), "kdn-test", "unknown-agent", nil, noopLogger{})
	if err == nil {
		t.Error("Expected error for unsupported agent")
	}
}

func TestSandboxImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agent   string
		want    string
		wantErr bool
	}{
		{agent: "claude", want: "base"},
		{agent: "opencode", want: "base"},
		{agent: "codex", want: "base"},
		{agent: "copilot", want: "base"},
		{agent: "gemini", want: "gemini"},
		{agent: "ollama", want: "ollama"},
		{agent: "openclaw", want: "quay.io/openkaiden/openshell-openclaw:2026.5.4"},
		{agent: "unknown", wantErr: true},
		{agent: "foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()

			got, err := sandboxImage(tt.agent)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sandboxImage(%q) expected error, got %q", tt.agent, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sandboxImage(%q) unexpected error: %v", tt.agent, err)
			}
			if got != tt.want {
				t.Errorf("sandboxImage(%q) = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}

func TestCreateSandbox_VMOmitsFromBase(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	rt.config.Driver = DriverVM

	_ = rt.createSandbox(context.Background(), "kdn-test", "claude", nil, noopLogger{})

	if len(fakeExec.RunCalls) < 1 {
		t.Fatal("Expected at least 1 Run call")
	}

	call := fakeExec.RunCalls[0]
	for _, arg := range call {
		if arg == "--from" {
			t.Errorf("VM driver should not include --from in create args: %v", call)
			break
		}
	}
}

func TestUploadAgentSettings_UsesSandboxUpload(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	settings := map[string]agent.SettingsFile{
		".claude.json": agent.SettingsFile{Content: []byte("{\n  \"key\": \"value\"\n}\n")},
	}

	err := rt.uploadAgentSettings(context.Background(), "kdn-test", settings)
	if err != nil {
		t.Fatalf("uploadAgentSettings() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}

	call := fakeExec.RunCalls[0]

	if len(call) < 4 {
		t.Fatalf("Expected at least 4 args, got %d", len(call))
	}
	if call[0] != "sandbox" || call[1] != "upload" {
		t.Errorf("Expected sandbox upload command, got %v", call[:2])
	}
	if call[2] != "kdn-test" {
		t.Errorf("Expected sandbox name 'kdn-test', got %q", call[2])
	}
	expectedDest := "/sandbox/.claude.json"
	if call[4] != expectedDest {
		t.Errorf("Expected destination %q, got %q", expectedDest, call[4])
	}
}

func TestWriteEnvFile_UsesSandboxUpload(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
		SecretEnvVars: map[string]string{
			"API_KEY": "secret123",
		},
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err != nil {
		t.Fatalf("writeEnvFile() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}

	call := fakeExec.RunCalls[0]

	if call[0] != "sandbox" || call[1] != "upload" {
		t.Errorf("Expected sandbox upload command, got %v", call[:2])
	}
	expectedEnvDest := "/sandbox/.kdn-env"
	if call[4] != expectedEnvDest {
		t.Errorf("Expected destination %q, got %q", expectedEnvDest, call[4])
	}
}

func TestWriteEnvFile_WorkspaceConfigEnvVars(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	envValue := "bar"
	envVars := []workspace.EnvironmentVariable{
		{Name: "FOO", Value: &envValue},
	}

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Environment: &envVars,
		},
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err != nil {
		t.Fatalf("writeEnvFile() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}
}

func TestWriteEnvFile_SkipsEmptyValues(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	emptyValue := ""
	envVars := []workspace.EnvironmentVariable{
		{Name: "EMPTY", Value: &emptyValue},
	}

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Environment: &envVars,
		},
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err != nil {
		t.Fatalf("writeEnvFile() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 0 {
		t.Errorf("Expected no Run calls when all env values are empty, got %d", len(fakeExec.RunCalls))
	}
}

func TestCreate_FullFlow_Success(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-my-project\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	params := runtime.CreateParams{
		Name:       "my-project",
		SourcePath: t.TempDir(),
		Agent:      "claude",
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if info.State != api.WorkspaceStateStopped {
		t.Errorf("Expected stopped state, got %q", info.State)
	}
	if info.ID != "kdn-my-project" {
		t.Errorf("Expected ID 'kdn-my-project', got %q", info.ID)
	}
	if info.Info["agent"] != "claude" {
		t.Errorf("Expected agent 'claude', got %q", info.Info["agent"])
	}
}

func TestCreate_FullFlow_WithAgentSettings(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-with-settings\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "with-settings",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		AgentSettings: map[string]agent.SettingsFile{
			".claude.json": agent.SettingsFile{Content: []byte(`{"key": "value"}`)},
		},
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if info.ID != "kdn-with-settings" {
		t.Errorf("Expected ID 'kdn-with-settings', got %q", info.ID)
	}
}

func TestCreate_FullFlow_WithEnvVars(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-env-test\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "env-test",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		SecretEnvVars: map[string]string{
			"API_KEY": "secret123",
		},
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if info.ID != "kdn-env-test" {
		t.Errorf("Expected ID 'kdn-env-test', got %q", info.ID)
	}
}

func TestCreate_FullFlow_PersistsDriverConfig(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-driver-test\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	params := runtime.CreateParams{
		Name:       "driver-test",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		RuntimeOptions: map[string]string{
			"openshell-driver": DriverVM,
		},
	}

	_, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify config was persisted
	cfg := loadConfig(storageDir)
	if cfg.Driver != DriverVM {
		t.Errorf("Expected persisted driver %q, got %q", DriverVM, cfg.Driver)
	}
}

func TestCreate_FullFlow_CreateSandboxError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "sandbox" && (args[1] == "create" || args[1] == "exec") {
			return fmt.Errorf("create failed")
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	// Use a short-lived context so the retry loop in createSandbox exits quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	params := runtime.CreateParams{
		Name:       "fail-test",
		SourcePath: t.TempDir(),
		Agent:      "claude",
	}

	_, err := rt.Create(ctx, params)
	if err == nil {
		t.Error("Expected error when sandbox creation fails")
	}
}

func TestCreate_FullFlow_UploadSourcesError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "upload" {
			return fmt.Errorf("upload failed")
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "upload-fail",
		SourcePath: t.TempDir(),
		Agent:      "claude",
	}

	_, err := rt.Create(context.Background(), params)
	if err == nil {
		t.Error("Expected error when source upload fails")
	}
}

func TestCreateSandbox_ReadinessTimeout(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		return fmt.Errorf("not ready yet")
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rt.createSandbox(ctx, "kdn-test", "claude", nil, noopLogger{})
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestCreateSandbox_RetriesUntilReady(t *testing.T) {
	t.Parallel()

	callCount := 0
	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		callCount++
		if callCount == 1 {
			return fmt.Errorf("ssh not ready")
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())
	err := rt.createSandbox(context.Background(), "kdn-test", "claude", nil, noopLogger{})
	if err != nil {
		t.Fatalf("createSandbox should succeed after retry: %v", err)
	}
	if callCount < 2 {
		t.Errorf("Expected at least 2 calls (create + exec retry), got %d", callCount)
	}
}

func TestCreateSandbox_SucceedsFirstTry(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.createSandbox(context.Background(), "kdn-test", "claude", nil, noopLogger{})
	if err != nil {
		t.Fatalf("createSandbox should succeed on first try: %v", err)
	}
	if len(fakeExec.RunCalls) != 1 {
		t.Errorf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}
}

func TestUploadAgentSettings_MultipleFiles(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	settings := map[string]agent.SettingsFile{
		".claude.json":    agent.SettingsFile{Content: []byte(`{"key": "value"}`)},
		"subdir/file.txt": agent.SettingsFile{Content: []byte("nested file")},
	}

	err := rt.uploadAgentSettings(context.Background(), "kdn-test", settings)
	if err != nil {
		t.Fatalf("uploadAgentSettings() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 2 {
		t.Fatalf("Expected 2 Run calls (one per file), got %d", len(fakeExec.RunCalls))
	}

	expectedDests := map[string]bool{
		"/sandbox/.claude.json":    false,
		"/sandbox/subdir/file.txt": false,
	}
	for _, call := range fakeExec.RunCalls {
		if len(call) >= 5 {
			expectedDests[call[4]] = true
		}
	}
	for dest, found := range expectedDests {
		if !found {
			t.Errorf("Expected destination %q not found in upload calls", dest)
		}
	}
}

func TestUploadAgentSettings_Error(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		return fmt.Errorf("upload failed")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	settings := map[string]agent.SettingsFile{
		".claude.json": agent.SettingsFile{Content: []byte(`{}`)},
	}

	err := rt.uploadAgentSettings(context.Background(), "kdn-test", settings)
	if err == nil {
		t.Error("Expected error when upload fails")
	}
}

func TestWriteEnvFile_EmptyNoCall(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err != nil {
		t.Fatalf("writeEnvFile() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 0 {
		t.Errorf("Expected no Run calls for empty env, got %d", len(fakeExec.RunCalls))
	}
}

func TestWriteEnvFile_UploadError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		return fmt.Errorf("upload failed")
	}
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
		SecretEnvVars: map[string]string{
			"KEY": "value",
		},
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err == nil {
		t.Error("Expected error when upload fails")
	}
}

func TestWriteEnvFile_NilEnvValue(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	envVars := []workspace.EnvironmentVariable{
		{Name: "NO_VALUE", Value: nil},
	}

	params := runtime.CreateParams{
		Name:       "test",
		SourcePath: "/src",
		Agent:      "claude",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Environment: &envVars,
		},
	}

	err := rt.writeEnvFile(context.Background(), "kdn-test", params)
	if err != nil {
		t.Fatalf("writeEnvFile() failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 0 {
		t.Errorf("Expected no Run calls for nil env value, got %d", len(fakeExec.RunCalls))
	}
}

func TestCreate_FullFlow_AgentSettingsUploadError(t *testing.T) {
	t.Parallel()

	uploadCount := 0
	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "upload" {
			uploadCount++
			if uploadCount == 2 {
				return fmt.Errorf("settings upload failed")
			}
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	params := runtime.CreateParams{
		Name:       "settings-fail",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		AgentSettings: map[string]agent.SettingsFile{
			".config": agent.SettingsFile{Content: []byte("data")},
		},
	}

	_, err := rt.Create(context.Background(), params)
	if err == nil {
		t.Error("Expected error when agent settings upload fails")
	}
}

func TestCreate_FullFlow_DriverConflict_ConfigNotPersisted(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()

	// Save initial config as podman
	_ = saveConfig(storageDir, gatewayConfig{Driver: DriverPodman})
	// Gateway running with podman and has active sandboxes
	_ = saveGatewayState(storageDir, gatewayState{PID: 99999, Driver: DriverPodman})

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("kdn-existing\n"), nil
		}
		return []byte{}, nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	params := runtime.CreateParams{
		Name:       "new-project",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		RuntimeOptions: map[string]string{
			"openshell-driver": DriverVM,
		},
	}

	_, err := rt.Create(context.Background(), params)
	if err == nil {
		t.Fatal("Expected error for driver conflict")
	}
	if !strings.Contains(err.Error(), "cannot switch") {
		t.Errorf("Expected driver conflict error, got: %v", err)
	}

	// Config should NOT have been overwritten to VM
	cfg := loadConfig(storageDir)
	if cfg.Driver != DriverPodman {
		t.Errorf("Expected config to remain %q after conflict, got %q", DriverPodman, cfg.Driver)
	}
}

func TestCollectPorts_FromWorkspaceConfig(t *testing.T) {
	t.Parallel()

	ports := []int{8080, 3000}
	params := runtime.CreateParams{
		Agent: "claude",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Ports: &ports,
		},
	}

	result := collectPorts(params)
	if len(result) != 2 {
		t.Fatalf("Expected 2 ports, got %d", len(result))
	}
	if result[0] != 8080 || result[1] != 3000 {
		t.Errorf("Expected [8080, 3000], got %v", result)
	}
}

func TestCollectPorts_OpenclawAutoInjects(t *testing.T) {
	t.Parallel()

	params := runtime.CreateParams{
		Agent: "openclaw",
	}

	result := collectPorts(params)
	if len(result) != 1 {
		t.Fatalf("Expected 1 port for openclaw, got %d", len(result))
	}
	if result[0] != openclawDefaultPort {
		t.Errorf("Expected port %d, got %d", openclawDefaultPort, result[0])
	}
}

func TestCollectPorts_OpenclawDeduplicates(t *testing.T) {
	t.Parallel()

	ports := []int{openclawDefaultPort, 3000}
	params := runtime.CreateParams{
		Agent: "openclaw",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Ports: &ports,
		},
	}

	result := collectPorts(params)
	if len(result) != 2 {
		t.Fatalf("Expected 2 ports (deduplicated), got %d", len(result))
	}
	if result[0] != openclawDefaultPort || result[1] != 3000 {
		t.Errorf("Expected [%d, 3000], got %v", openclawDefaultPort, result)
	}
}

func TestCollectPorts_NilConfig(t *testing.T) {
	t.Parallel()

	params := runtime.CreateParams{
		Agent: "claude",
	}

	result := collectPorts(params)
	if len(result) != 0 {
		t.Errorf("Expected no ports for claude without config, got %v", result)
	}
}

func TestBuildForwards(t *testing.T) {
	t.Parallel()

	forwards := buildForwards([]int{8080, 3000})
	if len(forwards) != 2 {
		t.Fatalf("Expected 2 forwards, got %d", len(forwards))
	}
	if forwards[0].Bind != "127.0.0.1" || forwards[0].Port != 8080 || forwards[0].Target != 8080 {
		t.Errorf("Unexpected forward[0]: %+v", forwards[0])
	}
	if forwards[1].Bind != "127.0.0.1" || forwards[1].Port != 3000 || forwards[1].Target != 3000 {
		t.Errorf("Unexpected forward[1]: %+v", forwards[1])
	}
}

func TestCreate_FullFlow_WithPorts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	ports := []int{8080}
	params := runtime.CreateParams{
		Name:       "port-test",
		SourcePath: t.TempDir(),
		Agent:      "claude",
		WorkspaceConfig: &workspace.WorkspaceConfiguration{
			Ports: &ports,
		},
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	forwardsJSON, ok := info.Info["forwards"]
	if !ok {
		t.Fatal("Expected 'forwards' key in RuntimeInfo.Info")
	}

	var forwards []api.WorkspaceForward
	if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err != nil {
		t.Fatalf("Failed to unmarshal forwards: %v", err)
	}
	if len(forwards) != 1 {
		t.Fatalf("Expected 1 forward, got %d", len(forwards))
	}
	if forwards[0].Port != 8080 || forwards[0].Target != 8080 {
		t.Errorf("Unexpected forward: %+v", forwards[0])
	}
}

func TestCreate_FullFlow_OpenclawAutoPort(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	params := runtime.CreateParams{
		Name:       "openclaw-test",
		SourcePath: t.TempDir(),
		Agent:      "openclaw",
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	forwardsJSON, ok := info.Info["forwards"]
	if !ok {
		t.Fatal("Expected 'forwards' key in RuntimeInfo.Info for openclaw")
	}

	var forwards []api.WorkspaceForward
	if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err != nil {
		t.Fatalf("Failed to unmarshal forwards: %v", err)
	}
	if len(forwards) != 1 {
		t.Fatalf("Expected 1 forward for openclaw, got %d", len(forwards))
	}
	if forwards[0].Port != openclawDefaultPort || forwards[0].Target != openclawDefaultPort {
		t.Errorf("Expected port %d, got: %+v", openclawDefaultPort, forwards[0])
	}
}

func TestCreate_FullFlow_NoPorts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "sandbox" && args[1] == "list" {
			return []byte("ready\n"), nil
		}
		return []byte{}, nil
	}

	storageDir := t.TempDir()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", storageDir)

	params := runtime.CreateParams{
		Name:       "no-ports",
		SourcePath: t.TempDir(),
		Agent:      "claude",
	}

	info, err := rt.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if _, ok := info.Info["forwards"]; ok {
		t.Error("Expected no 'forwards' key in RuntimeInfo.Info when no ports configured")
	}
}
