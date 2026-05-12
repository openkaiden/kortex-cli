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
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/containerurl"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

const (
	networkRuleName = "kdn-network"
	modelRuleName   = "kdn-model"
)

var networkEndpointPorts = []int{80, 443}

// loadNetworkConfig reads the merged workspace configuration for a project by
// combining workspace-level, project-level, and agent-level configs. It mirrors
// the merge logic used at workspace creation time so that edits take effect on
// the next Start() without recreating the workspace.
func loadNetworkConfig(sourcePath, storageDir, projectID, agentName string) (*workspace.WorkspaceConfiguration, error) {
	merger := config.NewMerger()

	var merged *workspace.WorkspaceConfiguration

	wsCfgLoader, err := config.NewConfig(filepath.Join(sourcePath, ".kaiden"))
	if err != nil {
		return nil, fmt.Errorf("initializing workspace config loader: %w", err)
	}
	if wc, loadErr := wsCfgLoader.Load(); loadErr == nil {
		merged = wc
	}

	projectLoader, err := config.NewProjectConfigLoader(storageDir)
	if err != nil {
		return nil, fmt.Errorf("initializing project config loader: %w", err)
	}
	if pc, loadErr := projectLoader.Load(projectID); loadErr == nil {
		merged = merger.Merge(merged, pc)
	}

	if agentName != "" {
		agentLoader, err := config.NewAgentConfigLoader(storageDir)
		if err != nil {
			return nil, fmt.Errorf("initializing agent config loader: %w", err)
		}
		if ac, loadErr := agentLoader.Load(agentName); loadErr == nil {
			merged = merger.Merge(merged, ac)
		}
	}

	return merged, nil
}

// collectSecretHosts returns the host patterns contributed by the secrets
// listed in wsCfg. For known secret types, patterns come from the secret
// service registry; for "other" secrets, they come from the stored metadata.
func collectSecretHosts(wsCfg *workspace.WorkspaceConfiguration, store secret.Store, registry secretservice.Registry) ([]string, error) {
	if wsCfg == nil || wsCfg.Secrets == nil || len(*wsCfg.Secrets) == 0 {
		return nil, nil
	}
	if store == nil || registry == nil {
		return nil, nil
	}

	items, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}

	byName := make(map[string]secret.ListItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	seen := make(map[string]bool)
	var hosts []string
	for _, name := range *wsCfg.Secrets {
		item, ok := byName[name]
		if !ok {
			continue
		}
		var itemHosts []string
		if item.Type == secret.TypeOther {
			itemHosts = item.Hosts
		} else {
			svc, svcErr := registry.Get(item.Type)
			if svcErr != nil {
				continue
			}
			itemHosts = svc.HostsPatterns()
		}
		for _, h := range itemHosts {
			if !seen[h] {
				seen[h] = true
				hosts = append(hosts, h)
			}
		}
	}
	return hosts, nil
}

// mergeHosts returns a deduplicated list of all hosts from a and b,
// preserving order (a first, then new entries from b).
func mergeHosts(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, h := range a {
		if !seen[h] {
			seen[h] = true
			result = append(result, h)
		}
	}
	for _, h := range b {
		if !seen[h] {
			seen[h] = true
			result = append(result, h)
		}
	}
	return result
}

// collectAllRegistryHosts returns the host patterns from every registered
// secret service. Used in allow mode where all known services should be
// reachable regardless of which secrets are configured in the workspace.
func collectAllRegistryHosts(registry secretservice.Registry) []string {
	if registry == nil {
		return nil
	}

	names := registry.List()
	if len(names) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var hosts []string
	for _, name := range names {
		svc, err := registry.Get(name)
		if err != nil {
			continue
		}
		for _, h := range svc.HostsPatterns() {
			if !seen[h] {
				seen[h] = true
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

type modelEndpoint struct {
	Host string
	Port int
}

// collectModelEndpoints extracts the hostname and port from the baseURL
// embedded in a "provider::model::baseURL" model ID. Localhost URLs are
// rewritten to host.openshell.internal so the sandbox can reach host-local
// model servers (e.g. Ollama).
func collectModelEndpoints(modelID string) []modelEndpoint {
	if modelID == "" {
		return nil
	}
	_, _, baseURL := config.ParseModelID(modelID)
	if baseURL == "" {
		return nil
	}
	rewritten := containerurl.RewriteURLWithHost(baseURL, openshellContainerHost)
	u, err := url.Parse(rewritten)
	if err != nil || u.Host == "" {
		return nil
	}
	hostname := u.Hostname()
	port := 0
	if p := u.Port(); p != "" {
		parsedPort, convErr := strconv.Atoi(p)
		if convErr != nil || parsedPort <= 0 || parsedPort > 65535 {
			return nil
		}
		port = parsedPort
	}
	if port == 0 {
		if u.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	return []modelEndpoint{{Host: hostname, Port: port}}
}

// applyNetworkPolicy determines the network mode and applies the appropriate
// policy to the sandbox. Called from both Create and Start.
//
// OpenShell sandboxes enforce deny-by-default networking and do not support
// unrestricted "allow all" policies (wildcard hosts like "**" are rejected).
// When no deny-mode hosts are configured, the sandbox keeps its default
// deny-all behavior. Only explicit host patterns are added as rules.
//
// In allow mode, hosts are derived from the secret service registry and
// workspace config network.hosts.
// In deny mode, hosts come from workspace config network.hosts and
// configured secret host patterns.
func (r *openshellRuntime) applyNetworkPolicy(ctx context.Context, sandboxName string, wsCfg *workspace.WorkspaceConfiguration, modelID string) error {
	l := logger.FromContext(ctx)
	modelEndpoints := collectModelEndpoints(modelID)
	fmt.Fprintf(l.Stderr(), "applyNetworkPolicy: modelEndpoints=%d\n", len(modelEndpoints))

	isDenyMode := wsCfg != nil &&
		wsCfg.Network != nil &&
		wsCfg.Network.Mode != nil &&
		*wsCfg.Network.Mode == workspace.Deny

	if !isDenyMode {
		registryHosts := collectAllRegistryHosts(r.secretServiceRegistry)
		var wsHosts []string
		if wsCfg != nil && wsCfg.Network != nil && wsCfg.Network.Hosts != nil {
			wsHosts = *wsCfg.Network.Hosts
		}
		hosts := mergeHosts(registryHosts, wsHosts)
		fmt.Fprintf(l.Stderr(), "Network policy (allow mode): %v\n", hosts)
		if err := r.configureNetworkPolicy(ctx, sandboxName, hosts); err != nil {
			return err
		}
	} else {
		var explicitHosts []string
		if wsCfg.Network.Hosts != nil {
			explicitHosts = *wsCfg.Network.Hosts
		}

		secretHosts, err := collectSecretHosts(wsCfg, r.secretStore, r.secretServiceRegistry)
		if err != nil {
			return fmt.Errorf("collecting secret hosts: %w", err)
		}
		allHosts := mergeHosts(explicitHosts, secretHosts)
		fmt.Fprintf(l.Stderr(), "Network policy (deny mode): %v\n", allHosts)
		if err := r.configureNetworkPolicy(ctx, sandboxName, allHosts); err != nil {
			return err
		}
	}

	if len(modelEndpoints) > 0 {
		return r.configureModelPolicy(ctx, sandboxName, modelEndpoints)
	}
	return nil
}

// configureNetworkPolicy applies network rules to the sandbox via the
// openshell policy update CLI. It always removes any existing kdn-managed
// rule first, then adds endpoints for each host on standard ports.
func (r *openshellRuntime) configureNetworkPolicy(ctx context.Context, sandboxName string, hosts []string) error {
	l := logger.FromContext(ctx)

	// Remove existing kdn-managed rule (ignore errors — rule may not exist)
	_ = r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"policy", "update", sandboxName, "--remove-rule", networkRuleName,
	)

	if len(hosts) == 0 {
		return nil
	}

	args := []string{"policy", "update", sandboxName}
	for _, host := range hosts {
		for _, port := range networkEndpointPorts {
			args = append(args, "--add-endpoint", fmt.Sprintf("%s:%d:full", host, port))
		}
	}
	args = append(args, "--binary", "/**", "--wait")
	fmt.Fprintf(l.Stderr(), "configureNetworkPolicy: running %v\n", args)

	if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), args...); err != nil {
		if strings.Contains(err.Error(), "sandbox has no spec") {
			fmt.Fprintf(l.Stderr(), "Network policy not supported for this sandbox, skipping\n")
			return nil
		}
		return fmt.Errorf("updating network policy: %w", err)
	}

	return nil
}

const resolveHostTimeout = 10 * time.Second

// resolveHostAliases resolves a hostname to its IP address inside the sandbox
// using getent. This is needed for internal hostnames like host.openshell.internal
// that resolve to internal IPs and would be blocked by SSRF protection when
// used as host:port:full endpoints.
func (r *openshellRuntime) resolveHostAliases(ctx context.Context, sandboxName, hostname string) (string, error) {
	l := logger.FromContext(ctx)
	resolveCtx, cancel := context.WithTimeout(ctx, resolveHostTimeout)
	defer cancel()
	out, err := r.executor.Output(resolveCtx, l.Stderr(),
		"sandbox", "exec", "--name", sandboxName, "--no-tty", "--", "getent", "hosts", hostname,
	)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", hostname, err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return "", fmt.Errorf("unexpected getent output for %s: %q", hostname, string(out))
	}
	return fields[0], nil
}

// configureModelPolicy adds policy rules for model endpoints so the agent
// can reach host-local model servers (e.g. Ollama). Hostnames are resolved
// to IP addresses inside the sandbox and added as allowed-ip endpoints to
// bypass SSRF protection for internal addresses.
func (r *openshellRuntime) configureModelPolicy(ctx context.Context, sandboxName string, modelEndpoints []modelEndpoint) error {
	l := logger.FromContext(ctx)

	_ = r.executor.Run(ctx, l.Stdout(), l.Stderr(),
		"policy", "update", sandboxName, "--remove-rule", modelRuleName,
	)

	if len(modelEndpoints) == 0 {
		return nil
	}

	for _, ep := range modelEndpoints {
		ip, err := r.resolveHostAliases(ctx, sandboxName, ep.Host)
		if err != nil {
			fmt.Fprintf(l.Stderr(), "configureModelPolicy: failed to resolve %s, skipping: %v\n", ep.Host, err)
			continue
		}
		fmt.Fprintf(l.Stderr(), "configureModelPolicy: resolved %s to %s\n", ep.Host, ip)

		endpoint := fmt.Sprintf("%s:%d::::allowed-ip=%s", ep.Host, ep.Port, ip)
		args := []string{
			"policy", "update", sandboxName,
			"--add-endpoint", endpoint,
			"--rule-name", modelRuleName,
			"--binary", "/**", "--wait",
		}
		fmt.Fprintf(l.Stderr(), "configureModelPolicy: running %v\n", args)

		if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), args...); err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "sandbox has no spec") {
				fmt.Fprintf(l.Stderr(), "Model policy not supported for this sandbox, skipping\n")
				return nil
			}
			if strings.Contains(errMsg, "exit status 124") || strings.Contains(errMsg, "Timeout") {
				fmt.Fprintf(l.Stderr(), "configureModelPolicy: policy submitted but timed out waiting for load, continuing\n")
				continue
			}
			return fmt.Errorf("updating model policy: %w", err)
		}
	}

	return nil
}
