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
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

const (
	openshellGatewayRelease  = "dev"
	openshellRelease         = "dev"
	openshellDriverVMRelease = "vm-dev"
	githubRepo               = "NVIDIA/OpenShell"
)

// platformAsset returns the asset name for the current platform.
func platformAsset(binary string) (string, error) {
	os := goruntime.GOOS
	arch := goruntime.GOARCH

	switch binary {
	case "openshell-gateway":
		switch {
		case os == "darwin" && arch == "arm64":
			return "openshell-gateway-aarch64-apple-darwin.tar.gz", nil
		case os == "linux" && arch == "arm64":
			return "openshell-gateway-aarch64-unknown-linux-gnu.tar.gz", nil
		case os == "linux" && arch == "amd64":
			return "openshell-gateway-x86_64-unknown-linux-gnu.tar.gz", nil
		}
	case "openshell":
		switch {
		case os == "darwin" && arch == "arm64":
			return "openshell-aarch64-apple-darwin.tar.gz", nil
		case os == "linux" && arch == "arm64":
			return "openshell-aarch64-unknown-linux-musl.tar.gz", nil
		case os == "linux" && arch == "amd64":
			return "openshell-x86_64-unknown-linux-musl.tar.gz", nil
		}
	case "openshell-driver-vm":
		switch {
		case os == "darwin" && arch == "arm64":
			return "openshell-driver-vm-aarch64-apple-darwin.tar.gz", nil
		case os == "linux" && arch == "arm64":
			return "openshell-driver-vm-aarch64-unknown-linux-gnu.tar.gz", nil
		case os == "linux" && arch == "amd64":
			return "openshell-driver-vm-x86_64-unknown-linux-gnu.tar.gz", nil
		}
	}

	return "", fmt.Errorf("unsupported platform %s/%s for binary %s", os, arch, binary)
}

// downloadURL returns the GitHub release download URL for an asset.
func downloadURL(tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, asset)
}

// ensureBinary downloads a binary if it doesn't exist in the bin directory.
func ensureBinary(binDir, binary, releaseTag string) (string, error) {
	binaryPath := filepath.Join(binDir, binary)

	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	asset, err := platformAsset(binary)
	if err != nil {
		return "", err
	}

	url := downloadURL(releaseTag, asset)
	if err := downloadAndExtract(url, binDir, binary); err != nil {
		return "", fmt.Errorf("failed to download %s: %w", binary, err)
	}

	return binaryPath, nil
}

// downloadAndExtract downloads a tar.gz from the URL and extracts the named binary.
func downloadAndExtract(url, destDir, binaryName string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d for %s", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Extract only the target binary (may be nested in a directory)
		name := filepath.Base(header.Name)
		if header.Typeflag != tar.TypeReg || !strings.HasPrefix(name, binaryName) {
			continue
		}
		if name != binaryName {
			continue
		}

		destPath := filepath.Join(destDir, binaryName)
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", destPath, err)
		}

		if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
			out.Close()
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}
		out.Close()
		return nil
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}
