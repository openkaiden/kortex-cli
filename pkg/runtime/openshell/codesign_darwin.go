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
	"fmt"
	"os"
	"os/exec"
)

const entitlementsPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.hypervisor</key>
    <true/>
</dict>
</plist>`

const codesignedSuffix = ".codesigned"

// codesignBinary signs the binary with the hypervisor entitlement on macOS.
// A marker file prevents re-signing on subsequent runs.
func codesignBinary(binaryPath string) error {
	markerPath := binaryPath + codesignedSuffix
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	}

	tmpFile, err := os.CreateTemp("", "kdn-entitlements-*.plist")
	if err != nil {
		return fmt.Errorf("failed to create entitlements file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(entitlementsPlist); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write entitlements: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("codesign", "--entitlements", tmpFile.Name(), "--force", "-s", "-", binaryPath) //nolint:gosec
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("codesign failed: %s: %w", string(output), err)
	}

	return os.WriteFile(markerPath, nil, 0644)
}
