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
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

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
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, workspaceName)

	info, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify pod start was called
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

func TestStart_PodStartFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("pod not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when pod start fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
}

func TestStart_InspectFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("inspect failed")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when inspect fails, got nil")
	}

	fakeExec.AssertRunCalledWith(t, "pod", "start", podName)
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)
}

func TestStart_StepLogger_Success(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		output := fmt.Sprintf("%s|running|kdn-test\n", containerID)
		return []byte(output), nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
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

func TestStart_StepLogger_FailOnPodStart(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("pod not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
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

	if len(fakeLogger.startCalls) != 1 {
		t.Fatalf("Expected 1 Start() call, got %d", len(fakeLogger.startCalls))
	}

	if fakeLogger.startCalls[0].inProgress != fmt.Sprintf("Starting pod: %s", podName) {
		t.Errorf("Expected first step to be 'Starting pod: %s', got %q", podName, fakeLogger.startCalls[0].inProgress)
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
		return nil, fmt.Errorf("failed to inspect container")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
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

	if len(fakeLogger.startCalls) != 2 {
		t.Fatalf("Expected 2 Start() calls, got %d", len(fakeLogger.startCalls))
	}

	expectedSteps := []string{
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
