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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/logger"
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman/config"
	"github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

// validateCreateParams validates the create parameters.
func (p *podmanRuntime) validateCreateParams(params runtime.CreateParams) error {
	if params.Name == "" {
		return fmt.Errorf("%w: name is required", runtime.ErrInvalidParams)
	}
	if params.SourcePath == "" {
		return fmt.Errorf("%w: source path is required", runtime.ErrInvalidParams)
	}
	if params.Agent == "" {
		return fmt.Errorf("%w: agent is required", runtime.ErrInvalidParams)
	}

	return nil
}

// createInstanceDirectory creates the working directory for a new instance.
func (p *podmanRuntime) createInstanceDirectory(name string) (string, error) {
	instanceDir := filepath.Join(p.storageDir, "instances", name)
	if err := os.MkdirAll(instanceDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create instance directory: %w", err)
	}
	return instanceDir, nil
}

// createContainerfile creates a Containerfile in the instance directory using the provided configs.
// If settings is non-empty, the files are written to an agent-settings/ subdirectory of instanceDir
// so they can be embedded in the image via a COPY instruction.
func (p *podmanRuntime) createContainerfile(instanceDir string, imageConfig *config.ImageConfig, agentConfig *config.AgentConfig, settings map[string][]byte) error {
	// Generate sudoers content
	sudoersContent := generateSudoers(imageConfig.Sudo)
	sudoersPath := filepath.Join(instanceDir, "sudoers")
	if err := os.WriteFile(sudoersPath, []byte(sudoersContent), 0644); err != nil {
		return fmt.Errorf("failed to write sudoers: %w", err)
	}

	// Write agent settings files to the build context if provided
	if len(settings) > 0 {
		settingsDir := filepath.Join(instanceDir, "agent-settings")
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agent settings dir: %w", err)
		}
		for relPath, content := range settings {
			destPath := filepath.Join(settingsDir, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", relPath, err)
			}
			if err := os.WriteFile(destPath, content, 0600); err != nil {
				return fmt.Errorf("failed to write agent settings file %s: %w", relPath, err)
			}
		}
	}

	// Generate Containerfile content
	containerfileContent := generateContainerfile(imageConfig, agentConfig, len(settings) > 0)
	containerfilePath := filepath.Join(instanceDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte(containerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Containerfile: %w", err)
	}

	return nil
}

// buildImage builds a podman image for the instance.
func (p *podmanRuntime) buildImage(ctx context.Context, imageName, instanceDir string) error {
	containerfilePath := filepath.Join(instanceDir, "Containerfile")

	// Get current user's UID and GID
	uid := p.system.Getuid()
	gid := p.system.Getgid()

	args := []string{
		"build",
		"--build-arg", fmt.Sprintf("UID=%d", uid),
		"--build-arg", fmt.Sprintf("GID=%d", gid),
		"-t", imageName,
		"-f", containerfilePath,
		instanceDir,
	}

	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), args...); err != nil {
		return fmt.Errorf("failed to build podman image: %w", err)
	}
	return nil
}

// buildContainerArgs builds the arguments for creating a podman container.
func (p *podmanRuntime) buildContainerArgs(params runtime.CreateParams, imageName string) ([]string, error) {
	args := []string{"create", "--name", params.Name}

	// Add environment variables from workspace config
	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Environment != nil {
		for _, env := range *params.WorkspaceConfig.Environment {
			if env.Value != nil {
				// Regular environment variable with a value
				args = append(args, "-e", fmt.Sprintf("%s=%s", env.Name, *env.Value))
			} else if env.Secret != nil {
				// Secret reference - use podman --secret flag
				// Format: --secret <secret-name>,type=env,target=<ENV_VAR_NAME>
				secretArg := fmt.Sprintf("%s,type=env,target=%s", *env.Secret, env.Name)
				args = append(args, "--secret", secretArg)
			}
		}
	}

	// Mount the source directory at /workspace/sources
	// This allows symlinks to work correctly with dependencies
	args = append(args, "-v", fmt.Sprintf("%s:/workspace/sources:Z", params.SourcePath))

	// Mount additional directories if specified
	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Mounts != nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		for _, m := range *params.WorkspaceConfig.Mounts {
			args = append(args, "-v", mountVolumeArg(m, params.SourcePath, homeDir))
		}
	}

	// Set working directory to /workspace/sources
	args = append(args, "-w", "/workspace/sources")

	// Add the image name
	args = append(args, imageName)

	// Add a default command to keep the container running
	args = append(args, "sleep", "infinity")

	return args, nil
}

// createContainer creates a podman container and returns its ID.
func (p *podmanRuntime) createContainer(ctx context.Context, args []string) (string, error) {
	l := logger.FromContext(ctx)
	output, err := p.executor.Output(ctx, l.Stderr(), args...)
	if err != nil {
		return "", fmt.Errorf("failed to create podman container: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Create creates a new Podman runtime instance.
func (p *podmanRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	// Validate parameters
	if err := p.validateCreateParams(params); err != nil {
		return runtime.RuntimeInfo{}, err
	}

	// Create instance directory
	stepLogger.Start("Creating temporary build directory", "Temporary build directory created")
	instanceDir, err := p.createInstanceDirectory(params.Name)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}
	// Clean up instance directory after use (whether success or error)
	// The Containerfile and sudoers are only needed during image build
	defer os.RemoveAll(instanceDir)

	// Load configurations
	imageConfig, err := p.config.LoadImage()
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to load image config: %w", err)
	}

	// Load agent configuration using the agent name from params
	agentConfig, err := p.config.LoadAgent(params.Agent)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to load agent config: %w", err)
	}

	// Create Containerfile
	stepLogger.Start("Generating Containerfile", "Containerfile generated")
	if err := p.createContainerfile(instanceDir, imageConfig, agentConfig, params.AgentSettings); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Build image
	imageName := fmt.Sprintf("kortex-cli-%s", params.Name)
	stepLogger.Start(fmt.Sprintf("Building container image: %s", imageName), "Container image built")
	if err := p.buildImage(ctx, imageName, instanceDir); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Build container creation arguments
	createArgs, err := p.buildContainerArgs(params, imageName)
	if err != nil {
		return runtime.RuntimeInfo{}, err
	}

	// Create container and get its ID directly from podman create output
	stepLogger.Start(fmt.Sprintf("Creating container: %s", params.Name), "Container created")
	containerID, err := p.createContainer(ctx, createArgs)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Return RuntimeInfo
	info := map[string]string{
		"container_id": containerID,
		"image_name":   imageName,
		"source_path":  params.SourcePath,
		"agent":        params.Agent,
	}

	return runtime.RuntimeInfo{
		ID:    containerID,
		State: api.WorkspaceStateStopped,
		Info:  info,
	}, nil
}
