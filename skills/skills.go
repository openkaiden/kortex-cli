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

package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// Adding a new built-in skill
//
// 1. Create a directory under skills/ named after the skill (e.g. skills/my-skill/).
// 2. Add a SKILL.md file inside it following the standard skill frontmatter format.
// 3. Add the directory name to the //go:embed directive below (space-separated).
//
// ExtractAll discovers embedded skills by walking the root of builtInFS and
// extracting every top-level directory it finds, so no other code changes are
// needed — the skill is automatically injected into every workspace whose agent
// supports skills.

//go:embed config-kdn-workspace
var builtInFS embed.FS

// ExtractAll extracts all built-in skill directories embedded in the binary
// into storageDir/skills/ and returns their host paths. Existing files are
// overwritten to keep skills up to date with the installed binary.
// New skills are picked up automatically when their directory is added under
// skills/ and listed in the embed directive above.
func ExtractAll(storageDir string) ([]string, error) {
	return extractAll(builtInFS, storageDir)
}

// extractAll is the testable core of ExtractAll. It accepts any fs.FS so that
// tests can inject custom file systems to exercise all code paths.
func extractAll(srcFS fs.FS, storageDir string) ([]string, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		destDir := filepath.Join(storageDir, "skills", entry.Name())
		if err := extractDir(srcFS, entry.Name(), destDir); err != nil {
			return nil, err
		}
		paths = append(paths, destDir)
	}
	return paths, nil
}

func extractDir(src fs.FS, srcDir, destDir string) error {
	return fs.WalkDir(src, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		// embed.FS normalizes all file permissions to 0444 regardless of the
		// original on-disk permissions, so we cannot preserve them through
		// embedding. Use 0644 so that files are writable and can be
		// overwritten on subsequent extractions (e.g., after a binary upgrade).
		return os.WriteFile(dest, data, 0o644)
	})
}
