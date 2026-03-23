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
	"strings"
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/config"
)

func TestGenerateSudoers(t *testing.T) {
	t.Parallel()

	t.Run("generates sudoers with single ALLOWED alias", func(t *testing.T) {
		t.Parallel()

		sudoBinaries := []string{"/usr/bin/dnf", "/bin/kill", "/usr/bin/killall"}
		result := generateSudoers(sudoBinaries)

		// Check for ALLOWED alias
		if !strings.Contains(result, "Cmnd_Alias ALLOWED =") {
			t.Error("Expected sudoers to contain 'Cmnd_Alias ALLOWED ='")
		}

		// Check that all binaries are listed
		for _, binary := range sudoBinaries {
			if !strings.Contains(result, binary) {
				t.Errorf("Expected sudoers to contain %s", binary)
			}
		}

		// Check for the sudo rule
		if !strings.Contains(result, "claude ALL = !ALL, NOPASSWD: ALLOWED") {
			t.Error("Expected sudoers to contain correct sudo rule")
		}
	})

	t.Run("generates no-access sudoers when no binaries provided", func(t *testing.T) {
		t.Parallel()

		result := generateSudoers([]string{})

		// Should only have the deny-all rule
		if !strings.Contains(result, "claude ALL = !ALL") {
			t.Error("Expected sudoers to contain 'claude ALL = !ALL'")
		}

		// Should not have ALLOWED alias
		if strings.Contains(result, "ALLOWED") {
			t.Error("Expected sudoers to not contain ALLOWED alias when no binaries provided")
		}
	})

	t.Run("joins multiple binaries with comma separator", func(t *testing.T) {
		t.Parallel()

		sudoBinaries := []string{"/usr/bin/dnf", "/bin/kill"}
		result := generateSudoers(sudoBinaries)

		// Check for comma-separated list
		if !strings.Contains(result, "/usr/bin/dnf, /bin/kill") {
			t.Error("Expected binaries to be comma-separated")
		}
	})
}

func TestGenerateContainerfile(t *testing.T) {
	t.Parallel()

	t.Run("generates containerfile with default configs", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{"which", "procps-ng"},
			Sudo:        []string{"/usr/bin/dnf"},
			RunCommands: []string{},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{"curl -fsSL https://claude.ai/install.sh | bash"},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		// Check for FROM line with correct base image
		expectedFrom := "FROM registry.fedoraproject.org/fedora:latest"
		if !strings.Contains(result, expectedFrom) {
			t.Errorf("Expected FROM line: %s", expectedFrom)
		}

		// Check for package installation
		if !strings.Contains(result, "RUN dnf install -y which procps-ng") {
			t.Error("Expected package installation line")
		}

		// Check for user/group setup
		if !strings.Contains(result, "ARG UID=1000") {
			t.Error("Expected UID argument")
		}
		if !strings.Contains(result, "ARG GID=1000") {
			t.Error("Expected GID argument")
		}
		if !strings.Contains(result, "USER claude:claude") {
			t.Error("Expected USER line")
		}

		// Check for sudoers copy
		if !strings.Contains(result, "COPY sudoers /etc/sudoers.d/claude") {
			t.Error("Expected COPY sudoers line")
		}

		// Check for sudoers chmod
		if !strings.Contains(result, "RUN chmod 0440 /etc/sudoers.d/claude") {
			t.Error("Expected RUN chmod for sudoers")
		}

		// Check for PATH environment
		if !strings.Contains(result, "ENV PATH=/home/claude/.local/bin:/usr/local/bin:/usr/bin") {
			t.Error("Expected PATH environment variable")
		}

		// Check for Containerfile copy
		if !strings.Contains(result, "COPY Containerfile /home/claude/Containerfile") {
			t.Error("Expected COPY Containerfile line")
		}

		// Check for agent RUN commands
		if !strings.Contains(result, "RUN curl -fsSL https://claude.ai/install.sh | bash") {
			t.Error("Expected agent RUN command")
		}
	})

	t.Run("uses custom fedora version", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:  "40",
			Packages: []string{},
			Sudo:     []string{},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		expectedFrom := "FROM registry.fedoraproject.org/fedora:40"
		if !strings.Contains(result, expectedFrom) {
			t.Errorf("Expected FROM line with custom version: %s", expectedFrom)
		}
	})

	t.Run("merges packages from image and agent configs", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:  "latest",
			Packages: []string{"package1", "package2"},
			Sudo:     []string{},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{"package3", "package4"},
			RunCommands:     []string{},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		// Should have all packages in a single RUN command
		if !strings.Contains(result, "RUN dnf install -y package1 package2 package3 package4") {
			t.Error("Expected merged package installation with all packages")
		}
	})

	t.Run("omits package installation when no packages", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:  "latest",
			Packages: []string{},
			Sudo:     []string{},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		// Should not have dnf install line
		if strings.Contains(result, "dnf install") {
			t.Error("Expected no dnf install line when no packages specified")
		}
	})

	t.Run("includes custom RUN commands from both configs", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{},
			Sudo:        []string{},
			RunCommands: []string{"echo 'image setup'"},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{"echo 'agent setup'"},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		// Should have both RUN commands
		if !strings.Contains(result, "RUN echo 'image setup'") {
			t.Error("Expected image RUN command")
		}
		if !strings.Contains(result, "RUN echo 'agent setup'") {
			t.Error("Expected agent RUN command")
		}
	})

	t.Run("image RUN commands come before agent RUN commands", func(t *testing.T) {
		t.Parallel()

		imageConfig := &config.ImageConfig{
			Version:     "latest",
			Packages:    []string{},
			Sudo:        []string{},
			RunCommands: []string{"echo 'image'"},
		}

		agentConfig := &config.AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{"echo 'agent'"},
			TerminalCommand: []string{"claude"},
		}

		result := generateContainerfile(imageConfig, agentConfig)

		// Find positions
		imagePos := strings.Index(result, "RUN echo 'image'")
		agentPos := strings.Index(result, "RUN echo 'agent'")

		if imagePos == -1 || agentPos == -1 {
			t.Fatal("Both RUN commands should be present")
		}

		if imagePos > agentPos {
			t.Error("Image RUN commands should come before agent RUN commands")
		}
	})
}
