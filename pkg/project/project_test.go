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

package project

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openkaiden/kdn/pkg/git"
)

// fakeGitDetector is a test double for git.Detector.
type fakeGitDetector struct {
	info *git.RepositoryInfo
	err  error
}

func (f *fakeGitDetector) DetectRepository(_ context.Context, _ string) (*git.RepositoryInfo, error) {
	return f.info, f.err
}

func detect(t *testing.T, info *git.RepositoryInfo, err error) string {
	t.Helper()
	d := NewDetector(&fakeGitDetector{info: info, err: err})
	return d.DetectProject(context.Background(), "/some/dir")
}

// TestDetectProject_NotGitRepo verifies that a non-git directory is returned as-is.
func TestDetectProject_NotGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := NewDetector(&fakeGitDetector{err: git.ErrNotGitRepository})
	got := d.DetectProject(context.Background(), dir)
	if got != dir {
		t.Errorf("expected %q, got %q", dir, got)
	}
}

// TestDetectProject_GitRoot_NoRemote verifies that a git repo at its root with
// no remote returns the repository root directory.
func TestDetectProject_GitRoot_NoRemote(t *testing.T) {
	t.Parallel()
	got := detect(t, &git.RepositoryInfo{RootDir: "/repo", RemoteURL: "", RelativePath: ""}, nil)
	if got != "/repo" {
		t.Errorf("expected %q, got %q", "/repo", got)
	}
}

// TestDetectProject_GitSubdir_NoRemote verifies that a git repo without a remote,
// where the directory is a subdirectory, returns the joined root+relative path.
func TestDetectProject_GitSubdir_NoRemote(t *testing.T) {
	t.Parallel()
	got := detect(t, &git.RepositoryInfo{RootDir: "/repo", RemoteURL: "", RelativePath: "sub/dir"}, nil)
	want := filepath.Join("/repo", "sub", "dir")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestDetectProject_GitRoot_WithRemote verifies that a git repo at its root with
// a remote returns the remote URL with a trailing slash.
func TestDetectProject_GitRoot_WithRemote(t *testing.T) {
	t.Parallel()
	got := detect(t, &git.RepositoryInfo{RootDir: "/repo", RemoteURL: "https://github.com/example/repo", RelativePath: ""}, nil)
	want := "https://github.com/example/repo/"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestDetectProject_GitRoot_WithRemote_TrailingSlash verifies that a remote URL
// that already ends with "/" is not doubled.
func TestDetectProject_GitRoot_WithRemote_TrailingSlash(t *testing.T) {
	t.Parallel()
	got := detect(t, &git.RepositoryInfo{RootDir: "/repo", RemoteURL: "https://github.com/example/repo/", RelativePath: ""}, nil)
	want := "https://github.com/example/repo/"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestDetectProject_GitSubdir_WithRemote verifies that a git repo with a remote,
// where the directory is a subdirectory, appends the relative path to the remote URL.
func TestDetectProject_GitSubdir_WithRemote(t *testing.T) {
	t.Parallel()
	got := detect(t, &git.RepositoryInfo{RootDir: "/repo", RemoteURL: "https://github.com/example/repo", RelativePath: "sub/dir"}, nil)
	want := "https://github.com/example/repo/sub/dir"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
