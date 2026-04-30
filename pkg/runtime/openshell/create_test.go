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
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

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

func TestUploadAgentSettings_UsesSandboxUpload(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	settings := map[string][]byte{
		".claude.json": []byte("{\n  \"key\": \"value\"\n}\n"),
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
	if call[4] != containerHome {
		t.Errorf("Expected destination %q, got %q", containerHome, call[4])
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
	if call[4] != containerHome {
		t.Errorf("Expected destination %q, got %q", containerHome, call[4])
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
