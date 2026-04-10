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
	"path"
	"path/filepath"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestResolveHostPath(t *testing.T) {
	t.Parallel()

	sourcesDir := filepath.Join("/host", "sources")
	homeDir := filepath.Join("/home", "user")

	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "$SOURCES root",
			host:     "$SOURCES",
			expected: sourcesDir,
		},
		{
			name:     "$SOURCES with subpath",
			host:     "$SOURCES/subdir",
			expected: filepath.Join(sourcesDir, "subdir"),
		},
		{
			name:     "$SOURCES with parent traversal",
			host:     "$SOURCES/../sibling",
			expected: filepath.Join(sourcesDir, "..", "sibling"),
		},
		{
			name:     "$HOME root",
			host:     "$HOME",
			expected: homeDir,
		},
		{
			name:     "$HOME with subpath",
			host:     "$HOME/.ssh",
			expected: filepath.Join(homeDir, ".ssh"),
		},
		{
			name:     "$HOME with nested subpath",
			host:     "$HOME/.config/gh",
			expected: filepath.Join(homeDir, ".config", "gh"),
		},
		{
			name:     "absolute path",
			host:     "/absolute/path",
			expected: filepath.Join("/absolute", "path"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := resolveHostPath(tc.host, sourcesDir, homeDir)
			if result != tc.expected {
				t.Errorf("resolveHostPath(%q) = %q, want %q", tc.host, result, tc.expected)
			}
		})
	}
}

func TestResolveTargetPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{
			name:     "$SOURCES root",
			target:   "$SOURCES",
			expected: containerWorkspaceSources,
		},
		{
			name:     "$SOURCES with subpath",
			target:   "$SOURCES/pkg",
			expected: path.Join(containerWorkspaceSources, "pkg"),
		},
		{
			name:     "$SOURCES with parent traversal",
			target:   "$SOURCES/../sibling",
			expected: path.Join("/workspace", "sibling"),
		},
		{
			name:     "$HOME root",
			target:   "$HOME",
			expected: containerHome,
		},
		{
			name:     "$HOME with subpath",
			target:   "$HOME/.ssh",
			expected: path.Join(containerHome, ".ssh"),
		},
		{
			name:     "$HOME with nested subpath",
			target:   "$HOME/.config/gh",
			expected: path.Join(containerHome, ".config", "gh"),
		},
		{
			name:     "absolute path",
			target:   "/workspace/sources",
			expected: "/workspace/sources",
		},
		{
			name:     "absolute path with dots cleaned",
			target:   "/workspace/sources/../other",
			expected: "/workspace/other",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := resolveTargetPath(tc.target)
			if result != tc.expected {
				t.Errorf("resolveTargetPath(%q) = %q, want %q", tc.target, result, tc.expected)
			}
		})
	}
}

func TestMountVolumeArg(t *testing.T) {
	t.Parallel()

	sourcesDir := filepath.Join("/host", "project")
	homeDir := filepath.Join("/home", "user")
	roTrue := true

	tests := []struct {
		name     string
		mount    workspace.Mount
		expected string
	}{
		{
			name:     "absolute host and target, read-write",
			mount:    workspace.Mount{Host: "/host/data", Target: "/workspace/data"},
			expected: filepath.Join("/host", "data") + ":/workspace/data:Z",
		},
		{
			name:     "absolute host and target, read-only",
			mount:    workspace.Mount{Host: "/host/data", Target: "/workspace/data", Ro: &roTrue},
			expected: filepath.Join("/host", "data") + ":/workspace/data:ro,Z",
		},
		{
			name:     "$SOURCES host and target",
			mount:    workspace.Mount{Host: "$SOURCES", Target: "$SOURCES"},
			expected: sourcesDir + ":" + containerWorkspaceSources + ":Z",
		},
		{
			name:     "$SOURCES with subpath",
			mount:    workspace.Mount{Host: "$SOURCES/pkg", Target: "$SOURCES/pkg"},
			expected: filepath.Join(sourcesDir, "pkg") + ":" + path.Join(containerWorkspaceSources, "pkg") + ":Z",
		},
		{
			name:     "$HOME host and target",
			mount:    workspace.Mount{Host: "$HOME/.ssh", Target: "$HOME/.ssh"},
			expected: filepath.Join(homeDir, ".ssh") + ":" + path.Join(containerHome, ".ssh") + ":Z",
		},
		{
			name:     "$HOME read-only",
			mount:    workspace.Mount{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig", Ro: &roTrue},
			expected: filepath.Join(homeDir, ".gitconfig") + ":" + path.Join(containerHome, ".gitconfig") + ":ro,Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := mountVolumeArg(tc.mount, sourcesDir, homeDir)
			if result != tc.expected {
				t.Errorf("mountVolumeArg() = %q, want %q", result, tc.expected)
			}
		})
	}
}
