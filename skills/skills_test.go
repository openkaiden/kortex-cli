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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
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

	t.Run("returns error when destination is not writable", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("chmod-based permission tests do not apply on Windows")
		}

		storageDir := t.TempDir()
		skillsDir := filepath.Join(storageDir, "skills")
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			t.Fatalf("failed to create skills dir: %v", err)
		}
		// Make skills/ read-only so extractDir cannot create subdirectories inside it.
		if err := os.Chmod(skillsDir, 0o555); err != nil {
			t.Fatalf("failed to chmod skills dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(skillsDir, 0o755) })

		_, err := ExtractAll(storageDir)
		if err == nil {
			t.Error("ExtractAll() expected error when destination is not writable, got nil")
		}
	})
}

// TestExtractAll_internal exercises code paths that are unreachable via the
// real embedded FS by injecting custom fs.FS implementations into extractAll.
func TestExtractAll_internal(t *testing.T) {
	t.Parallel()

	t.Run("non-directory root entries are skipped", func(t *testing.T) {
		t.Parallel()

		srcFS := fstest.MapFS{
			"my-skill/SKILL.md": &fstest.MapFile{Data: []byte("# skill")},
			"loose-file.txt":    &fstest.MapFile{Data: []byte("ignored")},
		}
		storageDir := t.TempDir()
		paths, err := extractAll(srcFS, storageDir)
		if err != nil {
			t.Fatalf("extractAll() error = %v", err)
		}
		if len(paths) != 1 {
			t.Fatalf("expected 1 path (only my-skill/), got %d: %v", len(paths), paths)
		}
		if filepath.Base(paths[0]) != "my-skill" {
			t.Errorf("path = %q, want basename my-skill", paths[0])
		}
		// loose-file.txt must not have been extracted
		if _, err := os.Stat(filepath.Join(storageDir, "skills", "loose-file.txt")); err == nil {
			t.Error("loose-file.txt should not have been extracted")
		}
	})

	t.Run("returns error when srcFS ReadDir fails", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		_, err := extractAll(errorFS{readDirErr: errors.New("read dir failed")}, storageDir)
		if err == nil {
			t.Error("expected error when ReadDir fails, got nil")
		}
	})

	t.Run("returns error when ReadFile fails during walk", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		inner := fstest.MapFS{
			"my-skill/SKILL.md": &fstest.MapFile{Data: []byte("# skill")},
		}
		srcFS := &readFileErrorFS{MapFS: &inner, failOn: "my-skill/SKILL.md"}
		_, err := extractAll(srcFS, storageDir)
		if err == nil {
			t.Error("expected error when ReadFile fails, got nil")
		}
	})

	t.Run("returns error when WalkDir passes error for a directory", func(t *testing.T) {
		t.Parallel()

		// readDirErrorFS makes ReadDir fail for a specific subdirectory, which
		// causes fs.WalkDir to invoke the callback with a non-nil err for that
		// entry — exercising the err-check at the top of the walkDir callback.
		srcFS := readDirErrorFS{
			MapFS: fstest.MapFS{
				"my-skill/SKILL.md": &fstest.MapFile{Data: []byte("# skill")},
			},
			failDir: "my-skill",
		}
		storageDir := t.TempDir()
		_, err := extractAll(srcFS, storageDir)
		if err == nil {
			t.Error("expected error when WalkDir reports a dir error, got nil")
		}
	})
}

// errorFS is an fs.FS that returns a configurable error from ReadDir.
type errorFS struct {
	readDirErr error
}

func (e errorFS) Open(name string) (fs.File, error) {
	return nil, e.readDirErr
}

// readFileErrorFS wraps fstest.MapFS and returns an error when ReadFile is
// called for a specific path. It overrides both Open and ReadFile because
// fs.ReadFile prefers the ReadFileFS interface over Open when available.
type readFileErrorFS struct {
	*fstest.MapFS
	failOn string
}

func (r *readFileErrorFS) Open(name string) (fs.File, error) {
	if name == r.failOn {
		return nil, errors.New("read file failed")
	}
	return r.MapFS.Open(name)
}

func (r *readFileErrorFS) ReadFile(name string) ([]byte, error) {
	if name == r.failOn {
		return nil, errors.New("read file failed")
	}
	return r.MapFS.ReadFile(name)
}

// readDirErrorFS wraps fstest.MapFS and returns an error from ReadDir for a
// specific directory name. This causes fs.WalkDir to invoke the walk callback
// with a non-nil err for that entry, exercising the error-check at the top of
// the walkDir callback.
type readDirErrorFS struct {
	fstest.MapFS
	failDir string
}

func (r readDirErrorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == r.failDir {
		return nil, errors.New("read dir failed")
	}
	return r.MapFS.ReadDir(name)
}
