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
	"os"
	osexec "os/exec"
	"runtime"
)

// macosPodmanPaths lists well-known install locations for podman on macOS,
// searched in order when the executable is not found in PATH.
var macosPodmanPaths = []string{
	"/opt/podman/bin/podman",
	"/usr/local/bin/podman",
	"/opt/homebrew/bin/podman",
}

// findPodmanCLI resolves the full path to the podman executable.
// It first searches PATH. On macOS, if not found there, it checks a list of
// well-known locations. On other platforms, nil is returned when not in PATH.
func findPodmanCLI() *string {
	if path, err := osexec.LookPath("podman"); err == nil {
		return &path
	}
	if runtime.GOOS == "darwin" {
		for _, candidate := range macosPodmanPaths {
			if _, err := os.Stat(candidate); err == nil {
				p := candidate
				return &p
			}
		}
	}
	return nil
}
