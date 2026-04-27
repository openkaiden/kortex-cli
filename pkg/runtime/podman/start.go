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
	"io"
	"net/http"
	"time"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

const (
	postgresMaxRetries    = 30
	postgresRetryInterval = 2 * time.Second
)

// Start starts all containers in the workspace pod.
// Postgres and onecli are started individually first. When the workspace is
// configured with mode: deny and at least one allowed host, networking rules and
// the approval-handler config are set up before the remaining containers are
// started. In all other cases (allow, no config, deny without hosts) any stale
// rules from a previous deny-mode start are cleared before the pod is started.
func (p *podmanRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	if id == "" {
		return runtime.RuntimeInfo{}, fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Resolve the pod name from the stored mapping
	podName, err := p.readPodName(id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to resolve pod name: %w", err)
	}

	l := logger.FromContext(ctx)

	// Start the postgres container first so it is accepting connections
	// before onecli attempts its database migration.
	postgresContainer := podName + "-postgres"
	stepLogger.Start("Starting postgres", "Postgres started")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", postgresContainer); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Wait until postgres is accepting connections
	stepLogger.Start("Waiting for postgres to be ready", "Postgres is ready")
	if err := p.waitForPostgres(ctx, podName); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("postgres did not become ready: %w", err)
	}

	// Start onecli individually so we can configure networking rules and write
	// the approval-handler config BEFORE the approval-handler container starts.
	onecliContainer := podName + "-onecli"
	stepLogger.Start("Starting OneCLI", "OneCLI started")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", onecliContainer); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to start onecli container: %w", err)
	}

	// Read persisted pod template data for networking config and approval handler path.
	tmplData, err := p.readPodTemplateData(id)
	if err != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to read pod template data: %w", err)
	}

	// Configure OneCLI networking rules. Deny is the default — rules are applied
	// unless the network mode is explicitly set to allow in the workspace config.
	// The policy is read fresh from projects.json on each start so edits take effect
	// without recreating the workspace.
	wsCfg, loadErr := loadNetworkConfig(tmplData.SourcePath, p.globalStorageDir, tmplData.ProjectID)
	if loadErr != nil {
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to load network config: %w", loadErr)
	}

	// Networking rules are only configured when mode is explicitly deny AND at
	// least one allowed host is specified. All other cases (allow, no config,
	// deny without hosts) clear any stale rules so that mode switches take
	// effect without recreating the workspace.
	shouldConfigureNetworking := wsCfg != nil &&
		wsCfg.Network != nil &&
		wsCfg.Network.Mode != nil &&
		*wsCfg.Network.Mode == workspace.Deny &&
		wsCfg.Network.Hosts != nil &&
		len(*wsCfg.Network.Hosts) > 0

	// Always connect to OneCLI so networking state is kept consistent across
	// mode switches without recreating the workspace.
	onecliBaseURL := p.onecliURL(tmplData.OnecliWebPort)

	stepLogger.Start("Waiting for OneCLI readiness", "OneCLI ready")
	if err := waitForReady(ctx, onecliBaseURL); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("OneCLI not ready: %w", err)
	}

	if shouldConfigureNetworking {
		hosts := *wsCfg.Network.Hosts

		stepLogger.Start("Configuring network rules", "Network rules configured")
		if err := p.configureNetworking(ctx, onecliBaseURL, hosts, tmplData.ApprovalHandlerDir); err != nil {
			stepLogger.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to configure networking: %w", err)
		}

		// Start the approval-handler now that config.json is in place.
		approvalContainer := podName + "-approval-handler"
		stepLogger.Start("Starting approval handler", "Approval handler started")
		if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "start", approvalContainer); err != nil {
			stepLogger.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to start approval handler: %w", err)
		}
	} else {
		// Clear any leftover rules from a previous deny-mode start.
		stepLogger.Start("Clearing network rules", "Network rules cleared")
		if err := p.clearNetworkingRules(ctx, onecliBaseURL); err != nil {
			stepLogger.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to clear networking rules: %w", err)
		}
	}

	// Start the remaining containers (workspace and, in allow mode, approval-handler).
	stepLogger.Start(fmt.Sprintf("Starting pod: %s", podName), "Pod started")
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "pod", "start", podName); err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to start pod: %w", err)
	}

	// Install OneCLI CA certificate into system trust store if present
	if caPath := p.readCAContainerPath(id); caPath != "" {
		stepLogger.Start("Installing CA certificate", "CA certificate installed")
		if err := p.installCACert(ctx, id, caPath); err != nil {
			stepLogger.Fail(err)
			return runtime.RuntimeInfo{}, fmt.Errorf("failed to install CA certificate: %w", err)
		}
	}

	// Verify workspace container status
	stepLogger.Start("Verifying container status", "Container status verified")
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		stepLogger.Fail(err)
		return runtime.RuntimeInfo{}, fmt.Errorf("failed to get container info after start: %w", err)
	}

	return info, nil
}

const caTrustAnchorPath = "/etc/pki/ca-trust/source/anchors/onecli-ca.pem"

// installCACert copies the OneCLI CA certificate into the system trust store
// and runs update-ca-trust so all tools (gh, curl, etc.) trust the proxy.
func (p *podmanRuntime) installCACert(ctx context.Context, containerID, caContainerPath string) error {
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"exec", "--user", "root", containerID, "cp", caContainerPath, caTrustAnchorPath,
	); err != nil {
		return fmt.Errorf("failed to copy CA certificate (podman exec --user root %s cp %s %s): %w", containerID, caContainerPath, caTrustAnchorPath, err)
	}
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"exec", "--user", "root", containerID, "update-ca-trust",
	); err != nil {
		return fmt.Errorf("update-ca-trust failed: %w", err)
	}
	return nil
}

// waitForPostgres polls the postgres container inside the pod until pg_isready succeeds.
// The postgres container name follows the podman kube play convention: <podName>-postgres.
func (p *podmanRuntime) waitForPostgres(ctx context.Context, podName string) error {
	containerName := podName + "-postgres"
	var lastErr error
	for range postgresMaxRetries {
		_, err := p.executor.Output(ctx, io.Discard,
			"exec", containerName, "pg_isready", "-U", "onecli")
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(postgresRetryInterval):
		}
	}
	return fmt.Errorf("postgres not ready after %d retries: %w", postgresMaxRetries, lastErr)
}

const (
	readinessTimeout  = 60 * time.Second
	readinessInterval = 2 * time.Second
)

// waitForReady polls the OneCLI health endpoint until it responds or the timeout expires.
func waitForReady(ctx context.Context, baseURL string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()

	httpClient := &http.Client{Timeout: 5 * time.Second}

	for {
		req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, baseURL+"/api/health", nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}

		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timed out after %s waiting for OneCLI at %s: %w", readinessTimeout, baseURL, timeoutCtx.Err())
		case <-time.After(readinessInterval):
		}
	}
}
