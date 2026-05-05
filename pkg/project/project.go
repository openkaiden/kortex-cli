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

// Package project provides project identifier detection for a source directory.
package project

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/openkaiden/kdn/pkg/git"
)

// Detector computes a stable string identifier for a project directory.
type Detector interface {
	// DetectProject returns an identifier for dir:
	//   - git repo with remote URL  → "<remoteURL>/" or "<remoteURL>/<relPath>"
	//   - git repo without remote   → root dir, or filepath.Join(root, relPath)
	//   - not a git repo            → dir itself
	DetectProject(ctx context.Context, dir string) string
}

type detector struct {
	gitDetector git.Detector
}

var _ Detector = (*detector)(nil)

// NewDetector returns a Detector backed by the provided git.Detector.
func NewDetector(gitDetector git.Detector) Detector {
	return &detector{gitDetector: gitDetector}
}

// DetectProject detects the project identifier for a source directory.
// Returns:
//   - Git repository with remote: the repository remote URL with workspace path appended
//     (e.g., "https://github.com/user/repo/" for root, "https://github.com/user/repo/sub/path" for subdirectory)
//   - Git repository without remote: the repository root directory with workspace path appended
//   - Non-git directory: the source directory
func (d *detector) DetectProject(ctx context.Context, dir string) string {
	if absDir, err := filepath.Abs(dir); err == nil {
		dir = absDir
	}

	// Try to detect git repository
	repoInfo, err := d.gitDetector.DetectRepository(ctx, dir)
	if err != nil {
		// Not a git repository, use source directory
		return dir
	}

	// Git repository found
	if repoInfo.RemoteURL != "" {
		// Has remote URL, construct project identifier with relative path.
		// Ensure URL ends with slash before appending path.
		base := repoInfo.RemoteURL
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		if repoInfo.RelativePath != "" {
			// Convert to forward slashes for URL compatibility (Windows uses backslashes).
			return base + filepath.ToSlash(repoInfo.RelativePath)
		}
		// At repository root, return base URL with trailing slash.
		return base
	}

	// No remote URL, use repository root directory with relative path.
	// Use filepath.Join for local paths (OS-specific separators).
	if repoInfo.RelativePath != "" {
		return filepath.Join(repoInfo.RootDir, repoInfo.RelativePath)
	}
	return repoInfo.RootDir
}
