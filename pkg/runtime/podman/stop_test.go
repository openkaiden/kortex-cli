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

// podInspectOutput returns a fake `podman pod inspect` output listing the
// three containers that kdn pods always contain.
func podInspectOutput(podName string) []byte {
	return []byte(fmt.Sprintf("%s\n%s-onecli\n%s-postgres\n", podName, podName, podName))
}

func TestStop_ValidatesID(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty ID", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		err := p.Stop(context.Background(), "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}

		if !errors.Is(err, runtime.ErrInvalidParams) {
			t.Errorf("Expected ErrInvalidParams, got %v", err)
		}
	})
}

func TestStop_Success(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "my-project")

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return podInspectOutput(podName), nil
	}

	err := p.Stop(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Verify pod inspect was called to discover containers dynamically.
	fakeExec.AssertOutputCalledWith(t, "pod", "inspect", "--format", "{{range .Containers}}{{.Name}}\n{{end}}", podName)

	// Verify each container was stopped individually (not via pod stop).
	fakeExec.AssertRunCalledWith(t, "stop", podName)
	fakeExec.AssertRunCalledWith(t, "stop", podName+"-onecli")
	fakeExec.AssertRunCalledWith(t, "stop", podName+"-postgres")

	for _, call := range fakeExec.RunCalls {
		if len(call) >= 2 && call[0] == "pod" && call[1] == "stop" {
			t.Errorf("Expected podman pod stop NOT to be called, but it was called with: %v", call)
		}
	}
}

func TestStop_PodInspectFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("pod not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	err := p.Stop(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when pod inspect fails, got nil")
	}

	fakeExec.AssertOutputCalledWith(t, "pod", "inspect", "--format", "{{range .Containers}}{{.Name}}\n{{end}}", podName)

	if len(fakeExec.RunCalls) != 0 {
		t.Errorf("Expected no Run calls when inspect fails, got: %v", fakeExec.RunCalls)
	}
}

func TestStop_ContainerStopFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return podInspectOutput(podName), nil
	}

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("container stop failed")
	}

	err := p.Stop(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when container stop fails, got nil")
	}
}

func TestStop_StepLogger_Success(t *testing.T) {
	t.Parallel()

	containerID := "abc123def456"
	fakeExec := exec.NewFake()

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return podInspectOutput(podName), nil
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	err := p.Stop(ctx, containerID)
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	if len(fakeLogger.failCalls) != 0 {
		t.Errorf("Expected no Fail() calls, got %d", len(fakeLogger.failCalls))
	}

	expectedSteps := []stepCall{
		{
			inProgress: fmt.Sprintf("Stopping pod: %s", podName),
			completed:  "Pod stopped",
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

func TestStop_StepLogger_FailOnContainerStop(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)
	podName := setupPodFiles(t, p, containerID, "test-ws")

	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return podInspectOutput(podName), nil
	}

	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("container not found")
	}

	fakeLogger := &fakeStepLogger{}
	ctx := steplogger.WithLogger(context.Background(), fakeLogger)

	err := p.Stop(ctx, containerID)
	if err == nil {
		t.Fatal("Expected Stop() to fail, got nil")
	}

	if fakeLogger.completeCalls != 1 {
		t.Errorf("Expected Complete() to be called 1 time, got %d", fakeLogger.completeCalls)
	}

	if len(fakeLogger.startCalls) != 1 {
		t.Fatalf("Expected 1 Start() call, got %d", len(fakeLogger.startCalls))
	}

	if fakeLogger.startCalls[0].inProgress != fmt.Sprintf("Stopping pod: %s", podName) {
		t.Errorf("Expected step to be 'Stopping pod: %s', got %q", podName, fakeLogger.startCalls[0].inProgress)
	}

	if len(fakeLogger.failCalls) != 1 {
		t.Fatalf("Expected 1 Fail() call, got %d", len(fakeLogger.failCalls))
	}
}
