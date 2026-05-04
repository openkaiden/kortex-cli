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
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestFindPodmanCLI cannot be parallel because subtests use t.Setenv.
func TestFindPodmanCLI(t *testing.T) {
	t.Run("returns path when podman is found in PATH", func(t *testing.T) {
		dir := t.TempDir()
		fakeName := "podman"
		if runtime.GOOS == "windows" {
			fakeName = "podman.exe"
		}
		fakeExe := filepath.Join(dir, fakeName)
		if err := os.WriteFile(fakeExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("failed to create fake podman: %v", err)
		}
		t.Setenv("PATH", dir)

		result := findPodmanCLI()

		if result == nil {
			t.Fatal("expected non-nil result when podman is in PATH")
		}
		if *result != fakeExe {
			t.Errorf("expected %q, got %q", fakeExe, *result)
		}
	})

	t.Run("returns nil when podman is not found in PATH on non-darwin", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Skip("test only applies to non-macOS platforms")
		}
		emptyDir := t.TempDir()
		t.Setenv("PATH", emptyDir)

		result := findPodmanCLI()

		if result != nil {
			t.Errorf("expected nil result, got %q", *result)
		}
	})

	t.Run("returns fallback path when podman not in PATH but found at known macOS location", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("test only applies to macOS")
		}
		dir := t.TempDir()
		fakePodman := filepath.Join(dir, "podman")
		if err := os.WriteFile(fakePodman, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("failed to create fake podman: %v", err)
		}
		emptyDir := t.TempDir()
		t.Setenv("PATH", emptyDir)

		orig := macosPodmanPaths
		macosPodmanPaths = []string{filepath.Join(t.TempDir(), "nonexistent"), fakePodman}
		defer func() { macosPodmanPaths = orig }()

		result := findPodmanCLI()

		if result == nil {
			t.Fatal("expected non-nil result when podman found at macOS fallback path")
		}
		if *result != fakePodman {
			t.Errorf("expected %q, got %q", fakePodman, *result)
		}
	})

	t.Run("returns nil when podman not in PATH and no macOS fallback exists", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("test only applies to macOS")
		}
		emptyDir := t.TempDir()
		t.Setenv("PATH", emptyDir)

		orig := macosPodmanPaths
		macosPodmanPaths = []string{
			filepath.Join(t.TempDir(), "nonexistent1"),
			filepath.Join(t.TempDir(), "nonexistent2"),
		}
		defer func() { macosPodmanPaths = orig }()

		result := findPodmanCLI()

		if result != nil {
			t.Errorf("expected nil result, got %q", *result)
		}
	})
}
