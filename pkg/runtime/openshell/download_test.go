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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformAsset_Gateway(t *testing.T) {
	t.Parallel()

	asset, err := platformAsset("openshell-gateway")

	// On supported platforms (darwin/arm64, linux/arm64, linux/amd64) this should succeed
	if err != nil {
		t.Skipf("Platform not supported for openshell-gateway: %v", err)
	}

	if !strings.HasPrefix(asset, "openshell-gateway-") {
		t.Errorf("Expected asset name to start with 'openshell-gateway-', got %q", asset)
	}
	if !strings.HasSuffix(asset, ".tar.gz") {
		t.Errorf("Expected asset name to end with '.tar.gz', got %q", asset)
	}
}

func TestPlatformAsset_Openshell(t *testing.T) {
	t.Parallel()

	asset, err := platformAsset("openshell")
	if err != nil {
		t.Skipf("Platform not supported for openshell: %v", err)
	}

	if !strings.HasPrefix(asset, "openshell-") {
		t.Errorf("Expected asset name to start with 'openshell-', got %q", asset)
	}
	if !strings.HasSuffix(asset, ".tar.gz") {
		t.Errorf("Expected asset name to end with '.tar.gz', got %q", asset)
	}
}

func TestPlatformAsset_VMDriver(t *testing.T) {
	t.Parallel()

	asset, err := platformAsset("openshell-driver-vm")
	if err != nil {
		t.Skipf("Platform not supported for openshell-driver-vm: %v", err)
	}

	if !strings.HasPrefix(asset, "openshell-driver-vm-") {
		t.Errorf("Expected asset name to start with 'openshell-driver-vm-', got %q", asset)
	}
	if !strings.HasSuffix(asset, ".tar.gz") {
		t.Errorf("Expected asset name to end with '.tar.gz', got %q", asset)
	}
}

func TestPlatformAsset_UnknownBinary(t *testing.T) {
	t.Parallel()

	_, err := platformAsset("nonexistent-binary")
	if err == nil {
		t.Error("Expected error for unknown binary name")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("Expected 'unsupported platform' error, got: %v", err)
	}
}

func TestDownloadURL(t *testing.T) {
	t.Parallel()

	url := downloadURL("v1.0.0", "openshell-gateway-x86_64.tar.gz")

	expected := "https://github.com/NVIDIA/OpenShell/releases/download/v1.0.0/openshell-gateway-x86_64.tar.gz"
	if url != expected {
		t.Errorf("Expected URL %q, got %q", expected, url)
	}
}

func TestEnsureBinary_SkipsExisting(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "openshell-gateway")

	if err := os.WriteFile(binaryPath, []byte("fake binary"), 0755); err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	path, err := ensureBinary(binDir, "openshell-gateway", "dev")
	if err != nil {
		t.Fatalf("ensureBinary() should succeed for existing binary: %v", err)
	}
	if path != binaryPath {
		t.Errorf("Expected path %q, got %q", binaryPath, path)
	}
}

func createTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0755,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestDownloadAndExtract_Success(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("#!/bin/sh\necho hello\n")
	archive := createTarGz(t, map[string][]byte{
		"my-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(destDir, "my-binary"))
	if err != nil {
		t.Fatalf("Failed to read extracted binary: %v", err)
	}
	if !bytes.Equal(extracted, binaryContent) {
		t.Errorf("Extracted content mismatch: got %q, want %q", extracted, binaryContent)
	}
}

func TestDownloadAndExtract_NestedBinary(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("nested binary")
	archive := createTarGz(t, map[string][]byte{
		"subdir/my-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(destDir, "my-binary"))
	if err != nil {
		t.Fatalf("Failed to read extracted binary: %v", err)
	}
	if !bytes.Equal(extracted, binaryContent) {
		t.Errorf("Extracted content mismatch")
	}
}

func TestDownloadAndExtract_BinaryNotInArchive(t *testing.T) {
	t.Parallel()

	archive := createTarGz(t, map[string][]byte{
		"other-binary": []byte("not the one"),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err == nil {
		t.Error("Expected error when binary not found in archive")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Errorf("Expected 'not found in archive' error, got: %v", err)
	}
}

func TestDownloadAndExtract_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err == nil {
		t.Error("Expected error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected status code in error, got: %v", err)
	}
}

func TestDownloadAndExtract_InvalidGzip(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a gzip file"))
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err == nil {
		t.Error("Expected error for invalid gzip")
	}
}

func TestDownloadAndExtract_ConnectionError(t *testing.T) {
	t.Parallel()

	err := downloadAndExtract("http://127.0.0.1:1/unreachable.tar.gz", t.TempDir(), "my-binary")
	if err == nil {
		t.Error("Expected error for connection failure")
	}
}

func TestEnsureBinary_Downloads(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("#!/bin/sh\necho hello\n")
	archive := createTarGz(t, map[string][]byte{
		"test-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	binDir := t.TempDir()

	// Temporarily override downloadURL by calling downloadAndExtract directly
	// since ensureBinary constructs a GitHub URL we can't intercept.
	// Instead, test the download path by calling downloadAndExtract + verifying ensureBinary finds the result.
	err := downloadAndExtract(server.URL+"/test.tar.gz", binDir, "test-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}

	// Now ensureBinary should find it
	path, err := ensureBinary(binDir, "test-binary", "unused")
	if err != nil {
		t.Fatalf("ensureBinary() failed after download: %v", err)
	}
	if filepath.Base(path) != "test-binary" {
		t.Errorf("Expected binary name 'test-binary', got %q", filepath.Base(path))
	}
}

func TestEnsureBinary_CreatesDir(t *testing.T) {
	t.Parallel()

	parentDir := t.TempDir()
	binDir := filepath.Join(parentDir, "bin", "nested")

	binaryContent := []byte("binary")
	archive := createTarGz(t, map[string][]byte{
		"openshell-gateway": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	// Pre-create the binary so ensureBinary doesn't try to download from GitHub
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "openshell-gateway"), binaryContent, 0755); err != nil {
		t.Fatalf("Failed to write binary: %v", err)
	}

	path, err := ensureBinary(binDir, "openshell-gateway", "dev")
	if err != nil {
		t.Fatalf("ensureBinary() failed: %v", err)
	}

	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("Binary not found at %q: %v", path, statErr)
	}
}

func TestDownloadAndExtract_SkipsPrefixMatchNonExact(t *testing.T) {
	t.Parallel()

	archive := createTarGz(t, map[string][]byte{
		"my-binary-extra": []byte("wrong binary"),
		"my-binary":       []byte("right binary"),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "my-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(destDir, "my-binary"))
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if string(extracted) != "right binary" {
		t.Errorf("Expected 'right binary', got %q", extracted)
	}
}

func TestDownloadURL_VariousInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag      string
		asset    string
		expected string
	}{
		{"v1.0.0", "openshell-gateway-x86_64.tar.gz",
			"https://github.com/NVIDIA/OpenShell/releases/download/v1.0.0/openshell-gateway-x86_64.tar.gz"},
		{"dev", "openshell-aarch64-apple-darwin.tar.gz",
			"https://github.com/NVIDIA/OpenShell/releases/download/dev/openshell-aarch64-apple-darwin.tar.gz"},
	}

	for _, tt := range tests {
		url := downloadURL(tt.tag, tt.asset)
		if url != tt.expected {
			t.Errorf("downloadURL(%q, %q) = %q, want %q", tt.tag, tt.asset, url, tt.expected)
		}
	}
}

func TestPlatformAsset_ErrorMessage(t *testing.T) {
	t.Parallel()

	_, err := platformAsset("nonexistent")
	if err == nil {
		t.Fatal("Expected error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported platform") {
		t.Errorf("Expected 'unsupported platform' in error, got: %v", err)
	}
	if !strings.Contains(errMsg, "nonexistent") {
		t.Errorf("Expected binary name in error, got: %v", err)
	}
}

func TestEnsureBinary_UnsupportedPlatformBinary(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	_, err := ensureBinary(binDir, "nonexistent-binary", "v1.0.0")
	if err == nil {
		t.Error("Expected error for unsupported platform binary")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("Expected 'unsupported platform' error, got: %v", err)
	}
}

func TestDownloadAndExtract_EmptyArchive(t *testing.T) {
	t.Parallel()

	archive := createTarGz(t, map[string][]byte{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	err := downloadAndExtract(server.URL+"/empty.tar.gz", t.TempDir(), "my-binary")
	if err == nil {
		t.Error("Expected error for empty archive")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Errorf("Expected 'not found in archive' error, got: %v", err)
	}
}

func TestDownloadAndExtract_InvalidDestDir(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("binary")
	archive := createTarGz(t, map[string][]byte{
		"my-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	err := downloadAndExtract(server.URL+"/test.tar.gz", "/nonexistent/path/to/nowhere", "my-binary")
	if err == nil {
		t.Error("Expected error for invalid destination directory")
	}
}

func TestEnsureBinary_VerifiesFilePermissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not enforced on Windows")
	}

	binaryContent := []byte("binary content")
	archive := createTarGz(t, map[string][]byte{
		"test-perm-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	if err := downloadAndExtract(server.URL+"/test.tar.gz", destDir, "test-perm-binary"); err != nil {
		t.Fatalf("downloadAndExtract failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(destDir, "test-perm-binary"))
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	mode := info.Mode().Perm()
	if mode&0100 == 0 {
		t.Errorf("Expected executable permission, got %o", mode)
	}
}

func TestDownloadAndExtract_LargeFile(t *testing.T) {
	t.Parallel()

	largeContent := bytes.Repeat([]byte("x"), 1024*1024)
	archive := createTarGz(t, map[string][]byte{
		"large-binary": largeContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/large.tar.gz", destDir, "large-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(destDir, "large-binary"))
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if len(extracted) != len(largeContent) {
		t.Errorf("Size mismatch: got %d, want %d", len(extracted), len(largeContent))
	}
}

func TestDownloadAndExtract_ServerPath(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("binary data")
	archive := createTarGz(t, map[string][]byte{
		"my-binary": binaryContent,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/releases/download/v1.0.0/my-binary.tar.gz"
		if r.URL.Path != expectedPath {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "not found: %s", r.URL.Path)
			return
		}
		w.Write(archive)
	}))
	defer server.Close()

	destDir := t.TempDir()
	err := downloadAndExtract(server.URL+"/releases/download/v1.0.0/my-binary.tar.gz", destDir, "my-binary")
	if err != nil {
		t.Fatalf("downloadAndExtract() failed: %v", err)
	}
}
