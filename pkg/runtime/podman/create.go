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
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/devcontainers/features"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/podman/config"
	"github.com/openkaiden/kdn/pkg/runtime/podman/constants"
	"github.com/openkaiden/kdn/pkg/runtime/podman/pods"
	podmanSystem "github.com/openkaiden/kdn/pkg/runtime/podman/system"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

const defaultOnecliVersion = "1.17"

// podTemplateData holds the values used to render the pod YAML template
// and is also persisted as per-pod metadata (pod-template-data.json) so
// that Start() can recover SourcePath, ProjectID, Agent and ApprovalHandlerDir
// without re-reading the original CreateParams.
type podTemplateData struct {
	Name               string
	OnecliWebPort      int
	OnecliVersion      string
	AgentUID           int
	BaseImageRegistry  string
	BaseImageVersion   string
	SourcePath         string
	ProjectID          string
	Agent              string
	ApprovalHandlerDir string
}

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
// If featureInfos is non-empty, the features have already been downloaded to instanceDir/features/
// and the Containerfile will include instructions to install them.
func (p *podmanRuntime) createContainerfile(instanceDir string, imageConfig *config.ImageConfig, agentConfig *config.AgentConfig, settings map[string][]byte, featureInfos []featureInstallInfo) error {
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
	containerfileContent := generateContainerfile(imageConfig, agentConfig, len(settings) > 0, featureInfos)
	containerfilePath := filepath.Join(instanceDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte(containerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Containerfile: %w", err)
	}

	return nil
}

// prepareFeatures downloads, orders, and merges options for devcontainer features declared in params.
// Feature directories are written to instanceDir/features/{dirName}/.
// Returns nil if no features are configured.
func (p *podmanRuntime) prepareFeatures(ctx context.Context, instanceDir string, params runtime.CreateParams) ([]featureInstallInfo, error) {
	if params.WorkspaceConfig == nil || params.WorkspaceConfig.Features == nil || len(*params.WorkspaceConfig.Features) == 0 {
		return nil, nil
	}

	feats, userOpts, err := features.FromMap(*params.WorkspaceConfig.Features, params.WorkspaceConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse features: %w", err)
	}

	if len(feats) == 0 {
		return nil, nil
	}

	// Assign a stable directory name to each feature based on sorted order.
	dirNames := make(map[string]string, len(feats))
	for i, f := range feats {
		dirNames[f.ID()] = fmt.Sprintf("feature-%d", i)
	}

	// Create the features directory in the build context.
	featuresDir := filepath.Join(instanceDir, "features")
	if err := os.MkdirAll(featuresDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create features directory: %w", err)
	}

	// Download each feature into its designated subdirectory.
	metadata := make(map[string]features.FeatureMetadata, len(feats))
	for _, f := range feats {
		destDir := filepath.Join(featuresDir, dirNames[f.ID()])
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for feature %q: %w", f.ID(), err)
		}
		meta, err := f.Download(ctx, destDir)
		if err != nil {
			return nil, fmt.Errorf("failed to download feature %q: %w", f.ID(), err)
		}
		metadata[f.ID()] = meta
	}

	// Topologically sort features according to their installsAfter declarations.
	ordered, err := features.Order(feats, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to order features: %w", err)
	}

	// Build the install info slice in installation order.
	infos := make([]featureInstallInfo, 0, len(ordered))
	for _, f := range ordered {
		meta := metadata[f.ID()]

		mergedOpts, err := meta.Options().Merge(userOpts[f.ID()])
		if err != nil {
			return nil, fmt.Errorf("failed to merge options for feature %q: %w", f.ID(), err)
		}

		infos = append(infos, featureInstallInfo{
			dirName: dirNames[f.ID()],
			options: mergedOpts,
			envVars: meta.ContainerEnv(),
		})
	}

	return infos, nil
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

// containerConfigArgs holds optional OneCLI container configuration to inject into the workspace.
type containerConfigArgs struct {
	envVars         map[string]string
	caFilePath      string
	caContainerPath string
}

// buildContainerArgs builds the arguments for creating the workspace container inside the pod.
func (p *podmanRuntime) buildContainerArgs(params runtime.CreateParams, imageName string, ccArgs *containerConfigArgs) ([]string, error) {
	args := []string{"create", "--pod", params.Name, "--name", params.Name}

	// Collect workspace env var names for collision detection
	workspaceEnvNames := make(map[string]bool)
	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Environment != nil {
		for _, env := range *params.WorkspaceConfig.Environment {
			if env.Value != nil {
				args = append(args, "-e", fmt.Sprintf("%s=%s", env.Name, *env.Value))
				workspaceEnvNames[env.Name] = true
			} else if env.Secret != nil {
				secretArg := fmt.Sprintf("%s,type=env,target=%s", *env.Secret, env.Name)
				args = append(args, "--secret", secretArg)
				workspaceEnvNames[env.Name] = true
			}
		}
	}

	// Add OneCLI proxy env vars after workspace config (OneCLI takes precedence).
	// Log collisions so users know their workspace values are being overridden.
	if ccArgs != nil {
		onecliEnvNames := make(map[string]bool)
		for k, v := range ccArgs.envVars {
			if workspaceEnvNames[k] {
				fmt.Fprintf(os.Stderr, "warning: OneCLI overrides workspace env var %q\n", k)
			}
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
			onecliEnvNames[k] = true
		}
		// Ensure local addresses bypass the OneCLI proxy so tools can reach
		// localhost and host.containers.internal (e.g. Ollama) directly.
		// Only inject if neither the workspace config nor OneCLI already set NO_PROXY.
		if !workspaceEnvNames["NO_PROXY"] && !workspaceEnvNames["no_proxy"] &&
			!onecliEnvNames["NO_PROXY"] && !onecliEnvNames["no_proxy"] {
			const noProxy = "localhost,127.0.0.1,host.containers.internal"
			args = append(args, "-e", "NO_PROXY="+noProxy, "-e", "no_proxy="+noProxy)
		}
		if ccArgs.caFilePath != "" && ccArgs.caContainerPath != "" {
			args = append(args, "-v", fmt.Sprintf("%s:%s:ro,Z", ccArgs.caFilePath, ccArgs.caContainerPath))
		}
	}

	// Add secret service env vars with placeholder values so CLI tools detect a configured credential.
	for k, v := range params.SecretEnvVars {
		if !workspaceEnvNames[k] {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Mount the source directory at /workspace/sources
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

// findFreePorts returns n free TCP ports on 127.0.0.1.
// Each port is obtained by binding to :0 and immediately closing the listener.
func findFreePorts(n int) ([]int, error) {
	ports := make([]int, 0, n)
	for range n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to find free port: %w", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		ports = append(ports, port)
	}
	return ports, nil
}

// renderPodYAML renders the embedded pod YAML template with the given data.
func renderPodYAML(data podTemplateData) ([]byte, error) {
	tmpl, err := template.New("pod").Parse(string(pods.OnecliPodYAML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pod template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render pod template: %w", err)
	}
	return buf.Bytes(), nil
}

// Create creates a new Podman runtime instance.
// It uses kube play to create a pod with onecli services from the embedded YAML template,
// then adds the workspace container to the same pod.
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
	defer os.RemoveAll(instanceDir)

	// Load configurations
	imageConfig, err := p.config.LoadImage()
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to load image config: %w", err)
	}

	agentConfig, err := p.config.LoadAgent(params.Agent)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to load agent config: %w", err)
	}

	// Prepare devcontainer features: download, order, and merge options.
	// Only emits a step when features are actually configured.
	var featureInfos []featureInstallInfo
	if params.WorkspaceConfig != nil && params.WorkspaceConfig.Features != nil && len(*params.WorkspaceConfig.Features) > 0 {
		stepLogger.Start("Downloading devcontainer features", "Devcontainer features downloaded")
		var featErr error
		featureInfos, featErr = p.prepareFeatures(ctx, instanceDir, params)
		if featErr != nil {
			stepLogger.Fail(featErr)
			return runtime.RuntimeInfo{}, featErr
		}
	}

	// Create Containerfile
	stepLogger.Start("Generating Containerfile", "Containerfile generated")
	if err := p.createContainerfile(instanceDir, imageConfig, agentConfig, params.AgentSettings, featureInfos); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Build image
	imageName := fmt.Sprintf("kdn-%s", params.Name)
	stepLogger.Start(fmt.Sprintf("Building container image: %s", imageName), "Container image built")
	if err := p.buildImage(ctx, imageName, instanceDir); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Allocate random free ports for the pod
	freePorts, err := findFreePorts(1)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to allocate free ports: %w", err)
	}

	// Prepare the approval-handler directory with the embedded Node.js script
	// so it is available as a hostPath volume when the pod is created.
	approvalHandlerDir := filepath.Join(p.storageDir, "approval-handler", params.Name)
	if err := writeApprovalHandlerFiles(approvalHandlerDir); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to write approval handler files: %w", err)
	}

	// Render the pod YAML template
	tmplData := podTemplateData{
		Name:               params.Name,
		OnecliWebPort:      freePorts[0],
		OnecliVersion:      defaultOnecliVersion,
		AgentUID:           p.system.Getuid(),
		BaseImageRegistry:  constants.BaseImageRegistry,
		BaseImageVersion:   imageConfig.Version,
		SourcePath:         params.SourcePath,
		ProjectID:          params.ProjectID,
		Agent:              params.Agent,
		ApprovalHandlerDir: podmanSystem.HostPathToMachinePath(approvalHandlerDir),
	}

	tmpPodDir := filepath.Join(instanceDir, "pod")
	if err := os.MkdirAll(tmpPodDir, 0755); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to create temp pod directory: %w", err)
	}
	tmpYAMLPath := filepath.Join(tmpPodDir, podYAMLFile)
	if err := writePodYAMLFile(tmpYAMLPath, tmplData); err != nil {
		return runtime.RuntimeInfo{}, err
	}

	// Create the pod with onecli services via kube play (--start=false keeps all containers stopped)
	stepLogger.Start("Creating onecli services", "Onecli services created")
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "kube", "play", "--userns=keep-id", "--start=false", tmpYAMLPath); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to create pod via kube play: %w", err)
	}

	// Clean up the pod if any subsequent step fails.
	// Use context.Background() so cleanup still runs if ctx is cancelled.
	podCreatedOK := false
	defer func() {
		if !podCreatedOK {
			_ = p.executor.Run(context.Background(), l.Stdout(), l.Stderr(), "pod", "rm", "-f", params.Name)
		}
	}()

	// Always start OneCLI to inject proxy env vars and the CA cert into the workspace container.
	// Without HTTP_PROXY/HTTPS_PROXY pointing at the OneCLI gateway, deny-mode networking rules
	// have no effect because workspace traffic bypasses the proxy entirely.
	// Secrets are provisioned if any were provided.
	containerConfig, setupErr := p.setupOnecli(ctx, stepLogger, l, params.Name, tmplData, params.OnecliSecrets)
	if setupErr != nil {
		return runtime.RuntimeInfo{}, setupErr
	}
	var ccArgs *containerConfigArgs
	if containerConfig != nil {
		ccArgs = &containerConfigArgs{
			envVars: containerConfig.Env,
		}
		// Write CA certificate to a durable location for mounting into the workspace container.
		// Use a shared certs directory under storageDir (not instanceDir which is cleaned up).
		if containerConfig.CACertificate != "" && containerConfig.CACertificateContainerPath != "" {
			certsDir := filepath.Join(p.storageDir, "certs", params.Name)
			if mkErr := os.MkdirAll(certsDir, 0755); mkErr != nil {
				return runtime.RuntimeInfo{}, fmt.Errorf("failed to create certs directory: %w", mkErr)
			}
			caPath := filepath.Join(certsDir, "onecli-ca.pem")
			if writeErr := os.WriteFile(caPath, []byte(containerConfig.CACertificate), 0644); writeErr != nil {
				return runtime.RuntimeInfo{}, fmt.Errorf("failed to write CA certificate: %w", writeErr)
			}
			ccArgs.caFilePath = caPath
			ccArgs.caContainerPath = containerConfig.CACertificateContainerPath
		}
	}

	// Build workspace container args with proxy env vars and CA cert mount from OneCLI
	stepLogger.Start(fmt.Sprintf("Creating workspace container: %s", params.Name), "Workspace container created")
	createArgs, err := p.buildContainerArgs(params, imageName, ccArgs)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	containerID, err := p.createContainer(ctx, createArgs)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, err
	}

	// Persist pod files keyed by the workspace container ID
	if err := p.writePodFiles(containerID, tmplData); err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist pod files: %w", err)
	}
	if ccArgs != nil && ccArgs.caContainerPath != "" {
		if err := p.writeCAContainerPath(containerID, ccArgs.caContainerPath); err != nil {
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist CA container path: %w", err)
		}
	}

	podCreatedOK = true

	// Return RuntimeInfo
	info := map[string]string{
		"container_id":    containerID,
		"image_name":      imageName,
		"source_path":     params.SourcePath,
		"agent":           params.Agent,
		"onecli_web_port": fmt.Sprintf("%d", tmplData.OnecliWebPort),
	}
	if ccArgs != nil && ccArgs.caContainerPath != "" {
		info["ca_container_path"] = ccArgs.caContainerPath
	}

	return runtime.RuntimeInfo{
		ID:    containerID,
		State: api.WorkspaceStateStopped,
		Info:  info,
	}, nil
}

// setupOnecli starts postgres, waits for it, then starts onecli to avoid migration race conditions.
// After provisioning secrets and retrieving container config, it stops the pod.
func (p *podmanRuntime) setupOnecli(ctx context.Context, stepLogger steplogger.StepLogger, l logger.Logger, podName string, tmplData podTemplateData, secrets []onecli.CreateSecretInput) (*onecli.ContainerConfig, error) {
	postgresContainer := podName + "-postgres"
	onecliContainer := podName + "-onecli"

	// Start only the postgres container first
	stepLogger.Start("Starting postgres", "Postgres started")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", postgresContainer); err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("failed to start postgres: %w", err)
	}

	// Wait for postgres to be ready before starting onecli
	stepLogger.Start("Waiting for postgres readiness", "Postgres ready")
	if err := p.waitForPostgres(ctx, podName); err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("postgres not ready: %w", err)
	}

	// Now start the onecli container (postgres is ready, migrations will succeed)
	stepLogger.Start("Starting OneCLI", "OneCLI started")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", onecliContainer); err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("failed to start OneCLI: %w", err)
	}

	baseURL := p.onecliURL(tmplData.OnecliWebPort)

	stepLogger.Start("Waiting for OneCLI readiness", "OneCLI ready")
	if err := waitForReady(ctx, baseURL); err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("OneCLI service not ready: %w", err)
	}

	// Get API key from OneCLI (bootstraps local user on first call)
	creds := onecli.NewCredentialProvider(baseURL)
	apiKey, err := creds.APIKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OneCLI API key: %w", err)
	}

	client := onecli.NewClient(baseURL, apiKey)

	// Provision secrets (skipped when none are configured)
	if len(secrets) > 0 {
		stepLogger.Start("Provisioning OneCLI secrets", "OneCLI secrets provisioned")
		provisioner := onecli.NewSecretProvisioner(client)
		if err := provisioner.ProvisionSecrets(ctx, secrets); err != nil {
			stepLogger.Fail(err)
			return nil, fmt.Errorf("failed to provision OneCLI secrets: %w", err)
		}
	}

	// Get container config (proxy env vars, CA cert, agent token)
	stepLogger.Start("Retrieving OneCLI container config", "Container config retrieved")
	containerConfig, err := client.GetContainerConfig(ctx)
	if err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("failed to get container config: %w", err)
	}

	// Stop the pod before creating the workspace container
	stepLogger.Start("Stopping OneCLI services", "OneCLI services stopped")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "pod", "stop", podName); err != nil {
		stepLogger.Fail(err)
		return nil, fmt.Errorf("failed to stop pod after OneCLI setup: %w", err)
	}

	return containerConfig, nil
}

// writeApprovalHandlerFiles writes the embedded Node.js approval handler
// script and package.json into the given directory so it can be mounted
// as a hostPath volume into the approval-handler sidecar container.
func writeApprovalHandlerFiles(dir string) error {
	if err := os.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("failed to create approval handler directory: %w", err)
	}
	// MkdirAll is subject to umask, so explicitly set permissions to allow
	// the non-root container user (UID 1001) to write node_modules/.
	if err := os.Chmod(dir, 0777); err != nil {
		return fmt.Errorf("failed to chmod approval handler directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "approval-handler.ts"), pods.ApprovalHandlerTS, 0644); err != nil {
		return fmt.Errorf("failed to write approval-handler.ts: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pods.ApprovalHandlerPackageJSON, 0644); err != nil {
		return fmt.Errorf("failed to write package.json: %w", err)
	}
	return nil
}

// writePodYAMLFile renders and writes the pod YAML template to the given path.
func writePodYAMLFile(path string, data podTemplateData) error {
	content, err := renderPodYAML(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write pod YAML: %w", err)
	}
	return nil
}
