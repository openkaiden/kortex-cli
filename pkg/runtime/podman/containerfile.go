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
	"strings"

	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/config"
)

const (
	// BaseImageRegistry is the hardcoded base image registry
	BaseImageRegistry = "registry.fedoraproject.org/fedora"

	// ContainerUser is the hardcoded container user
	ContainerUser = "claude"

	// ContainerGroup is the hardcoded container group
	ContainerGroup = "claude"
)

// generateSudoers generates the sudoers file content from a list of allowed binaries.
// It creates a single ALLOWED command alias and sets up sudo rules.
func generateSudoers(sudoBinaries []string) string {
	if len(sudoBinaries) == 0 {
		// No sudo access if no binaries are specified
		return fmt.Sprintf("%s ALL = !ALL\n", ContainerUser)
	}

	var lines []string

	// Create single ALLOWED command alias
	lines = append(lines, fmt.Sprintf("Cmnd_Alias ALLOWED = %s", strings.Join(sudoBinaries, ", ")))
	lines = append(lines, "")

	// Create sudo rule
	lines = append(lines, fmt.Sprintf("%s ALL = !ALL, NOPASSWD: ALLOWED", ContainerUser))

	return strings.Join(lines, "\n") + "\n"
}

// generateContainerfile generates the Containerfile content from image and agent configurations.
func generateContainerfile(imageConfig *config.ImageConfig, agentConfig *config.AgentConfig) string {
	if imageConfig == nil {
		return ""
	}
	if agentConfig == nil {
		return ""
	}
		
	var lines []string

	// FROM line with base image
	baseImage := fmt.Sprintf("%s:%s", BaseImageRegistry, imageConfig.Version)
	lines = append(lines, fmt.Sprintf("FROM %s", baseImage))
	lines = append(lines, "")

	// Merge packages from image and agent configs
	allPackages := append([]string{}, imageConfig.Packages...)
	allPackages = append(allPackages, agentConfig.Packages...)

	// Install packages if any
	if len(allPackages) > 0 {
		lines = append(lines, fmt.Sprintf("RUN dnf install -y %s", strings.Join(allPackages, " ")))
		lines = append(lines, "")
	}

	// User and group setup (hardcoded)
	lines = append(lines, "ARG UID=1000")
	lines = append(lines, "ARG GID=1000")
	lines = append(lines, `RUN GROUPNAME=$(grep $GID /etc/group | cut -d: -f1); [ -n "$GROUPNAME" ] && groupdel $GROUPNAME || true`)
	lines = append(lines, fmt.Sprintf(`RUN groupadd -g "${GID}" %s && useradd -u "${UID}" -g "${GID}" -m %s`, ContainerGroup, ContainerUser))
	lines = append(lines, "COPY sudoers /etc/sudoers.d/claude")
	lines = append(lines, fmt.Sprintf("USER %s:%s", ContainerUser, ContainerGroup))
	lines = append(lines, "")

	// Environment PATH
	lines = append(lines, fmt.Sprintf("ENV PATH=/home/%s/.local/bin:/usr/local/bin:/usr/bin", ContainerUser))

	// Custom RUN commands from image config
	for _, cmd := range imageConfig.RunCommands {
		lines = append(lines, fmt.Sprintf("RUN %s", cmd))
	}

	// Custom RUN commands from agent config
	for _, cmd := range agentConfig.RunCommands {
		lines = append(lines, fmt.Sprintf("RUN %s", cmd))
	}

	// Add final newline if there are RUN commands
	if len(imageConfig.RunCommands) > 0 || len(agentConfig.RunCommands) > 0 {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
