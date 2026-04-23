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
	"testing"

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

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when pod start fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "start", podName+"-postgres")
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
			inProgress: fmt.Sprintf("Starting pod: %s", podName),
			completed:  "Pod started",
		},
		{
			inProgress: "Waiting for OneCLI readiness",
			completed:  "OneCLI ready",
		},
		{
			inProgress: "Configuring network rules",
			completed:  "Network rules configured",
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

	if len(fakeLogger.startCalls) != 6 {
		t.Fatalf("Expected 6 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
		"Starting postgres",
		"Waiting for postgres to be ready",
		fmt.Sprintf("Starting pod: %s", podName),
		"Waiting for OneCLI readiness",
		"Configuring network rules",
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
