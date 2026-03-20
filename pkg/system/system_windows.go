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

//go:build windows

package system

import (
	"os/exec"
	"strconv"
	"strings"
)

// Getuid returns the numeric user ID of the caller.
// On Windows, this queries WSL2 for the actual UID and falls back to 1000 if unavailable.
func (s *systemImpl) Getuid() int {
	// Try to get UID from WSL2
	cmd := exec.Command("wsl", "id", "-u")
	output, err := cmd.Output()
	if err == nil {
		if uid, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			return uid
		}
	}
	// Fallback to 1000 (common Linux container default)
	return 1000
}

// Getgid returns the numeric group ID of the caller.
// On Windows, this queries WSL2 for the actual GID and falls back to 1000 if unavailable.
func (s *systemImpl) Getgid() int {
	// Try to get GID from WSL2
	cmd := exec.Command("wsl", "id", "-g")
	output, err := cmd.Output()
	if err == nil {
		if gid, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			return gid
		}
	}
	// Fallback to 1000 (common Linux container default)
	return 1000
}
