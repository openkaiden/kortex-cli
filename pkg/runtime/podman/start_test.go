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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// newOnecliStartTestServer starts an httptest server that handles the OneCLI endpoints
// invoked during Start() (health, api-key, rules). Use together with
// podmanRuntime.onecliBaseURLFn to avoid dialling a real localhost port in tests.
func newOnecliStartTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "new-rule"})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestStart_ValidatesID(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty ID", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		_, err := p.Start(context.Background(), "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}

		if !errors.Is(err, runtime.ErrInvalidParams) {
			t.Errorf("Expected ErrInvalidParams, got %v", err)
		}
	})
}

func TestStart_Success(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	workspaceName := "my-project"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	onecliServer := newOnecliStartTestServer(t)
	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return onecliServer.URL }
	podName := setupPodFiles(t, p, containerID, workspaceName)

	info, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify postgres was started individually first
	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")

	// Verify pg_isready was called to wait for postgres
	fakeExec.AssertOutputCalledWith(t, "exec", podName+"-postgres", "pg_isready", "-U", "onecli")

	// Verify network-guard was started
	fakeExec.AssertRunCalledWith(t, "start", podName+"-network-guard")

	// Verify pod start was called for remaining containers
	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)

	// Verify inspect was called
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)

	if info.ID != containerID {
		t.Errorf("Expected ID %s, got %s", containerID, info.ID)
	}
	if info.State != "running" {
		t.Errorf("Expected state 'running', got %s", info.State)
	}
	if info.Info["container_id"] != containerID {
		t.Errorf("Expected container_id %s, got %s", containerID, info.Info["container_id"])
	}
	if info.Info["image_name"] != "kdn-test" {
		t.Errorf("Expected image_name 'kdn-test', got %s", info.Info["image_name"])
	}
}

func TestStart_PostgresStartFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		if len(args) > 0 && args[0] == "start" {
			return fmt.Errorf("container not found")
		}
		return nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when postgres start fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")
}

func TestStart_PodStartFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		if len(args) >= 2 && args[0] == "pod" && args[1] == "start" {
			return fmt.Errorf("pod not found")
		}
		return nil
	}

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("accepting connections\n"), nil
	}

	onecliServer := newOnecliStartTestServer(t)
	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return onecliServer.URL }
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when pod start fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")
	fakeExec.AssertRunCalledWith(t, "start", podName+"-onecli")
	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
}

func TestStart_InspectFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		return nil, fmt.Errorf("inspect failed")
	}

	onecliServer := newOnecliStartTestServer(t)
	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return onecliServer.URL }
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when inspect fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")
	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)
}

func TestStart_PostgresReadinessFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Start(ctx, containerID)
	if err == nil {
		t.Fatal("Expected error when postgres readiness check fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")
}

func TestStart_StepLogger_Success(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	onecliServer := newOnecliStartTestServer(t)
	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return onecliServer.URL }
	podName := setupPodFiles(t, p, containerID, "test-ws")

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	_, err := p.Start(ctx, containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	if len(fakeLogger.failCalls) != 0 {
		t.Errorf("Expected no Fail() calls, got %d", len(fakeLogger.failCalls))
	}

	expectedSteps := []stepCall{
		{
			inProgress: "Starting postgres",
			completed:  "Postgres started",
		},
		{
			inProgress: "Waiting for postgres to be ready",
			completed:  "Postgres is ready",
		},
		{
			inProgress: "Starting OneCLI",
			completed:  "OneCLI started",
		},
		{
			inProgress: "Starting network guard",
			completed:  "Network guard started",
		},
		{
			inProgress: "Waiting for OneCLI readiness",
			completed:  "OneCLI ready",
		},
		{
			inProgress: "Clearing network rules",
			completed:  "Network rules cleared",
		},
		{
			inProgress: "Clearing firewall rules",
			completed:  "Firewall rules cleared",
		},
		{
			inProgress: fmt.Sprintf("Starting pod: %s", podName),
			completed:  "Pod started",
		},
		{
			inProgress: "Verifying container status",
			completed:  "Container status verified",
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

func TestStart_StepLogger_FailOnPostgresStart(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("container not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	setupPodFiles(t, p, containerID, "test-ws")

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	_, err := p.Start(ctx, containerID)
	if err == nil {
		t.Fatal("Expected Start() to fail, got nil")
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	if len(fakeLogger.startCalls) != 1 {
		t.Fatalf("Expected 1 Start() call, got %d", len(fakeLogger.startCalls))
	}

	if fakeLogger.startCalls[0].inProgress != "Starting postgres" {
		t.Errorf("Expected first step to be 'Starting postgres', got %q", fakeLogger.startCalls[0].inProgress)
	}

	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}
}

func TestStart_StepLogger_FailOnGetContainerInfo(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		return nil, fmt.Errorf("failed to inspect container")
	}

	onecliServer := newOnecliStartTestServer(t)
	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return onecliServer.URL }
	podName := setupPodFiles(t, p, containerID, "test-ws")

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	_, err := p.Start(ctx, containerID)
	if err == nil {
		t.Fatal("Expected Start() to fail, got nil")
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	if len(fakeLogger.startCalls) != 9 {
		t.Fatalf("Expected 9 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Starting postgres",
		"Waiting for postgres to be ready",
		"Starting OneCLI",
		"Starting network guard",
		"Waiting for OneCLI readiness",
		"Clearing network rules",
		"Clearing firewall rules",
		fmt.Sprintf("Starting pod: %s", podName),
		"Verifying container status",
	}

	for i, expected := range expectedSteps {
		if fakeLogger.startCalls[i].inProgress != expected {
			t.Errorf("Step %d: expected %q, got %q", i, expected, fakeLogger.startCalls[i].inProgress)
		}
	}

	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}
}

// setupPodFilesWithSource is like setupPodFiles but writes a workspace.json at
// sourceDir/.kaiden/workspace.json so loadNetworkConfig picks it up.
func setupPodFilesWithSource(t *testing.T, p *podmanRuntime, containerID, workspaceName, workspaceJSON string) string {
	t.Helper()
	if p.storageDir == "" {
		p.storageDir = t.TempDir()
	}
	if p.globalStorageDir == "" {
		p.globalStorageDir = t.TempDir()
	}

	sourceDir := t.TempDir()
	kaidenDir := filepath.Join(sourceDir, ".kaiden")
	if err := os.MkdirAll(kaidenDir, 0755); err != nil {
		t.Fatalf("failed to create .kaiden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kaidenDir, "workspace.json"), []byte(workspaceJSON), 0644); err != nil {
		t.Fatalf("failed to write workspace.json: %v", err)
	}

	approvalDir := filepath.Join(p.storageDir, "approval-handler", workspaceName)
	if err := os.MkdirAll(approvalDir, 0755); err != nil {
		t.Fatalf("failed to create approval handler dir: %v", err)
	}
	data := podTemplateData{
		Name:               workspaceName,
		OnecliWebPort:      20254,
		OnecliVersion:      defaultOnecliVersion,
		AgentUID:           1000,
		BaseImageRegistry:  "registry.fedoraproject.org/fedora",
		BaseImageVersion:   "latest",
		SourcePath:         sourceDir,
		ApprovalHandlerDir: approvalDir,
	}
	if err := p.writePodFiles(containerID, data); err != nil {
		t.Fatalf("failed to write pod files for test: %v", err)
	}
	return workspaceName
}

func TestStart_AllowMode_ClearsRules(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	deletedIDs := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			existing := []map[string]string{
				{"id": "stale-rule-1"},
				{"id": "stale-rule-2"},
			}
			_ = json.NewEncoder(w).Encode(existing)
		case r.Method == http.MethodDelete:
			deletedIDs = append(deletedIDs, r.URL.Path[len("/api/rules/"):])
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return server.URL }
	setupPodFilesWithSource(t, p, containerID, "test-ws", `{"network":{"mode":"allow"}}`)

	_, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Stale rules from a previous deny-mode start must be deleted.
	if len(deletedIDs) != 2 {
		t.Fatalf("expected 2 deletions, got %d: %v", len(deletedIDs), deletedIDs)
	}
	if deletedIDs[0] != "stale-rule-1" || deletedIDs[1] != "stale-rule-2" {
		t.Errorf("deleted IDs = %v, want [stale-rule-1 stale-rule-2]", deletedIDs)
	}

	// Network-guard must be started in allow mode (to clear firewall rules).
	fakeExec.AssertRunCalledWith(t, "start", "test-ws-network-guard")

	// Approval handler must NOT be started individually in allow mode.
	fakeExec.AssertRunNotCalledWith(t, "start", "test-ws-approval-handler")

	// Pod start must still be called.
	fakeExec.AssertRunCalledWith(t, "pod", "start", "test-ws")
}

func TestStart_AllowMode_StepLogger(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return server.URL }
	podName := setupPodFilesWithSource(t, p, containerID, "test-ws", `{"network":{"mode":"allow"}}`)

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	_, err := p.Start(ctx, containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	expectedSteps := []stepCall{
		{"Starting postgres", "Postgres started"},
		{"Waiting for postgres to be ready", "Postgres is ready"},
		{"Starting OneCLI", "OneCLI started"},
		{"Starting network guard", "Network guard started"},
		{"Waiting for OneCLI readiness", "OneCLI ready"},
		{"Clearing network rules", "Network rules cleared"},
		{"Clearing firewall rules", "Firewall rules cleared"},
		{fmt.Sprintf("Starting pod: %s", podName), "Pod started"},
		{"Verifying container status", "Container status verified"},
	}

	if len(fakeLogger.startCalls) != len(expectedSteps) {
		t.Fatalf("expected %d steps, got %d: %v", len(expectedSteps), len(fakeLogger.startCalls), fakeLogger.startCalls)
	}
	for i, want := range expectedSteps {
		got := fakeLogger.startCalls[i]
		if got.inProgress != want.inProgress || got.completed != want.completed {
			t.Errorf("step %d: got {%q, %q}, want {%q, %q}", i, got.inProgress, got.completed, want.inProgress, want.completed)
		}
	}
}

func TestStart_DenyMode_ConfiguresNetworking(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	var ruleActions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
			var body struct {
				Action string `json:"action"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ruleActions = append(ruleActions, body.Action)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "new-rule"})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return server.URL }
	podName := setupPodFilesWithSource(t, p, containerID, "test-ws", `{"network":{"mode":"deny","hosts":["*.github.com","registry.npmjs.org"]}}`)

	_, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// A single manual_approval catch-all rule should be created; the approval-handler
	// decides per-request using the hosts list, so OneCLI doesn't need per-host rules.
	if len(ruleActions) != 1 || ruleActions[0] != "manual_approval" {
		t.Errorf("expected one manual_approval rule, got: %v", ruleActions)
	}

	// config.json must contain the glob pattern so the approval-handler's matchesPattern
	// can approve api.github.com when *.github.com is in the hosts list.
	approvalDir := filepath.Join(p.storageDir, "approval-handler", "test-ws")
	data, readErr := os.ReadFile(filepath.Join(approvalDir, "config.json"))
	if readErr != nil {
		t.Fatalf("reading config.json: %v", readErr)
	}
	var cfg approvalHandlerConfig
	if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
		t.Fatalf("unmarshaling config.json: %v", jsonErr)
	}
	if len(cfg.Hosts) != 2 || cfg.Hosts[0] != "*.github.com" || cfg.Hosts[1] != "registry.npmjs.org" {
		t.Errorf("config.hosts = %v, want [*.github.com registry.npmjs.org]", cfg.Hosts)
	}

	// Approval-handler must be started individually after config.json is written,
	// not via pod start (which would run it before config is available).
	fakeExec.AssertRunCalledWith(t, "start", podName+"-approval-handler")

	// Pod start brings up the workspace agent container last.
	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
}

func TestStart_DenyMode_NoHosts_ConfiguresNetworking(t *testing.T) {
	t.Parallel()

	// Regression test: deny mode with no allowed hosts must still configure
	// networking (write config.json and start the approval-handler) so that
	// the approval-handler container has a config file available and does not
	// exit immediately, causing it to be set as stopped.
	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	var ruleActions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
			var body struct {
				Action string `json:"action"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ruleActions = append(ruleActions, body.Action)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "new-rule"})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return server.URL }
	// Deny mode with no hosts field at all.
	podName := setupPodFilesWithSource(t, p, containerID, "test-ws", `{"network":{"mode":"deny"}}`)

	_, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// manual_approval rule must still be created even with no allowed hosts.
	if len(ruleActions) != 1 || ruleActions[0] != "manual_approval" {
		t.Errorf("expected one manual_approval rule, got: %v", ruleActions)
	}

	// config.json must be written with an empty hosts list.
	approvalDir := filepath.Join(p.storageDir, "approval-handler", "test-ws")
	data, readErr := os.ReadFile(filepath.Join(approvalDir, "config.json"))
	if readErr != nil {
		t.Fatalf("reading config.json: %v", readErr)
	}
	var cfg approvalHandlerConfig
	if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
		t.Fatalf("unmarshaling config.json: %v", jsonErr)
	}
	if len(cfg.Hosts) != 0 {
		t.Errorf("config.hosts = %v, want empty", cfg.Hosts)
	}

	// Approval-handler must be started before pod start so it has config.json.
	fakeExec.AssertRunCalledWith(t, "start", podName+"-approval-handler")
	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
}

func TestCollectCredentialHosts(t *testing.T) {
	t.Parallel()

	makeCredDir := func(t *testing.T, storageDir, workspaceName, credName string) {
		t.Helper()
		dir := filepath.Join(storageDir, "credentials", workspaceName, credName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create credential dir: %v", err)
		}
	}

	t.Run("nil registry returns nil", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{storageDir: t.TempDir()}
		result := p.collectCredentialHosts("ws", nil)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("empty registry returns nil", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		p := &podmanRuntime{credentialRegistry: reg, storageDir: t.TempDir()}
		result := p.collectCredentialHosts("ws", nil)
		if len(result) != 0 {
			t.Errorf("Expected empty result, got %v", result)
		}
	})

	t.Run("credential dir absent skips credential", func(t *testing.T) {
		t.Parallel()

		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{
			name:        "mycred",
			detectPath:  "/host/path",
			fakeContent: []byte("ok"),
		})
		p := &podmanRuntime{credentialRegistry: reg, storageDir: t.TempDir()}

		// No credential dir created → stat fails → credential skipped.
		result := p.collectCredentialHosts("ws", nil)
		if len(result) != 0 {
			t.Errorf("Expected empty result when credential dir absent, got %v", result)
		}
	})

	t.Run("credential dir present, nil wsCfg: HostPatterns called with empty hostPath", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		reg := credential.NewRegistry()
		cred := &fakeCredentialForDetect{
			name:        "mycred",
			detectPath:  "/host/path",
			fakeContent: []byte("ok"),
		}
		_ = reg.Register(cred)
		makeCredDir(t, storageDir, "ws", "mycred")

		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}
		result := p.collectCredentialHosts("ws", nil)

		// fakeCredentialForDetect.HostPatterns returns nil, so result is empty.
		if len(result) != 0 {
			t.Errorf("Expected empty hosts (fake returns nil), got %v", result)
		}
	})

	t.Run("credential dir present, nil Mounts: HostPatterns called with empty hostPath", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		reg := credential.NewRegistry()
		_ = reg.Register(&fakeCredentialForDetect{name: "mycred", detectPath: "/host/path"})
		makeCredDir(t, storageDir, "ws", "mycred")

		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}
		result := p.collectCredentialHosts("ws", &workspace.WorkspaceConfiguration{})
		if len(result) != 0 {
			t.Errorf("Expected empty hosts, got %v", result)
		}
	})

	t.Run("credential dir present, Mounts set: Detect called and hostPath forwarded", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// Use a credential that returns host patterns so we can verify the output.
		cred := &fakeCredentialWithHosts{
			fakeCredentialForDetect: fakeCredentialForDetect{
				name:       "mycred",
				detectPath: "/real/credential",
			},
			hostPatterns: []string{"api.example.com", "*.example.com"},
		}
		reg := credential.NewRegistry()
		_ = reg.Register(cred)
		makeCredDir(t, storageDir, "ws", "mycred")

		mounts := []workspace.Mount{{Host: "/real/path", Target: "/container/path"}}
		wsCfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}
		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}

		result := p.collectCredentialHosts("ws", wsCfg)

		if len(result) != 2 {
			t.Fatalf("Expected 2 host patterns, got %d: %v", len(result), result)
		}
		if result[0] != "api.example.com" || result[1] != "*.example.com" {
			t.Errorf("host patterns = %v, want [api.example.com *.example.com]", result)
		}
		// Verify that the detected host path is forwarded to HostPatterns.
		if cred.lastHostPath != "/real/credential" {
			t.Errorf("HostPatterns received hostPath = %q, want %q", cred.lastHostPath, "/real/credential")
		}
	})

	t.Run("only credentials with existing dirs contribute hosts", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		present := &fakeCredentialWithHosts{
			fakeCredentialForDetect: fakeCredentialForDetect{name: "present"},
			hostPatterns:            []string{"present.example.com"},
		}
		absent := &fakeCredentialWithHosts{
			fakeCredentialForDetect: fakeCredentialForDetect{name: "absent"},
			hostPatterns:            []string{"absent.example.com"},
		}
		reg := credential.NewRegistry()
		_ = reg.Register(present)
		_ = reg.Register(absent)

		// Only create the dir for "present".
		makeCredDir(t, storageDir, "ws", "present")

		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}
		result := p.collectCredentialHosts("ws", nil)

		if len(result) != 1 || result[0] != "present.example.com" {
			t.Errorf("host patterns = %v, want [present.example.com]", result)
		}
	})

	t.Run("multiple credentials with dirs: all host patterns collected", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		credA := &fakeCredentialWithHosts{
			fakeCredentialForDetect: fakeCredentialForDetect{name: "cred-a"},
			hostPatterns:            []string{"a1.example.com", "a2.example.com"},
		}
		credB := &fakeCredentialWithHosts{
			fakeCredentialForDetect: fakeCredentialForDetect{name: "cred-b"},
			hostPatterns:            []string{"b.example.com"},
		}
		reg := credential.NewRegistry()
		_ = reg.Register(credA)
		_ = reg.Register(credB)

		makeCredDir(t, storageDir, "ws", "cred-a")
		makeCredDir(t, storageDir, "ws", "cred-b")

		p := &podmanRuntime{credentialRegistry: reg, storageDir: storageDir}
		result := p.collectCredentialHosts("ws", nil)

		if len(result) != 3 {
			t.Fatalf("Expected 3 host patterns, got %d: %v", len(result), result)
		}
	})
}

// fakeCredentialWithHosts extends fakeCredentialForDetect with configurable HostPatterns.
type fakeCredentialWithHosts struct {
	fakeCredentialForDetect
	hostPatterns []string
	lastHostPath string
}

func (f *fakeCredentialWithHosts) HostPatterns(hostPath string) []string {
	f.lastHostPath = hostPath
	return f.hostPatterns
}

func TestStart_DenyMode_StepLogger(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("accepting connections\n"), nil
		}
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "new-rule"})
		default:
			t.Errorf("unexpected OneCLI request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	p.onecliBaseURLFn = func(_ int) string { return server.URL }
	podName := setupPodFilesWithSource(t, p, containerID, "test-ws", `{"network":{"mode":"deny","hosts":["*.github.com"]}}`)

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	_, err := p.Start(ctx, containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	expectedSteps := []stepCall{
		{"Starting postgres", "Postgres started"},
		{"Waiting for postgres to be ready", "Postgres is ready"},
		{"Starting OneCLI", "OneCLI started"},
		{"Starting network guard", "Network guard started"},
		{"Waiting for OneCLI readiness", "OneCLI ready"},
		{"Configuring network rules", "Network rules configured"},
		{"Configuring firewall rules", "Firewall rules configured"},
		{"Starting approval handler", "Approval handler started"},
		{fmt.Sprintf("Starting pod: %s", podName), "Pod started"},
		{"Verifying container status", "Container status verified"},
	}

	if len(fakeLogger.startCalls) != len(expectedSteps) {
		t.Fatalf("expected %d steps, got %d: %v", len(expectedSteps), len(fakeLogger.startCalls), fakeLogger.startCalls)
	}
	for i, want := range expectedSteps {
		got := fakeLogger.startCalls[i]
		if got.inProgress != want.inProgress || got.completed != want.completed {
			t.Errorf("step %d: got {%q, %q}, want {%q, %q}", i, got.inProgress, got.completed, want.inProgress, want.completed)
		}
	}
}
