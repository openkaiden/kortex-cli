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

package exec_test

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"

	podmanexec "github.com/openkaiden/kdn/pkg/runtime/podman/exec"
)

func findCommand(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not found in PATH, skipping", name)
	}
	return path
}

func TestNew(t *testing.T) {
	t.Parallel()

	e := podmanexec.New("podman")
	if e == nil {
		t.Fatal("New() returned nil")
	}
}

func TestExecutor_Run(t *testing.T) {
	t.Parallel()

	t.Run("executes command using the configured path", func(t *testing.T) {
		t.Parallel()

		echo := findCommand(t, "echo")
		e := podmanexec.New(echo)

		var stdout strings.Builder
		err := e.Run(context.Background(), &stdout, io.Discard, "hello")
		if err != nil {
			t.Fatalf("Run() returned unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "hello") {
			t.Errorf("expected stdout to contain %q, got %q", "hello", stdout.String())
		}
	})

	t.Run("returns error with stderr on command failure", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		err := e.Run(context.Background(), io.Discard, io.Discard, "-c", "echo 'detailed error' >&2; exit 1")
		if err == nil {
			t.Fatal("expected Run() to return error, got nil")
		}
		if !strings.Contains(err.Error(), "detailed error") {
			t.Errorf("expected error to contain stderr output, got %q", err.Error())
		}
	})

	t.Run("returns error without stderr annotation when stderr is empty", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		err := e.Run(context.Background(), io.Discard, io.Discard, "-c", "exit 1")
		if err == nil {
			t.Fatal("expected Run() to return error, got nil")
		}
		if strings.Contains(err.Error(), "Podman stderr") {
			t.Errorf("expected no stderr annotation when stderr is empty, got %q", err.Error())
		}
	})

	t.Run("writes stderr to provided writer", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		var stderr strings.Builder
		_ = e.Run(context.Background(), io.Discard, &stderr, "-c", "echo 'err output' >&2; exit 1")
		if !strings.Contains(stderr.String(), "err output") {
			t.Errorf("expected stderr writer to receive output, got %q", stderr.String())
		}
	})
}

func TestExecutor_Output(t *testing.T) {
	t.Parallel()

	t.Run("returns stdout bytes on success", func(t *testing.T) {
		t.Parallel()

		echo := findCommand(t, "echo")
		e := podmanexec.New(echo)

		out, err := e.Output(context.Background(), io.Discard, "world")
		if err != nil {
			t.Fatalf("Output() returned unexpected error: %v", err)
		}
		if !strings.Contains(string(out), "world") {
			t.Errorf("expected output to contain %q, got %q", "world", string(out))
		}
	})

	t.Run("returns error with stderr on command failure", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		_, err := e.Output(context.Background(), io.Discard, "-c", "echo 'output error' >&2; exit 1")
		if err == nil {
			t.Fatal("expected Output() to return error, got nil")
		}
		if !strings.Contains(err.Error(), "output error") {
			t.Errorf("expected error to contain stderr output, got %q", err.Error())
		}
	})

	t.Run("returns error without stderr annotation when stderr is empty", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		_, err := e.Output(context.Background(), io.Discard, "-c", "exit 1")
		if err == nil {
			t.Fatal("expected Output() to return error, got nil")
		}
		if strings.Contains(err.Error(), "Podman stderr") {
			t.Errorf("expected no stderr annotation when stderr is empty, got %q", err.Error())
		}
	})

	t.Run("writes stderr to provided writer", func(t *testing.T) {
		t.Parallel()

		sh := findCommand(t, "sh")
		e := podmanexec.New(sh)

		var stderr strings.Builder
		_, _ = e.Output(context.Background(), &stderr, "-c", "echo 'out stderr' >&2; exit 1")
		if !strings.Contains(stderr.String(), "out stderr") {
			t.Errorf("expected stderr writer to receive output, got %q", stderr.String())
		}
	})
}
