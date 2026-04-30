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

//go:build darwin

package openshell

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCodesignBinary_SignsBinary(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	src, err := os.Open("/usr/bin/true")
	if err != nil {
		t.Fatalf("failed to open /usr/bin/true: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(binaryPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatalf("failed to copy binary: %v", err)
	}
	dst.Close()

	if err := codesignBinary(binaryPath); err != nil {
		t.Fatalf("codesignBinary() error = %v", err)
	}

	markerPath := binaryPath + codesignedSuffix
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("marker file not created: %v", err)
	}

	cmd := exec.Command("codesign", "--verify", "--verbose", binaryPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("codesign verify failed: %s: %v", string(output), err)
	}
}

func TestCodesignBinary_MarkerPreventsResign(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	if err := os.WriteFile(binaryPath, []byte("not a real binary"), 0755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	markerPath := binaryPath + codesignedSuffix
	if err := os.WriteFile(markerPath, nil, 0644); err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	if err := codesignBinary(binaryPath); err != nil {
		t.Errorf("codesignBinary() should skip signing when marker exists, got error = %v", err)
	}
}

func TestCodesignBinary_NonexistentBinary(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "nonexistent")

	if err := codesignBinary(binaryPath); err == nil {
		t.Error("codesignBinary() should fail for nonexistent binary")
	}

	markerPath := binaryPath + codesignedSuffix
	if _, err := os.Stat(markerPath); err == nil {
		t.Error("marker file should not exist after failed codesign")
	}
}
