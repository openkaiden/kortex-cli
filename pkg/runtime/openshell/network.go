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
	"path/filepath"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

const networkRuleName = "kdn-network"

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

// applyNetworkPolicy determines the network mode and applies the appropriate
// policy to the sandbox. Called from both Create and Start.
//
// OpenShell sandboxes enforce deny-by-default networking and do not support
// unrestricted "allow all" policies (wildcard hosts like "**" are rejected).
// When no deny-mode hosts are configured, the sandbox keeps its default
// deny-all behavior. Only explicit host patterns are added as rules.
//
// In allow mode, hosts are derived from the secret service registry and
// the allowHosts parameter (from --openshell-allow-hosts).
// In deny mode, hosts come from workspace config network.hosts and
// configured secret host patterns.
func (r *openshellRuntime) applyNetworkPolicy(ctx context.Context, sandboxName string, wsCfg *workspace.WorkspaceConfiguration, allowHosts []string) error {
	l := logger.FromContext(ctx)

	isDenyMode := wsCfg != nil &&
		wsCfg.Network != nil &&
		wsCfg.Network.Mode != nil &&
		*wsCfg.Network.Mode == workspace.Deny

	if !isDenyMode {
		registryHosts := collectAllRegistryHosts(r.secretServiceRegistry)
		hosts := mergeHosts(registryHosts, allowHosts)
		fmt.Fprintf(l.Stderr(), "Network policy (allow mode): %v\n", hosts)
		return r.configureNetworkPolicy(ctx, sandboxName, hosts)
	}

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

	return r.configureNetworkPolicy(ctx, sandboxName, allHosts)
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

	if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), args...); err != nil {
		if strings.Contains(err.Error(), "sandbox has no spec") {
			fmt.Fprintf(l.Stderr(), "Network policy not supported for this sandbox, skipping\n")
			return nil
		}
		return fmt.Errorf("updating network policy: %w", err)
	}

	return nil
}
