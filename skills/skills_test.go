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
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAll(t *testing.T) {
	t.Parallel()

	t.Run("extracts all built-in skills to storageDir/skills/", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		paths, err := ExtractAll(storageDir)
		if err != nil {
			t.Fatalf("ExtractAll() error = %v", err)
		}

		if len(paths) == 0 {
			t.Fatal("ExtractAll() returned no paths")
		}

		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				t.Errorf("extracted path %q does not exist: %v", p, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("extracted path %q is not a directory", p)
			}
			// Every extracted skill must have a SKILL.md
			skillMD := filepath.Join(p, "SKILL.md")
			if _, err := os.Stat(skillMD); err != nil {
				t.Errorf("extracted skill %q is missing SKILL.md: %v", p, err)
			}
		}
	})

	t.Run("config-kdn-workspace skill is extracted", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		paths, err := ExtractAll(storageDir)
		if err != nil {
			t.Fatalf("ExtractAll() error = %v", err)
		}

		found := false
		for _, p := range paths {
			if filepath.Base(p) == "config-kdn-workspace" {
				found = true
				wantDir := filepath.Join(storageDir, "skills", "config-kdn-workspace")
				if p != wantDir {
					t.Errorf("path = %q, want %q", p, wantDir)
				}
			}
		}
		if !found {
			t.Errorf("config-kdn-workspace not found in extracted paths: %v", paths)
		}
	})

	t.Run("existing files are overwritten on re-extraction", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		paths, err := ExtractAll(storageDir)
		if err != nil {
			t.Fatalf("first ExtractAll() error = %v", err)
		}
		if len(paths) == 0 {
			t.Fatal("first ExtractAll() returned no paths")
		}

		// Corrupt one file
		skillMD := filepath.Join(paths[0], "SKILL.md")
		if err := os.WriteFile(skillMD, []byte("corrupted"), 0o644); err != nil {
			t.Fatalf("failed to corrupt SKILL.md: %v", err)
		}

		// Re-extract should restore the original content
		if _, err := ExtractAll(storageDir); err != nil {
			t.Fatalf("second ExtractAll() error = %v", err)
		}
		data, err := os.ReadFile(skillMD)
		if err != nil {
			t.Fatalf("failed to read SKILL.md after re-extraction: %v", err)
		}
		if string(data) == "corrupted" {
			t.Error("SKILL.md was not overwritten on re-extraction")
		}
	})
}
