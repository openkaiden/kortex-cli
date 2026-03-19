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

	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/exec"
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
	fakeExec := exec.NewFake()

	// Set up OutputFunc to return container info
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		// Simulate podman inspect output
		output := fmt.Sprintf("%s|running|kortex-cli-test\n", containerID)
		return []byte(output), nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	info, err := p.Start(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify Run was called to start the container
	fakeExec.AssertRunCalledWith(t, "start", containerID)

	// Verify Output was called to inspect the container
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)

	// Verify returned info
	if info.ID != containerID {
		t.Errorf("Expected ID %s, got %s", containerID, info.ID)
	}
	if info.State != "running" {
		t.Errorf("Expected state 'running', got %s", info.State)
	}
	if info.Info["container_id"] != containerID {
		t.Errorf("Expected container_id %s, got %s", containerID, info.Info["container_id"])
	}
	if info.Info["image_name"] != "kortex-cli-test" {
		t.Errorf("Expected image_name 'kortex-cli-test', got %s", info.Info["image_name"])
	}
}

func TestStart_StartContainerFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	// Set up RunFunc to return an error
	fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
		return fmt.Errorf("container not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when start fails, got nil")
	}

	// Verify Run was called
	fakeExec.AssertRunCalledWith(t, "start", containerID)
}

func TestStart_InspectFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	// Set up OutputFunc to return an error
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("inspect failed")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	_, err := p.Start(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when inspect fails, got nil")
	}

	// Verify both Run and Output were called
	fakeExec.AssertRunCalledWith(t, "start", containerID)
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)
}

func TestGetContainerInfo_ParsesOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerID   string
		output        string
		expectedState string
		expectedImage string
	}{
		{
			name:          "running container",
			containerID:   "abc123",
			output:        "abc123def456|running|kortex-cli-test\n",
			expectedState: "running",
			expectedImage: "kortex-cli-test",
		},
		{
			name:          "stopped container",
			containerID:   "xyz789",
			output:        "xyz789ghi012|exited|kortex-cli-stopped\n",
			expectedState: "exited",
			expectedImage: "kortex-cli-stopped",
		},
		{
			name:          "created container",
			containerID:   "def456",
			output:        "def456|created|kortex-cli-new\n",
			expectedState: "created",
			expectedImage: "kortex-cli-new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeExec := exec.NewFake()
			fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte(tt.output), nil
			}

			p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

			info, err := p.getContainerInfo(context.Background(), tt.containerID)
			if err != nil {
				t.Fatalf("getContainerInfo() failed: %v", err)
			}

			if info.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, info.State)
			}
			if info.Info["image_name"] != tt.expectedImage {
				t.Errorf("Expected image_name %s, got %s", tt.expectedImage, info.Info["image_name"])
			}
		})
	}
}

func TestGetContainerInfo_MalformedOutput(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("invalid-output-without-pipes\n"), nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	_, err := p.getContainerInfo(context.Background(), "abc123")
	if err == nil {
		t.Fatal("Expected error for malformed output, got nil")
	}
}
