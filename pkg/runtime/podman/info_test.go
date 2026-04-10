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

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
)

func TestInfo_ValidatesID(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty ID", func(t *testing.T) {
		t.Parallel()

		p := &podmanRuntime{}

		_, err := p.Info(context.Background(), "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}

		if !errors.Is(err, runtime.ErrInvalidParams) {
			t.Errorf("Expected ErrInvalidParams, got %v", err)
		}
	})
}

func TestInfo_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerID   string
		output        string
		expectedState api.WorkspaceState
		expectedImage string
	}{
		{
			name:          "running container",
			containerID:   "abc123def456",
			output:        "abc123def456|running|kdn-test\n",
			expectedState: api.WorkspaceStateRunning,
			expectedImage: "kdn-test",
		},
		{
			name:          "stopped container",
			containerID:   "xyz789ghi012",
			output:        "xyz789ghi012|exited|kdn-stopped\n",
			expectedState: api.WorkspaceStateStopped,
			expectedImage: "kdn-stopped",
		},
		{
			name:          "created container",
			containerID:   "def456jkl789",
			output:        "def456jkl789|created|kdn-new\n",
			expectedState: api.WorkspaceStateStopped,
			expectedImage: "kdn-new",
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

			info, err := p.Info(context.Background(), tt.containerID)
			if err != nil {
				t.Fatalf("Info() failed: %v", err)
			}

			// Verify Output was called to inspect the container
			fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", tt.containerID)

			// Verify returned info
			if info.ID != tt.containerID {
				t.Errorf("Expected ID %s, got %s", tt.containerID, info.ID)
			}
			if info.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, info.State)
			}
			if info.Info["container_id"] != tt.containerID {
				t.Errorf("Expected container_id %s, got %s", tt.containerID, info.Info["container_id"])
			}
			if info.Info["image_name"] != tt.expectedImage {
				t.Errorf("Expected image_name %s, got %s", tt.expectedImage, info.Info["image_name"])
			}
		})
	}
}

func TestInfo_InspectFailure(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	// Set up OutputFunc to return an error
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("container not found")
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	_, err := p.Info(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error when inspect fails, got nil")
	}

	// Verify Output was called
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)
}

func TestInfo_MalformedOutput(t *testing.T) {
	t.Parallel()

	containerID := "abc123"
	fakeExec := exec.NewFake()

	// Set up OutputFunc to return malformed output
	fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("invalid-output-without-pipes\n"), nil
	}

	p := newWithDeps(&fakeSystem{}, fakeExec).(*podmanRuntime)

	_, err := p.Info(context.Background(), containerID)
	if err == nil {
		t.Fatal("Expected error for malformed output, got nil")
	}

	// Verify Output was called
	fakeExec.AssertOutputCalledWith(t, "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{.ImageName}}", containerID)
}

func TestGetContainerInfo_ParsesOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerID   string
		output        string
		expectedState api.WorkspaceState
		expectedImage string
	}{
		{
			name:          "running container",
			containerID:   "abc123",
			output:        "abc123def456|running|kdn-test\n",
			expectedState: api.WorkspaceStateRunning,
			expectedImage: "kdn-test",
		},
		{
			name:          "stopped container",
			containerID:   "xyz789",
			output:        "xyz789ghi012|exited|kdn-stopped\n",
			expectedState: api.WorkspaceStateStopped,
			expectedImage: "kdn-stopped",
		},
		{
			name:          "created container",
			containerID:   "def456",
			output:        "def456|created|kdn-new\n",
			expectedState: api.WorkspaceStateStopped,
			expectedImage: "kdn-new",
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
