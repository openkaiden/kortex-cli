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
	"fmt"
	"path"
	"path/filepath"
	"strings"

	workspace "github.com/kortex-hub/kortex-cli-api/workspace-configuration/go"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/constants"
)

// containerWorkspaceSources is the mount point for the sources directory inside the container.
var containerWorkspaceSources = path.Join("/workspace", "sources")

// containerHome is the home directory of the container user.
var containerHome = path.Join("/home", constants.ContainerUser)

// resolveHostPath expands $SOURCES and $HOME variables in a mount host path
// and returns the absolute host filesystem path.
func resolveHostPath(host, sourcesDir, homeDir string) string {
	switch {
	case strings.HasPrefix(host, "$SOURCES"):
		rest := filepath.FromSlash(host[len("$SOURCES"):])
		return filepath.Join(sourcesDir, rest)
	case strings.HasPrefix(host, "$HOME"):
		rest := filepath.FromSlash(host[len("$HOME"):])
		return filepath.Join(homeDir, rest)
	default:
		return filepath.Clean(host)
	}
}

// resolveTargetPath expands $SOURCES and $HOME variables in a mount target path
// and returns the absolute container filesystem path.
func resolveTargetPath(target string) string {
	switch {
	case strings.HasPrefix(target, "$SOURCES"):
		return path.Join(containerWorkspaceSources, target[len("$SOURCES"):])
	case strings.HasPrefix(target, "$HOME"):
		return path.Join(containerHome, target[len("$HOME"):])
	default:
		return path.Clean(target)
	}
}

// isWithinRoot returns true if resolved is equal to root or is a direct descendant.
func isWithinRoot(resolved, root string) bool {
	suffix := strings.TrimPrefix(resolved, root)
	// TrimPrefix returns the original string unchanged when the prefix is absent,
	// so suffix == resolved means resolved does not start with root.
	return suffix != resolved && (suffix == "" || strings.HasPrefix(suffix, "/"))
}

// validateMount checks that a mount's resolved target path does not escape its
// expected root directory inside the container.
//
// Rules:
//   - $SOURCES-based targets must remain within /workspace
//   - $HOME-based targets must remain within /home/<containerUser>
//   - Absolute targets are accepted as-is
func validateMount(m workspace.Mount) error {
	if strings.HasPrefix(m.Target, "$SOURCES") {
		if !isWithinRoot(resolveTargetPath(m.Target), "/workspace") {
			return fmt.Errorf("mount target %q escapes above /workspace", m.Target)
		}
	} else if strings.HasPrefix(m.Target, "$HOME") {
		if !isWithinRoot(resolveTargetPath(m.Target), containerHome) {
			return fmt.Errorf("mount target %q escapes above %s", m.Target, containerHome)
		}
	}
	return nil
}

// mountVolumeArg builds the podman -v argument string for a mount.
// The returned string has the form "hostPath:targetPath:options".
func mountVolumeArg(m workspace.Mount, sourcesDir, homeDir string) string {
	hostPath := resolveHostPath(m.Host, sourcesDir, homeDir)
	targetPath := resolveTargetPath(m.Target)

	options := "Z"
	if m.Ro != nil && *m.Ro {
		options = "ro,Z"
	}

	return fmt.Sprintf("%s:%s:%s", hostPath, targetPath, options)
}
