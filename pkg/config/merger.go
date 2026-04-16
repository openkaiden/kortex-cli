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

package config

import (
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// Merger merges multiple WorkspaceConfiguration objects with proper precedence rules.
// When merging:
// - Environment variables: Later configs override earlier ones (by name)
// - Mounts: Deduplicated by host+target pair (preserves order, no duplicates)
type Merger interface {
	// Merge combines two WorkspaceConfiguration objects.
	// The override config takes precedence over the base config.
	// Returns a new merged configuration without modifying the inputs.
	Merge(base, override *workspace.WorkspaceConfiguration) *workspace.WorkspaceConfiguration
}

// merger is the internal implementation of Merger
type merger struct{}

// Compile-time check to ensure merger implements Merger interface
var _ Merger = (*merger)(nil)

// NewMerger creates a new configuration merger
func NewMerger() Merger {
	return &merger{}
}

// Merge combines two WorkspaceConfiguration objects with override taking precedence
func (m *merger) Merge(base, override *workspace.WorkspaceConfiguration) *workspace.WorkspaceConfiguration {
	// If both are nil, return nil
	if base == nil && override == nil {
		return nil
	}

	// If only base is nil, return a copy of override
	if base == nil {
		return copyConfig(override)
	}

	// If only override is nil, return a copy of base
	if override == nil {
		return copyConfig(base)
	}

	// Merge both configurations
	result := &workspace.WorkspaceConfiguration{}

	// Merge environment variables
	result.Environment = mergeEnvironment(base.Environment, override.Environment)

	// Merge mounts
	result.Mounts = mergeMounts(base.Mounts, override.Mounts)

	// Merge skills
	result.Skills = mergeSkills(base.Skills, override.Skills)

	// Merge MCP configuration
	result.Mcp = mergeMCP(base.Mcp, override.Mcp)

	// Merge secrets
	result.Secrets = mergeSecrets(base.Secrets, override.Secrets)

	// Merge network configuration
	result.Network = mergeNetwork(base.Network, override.Network)

	return result
}

// mergeEnvironment merges environment variables, with override taking precedence by name
func mergeEnvironment(base, override *[]workspace.EnvironmentVariable) *[]workspace.EnvironmentVariable {
	if base == nil && override == nil {
		return nil
	}

	// Create a map to track variables by name
	envMap := make(map[string]workspace.EnvironmentVariable)
	var order []string

	// Add base environment variables
	if base != nil {
		for _, env := range *base {
			envMap[env.Name] = env
			order = append(order, env.Name)
		}
	}

	// Override with variables from override config
	if override != nil {
		for _, env := range *override {
			if _, exists := envMap[env.Name]; !exists {
				// New variable, add to order
				order = append(order, env.Name)
			}
			// Override or add the variable
			envMap[env.Name] = env
		}
	}

	// Build result array preserving order
	if len(envMap) == 0 {
		return nil
	}

	result := make([]workspace.EnvironmentVariable, 0, len(order))
	for _, name := range order {
		result = append(result, envMap[name])
	}

	return &result
}

// deepCopyMount returns a deep copy of m with the Ro pointer independent from the original.
func deepCopyMount(m workspace.Mount) workspace.Mount {
	if m.Ro != nil {
		roCopy := *m.Ro
		m.Ro = &roCopy
	}
	return m
}

// mergeMounts merges mount slices, deduplicating by host+target pair.
// Mounts from base are appended first; if override contains a mount with the same
// host+target key, it replaces the base entry in-place (preserving position) so that
// per-mount fields such as Ro are correctly overridden.
func mergeMounts(base, override *[]workspace.Mount) *[]workspace.Mount {
	if base == nil && override == nil {
		return nil
	}

	type mountKey struct{ host, target string }
	seen := make(map[mountKey]int) // value is index in result
	var result []workspace.Mount

	for _, slice := range []*[]workspace.Mount{base, override} {
		if slice == nil {
			continue
		}
		isOverride := slice == override
		for _, m := range *slice {
			key := mountKey{m.Host, m.Target}
			if idx, exists := seen[key]; !exists {
				seen[key] = len(result)
				result = append(result, deepCopyMount(m))
			} else if isOverride {
				result[idx] = deepCopyMount(m)
			}
		}
	}

	if len(result) == 0 {
		return nil
	}

	return &result
}

// mergeSkills merges skills slices, deduplicating by path value.
// Skills from base come first; skills from override are appended if not already present.
func mergeSkills(base, override *[]string) *[]string {
	if base == nil && override == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string

	for _, slice := range []*[]string{base, override} {
		if slice == nil {
			continue
		}
		for _, s := range *slice {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return &result
}

// deepCopySecret returns a deep copy of s so that its pointer fields do not
// alias the original.
func deepCopySecret(s workspace.Secret) workspace.Secret {
	if s.Header != nil {
		headerCopy := *s.Header
		s.Header = &headerCopy
	}
	if s.HeaderTemplate != nil {
		htCopy := *s.HeaderTemplate
		s.HeaderTemplate = &htCopy
	}
	if s.Hosts != nil {
		hostsCopy := make([]string, len(*s.Hosts))
		copy(hostsCopy, *s.Hosts)
		s.Hosts = &hostsCopy
	}
	if s.Name != nil {
		nameCopy := *s.Name
		s.Name = &nameCopy
	}
	if s.Path != nil {
		pathCopy := *s.Path
		s.Path = &pathCopy
	}
	return s
}

// secretKey builds the deduplication key for a secret from its (type, name) tuple.
func secretKey(s workspace.Secret) struct{ typ, name string } {
	name := ""
	if s.Name != nil {
		name = *s.Name
	}
	return struct{ typ, name string }{typ: s.Type, name: name}
}

// mergeSecrets merges secret slices, deduplicating by (type, name) tuple.
// Secrets from base come first; override entries replace base entries with the
// same key.
func mergeSecrets(base, override *[]workspace.Secret) *[]workspace.Secret {
	if base == nil && override == nil {
		return nil
	}

	type sKey = struct{ typ, name string }
	sMap := make(map[sKey]workspace.Secret)
	var order []sKey

	if base != nil {
		for _, s := range *base {
			key := secretKey(s)
			sMap[key] = s
			order = append(order, key)
		}
	}

	if override != nil {
		for _, s := range *override {
			key := secretKey(s)
			if _, exists := sMap[key]; !exists {
				order = append(order, key)
			}
			sMap[key] = s
		}
	}

	if len(sMap) == 0 {
		return nil
	}

	result := make([]workspace.Secret, 0, len(order))
	for _, key := range order {
		result = append(result, deepCopySecret(sMap[key]))
	}
	return &result
}

// mergeMCP merges two McpConfiguration objects, with override taking precedence by name.
// Commands and servers from base are included first; override entries replace base entries
// with the same name.
//
// Cross-type collisions are resolved in favour of the override side: if override defines
// a command named "foo", any base server named "foo" is dropped, and vice-versa. This
// prevents the lower-precedence type from silently overwriting the higher-precedence one
// when an agent flattens both lists into a single mcpServers map.
func mergeMCP(base, override *workspace.McpConfiguration) *workspace.McpConfiguration {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		return copyMCP(override)
	}
	if override == nil {
		return copyMCP(base)
	}

	// Build sets of names claimed by each override list so we can resolve cross-type
	// collisions (e.g. base.Servers["foo"] must lose to override.Commands["foo"]).
	overrideCmdNames := make(map[string]bool)
	if override.Commands != nil {
		for _, cmd := range *override.Commands {
			overrideCmdNames[cmd.Name] = true
		}
	}
	overrideSrvNames := make(map[string]bool)
	if override.Servers != nil {
		for _, srv := range *override.Servers {
			overrideSrvNames[srv.Name] = true
		}
	}

	result := &workspace.McpConfiguration{}
	result.Commands = mergeMCPCommands(base.Commands, override.Commands)
	result.Servers = mergeMCPServers(base.Servers, override.Servers)

	// Drop any command whose name was claimed by override.Servers, and any server
	// whose name was claimed by override.Commands.
	if result.Commands != nil && len(overrideSrvNames) > 0 {
		filtered := (*result.Commands)[:0:0]
		for _, cmd := range *result.Commands {
			if !overrideSrvNames[cmd.Name] {
				filtered = append(filtered, cmd)
			}
		}
		if len(filtered) == 0 {
			result.Commands = nil
		} else {
			result.Commands = &filtered
		}
	}
	if result.Servers != nil && len(overrideCmdNames) > 0 {
		filtered := (*result.Servers)[:0:0]
		for _, srv := range *result.Servers {
			if !overrideCmdNames[srv.Name] {
				filtered = append(filtered, srv)
			}
		}
		if len(filtered) == 0 {
			result.Servers = nil
		} else {
			result.Servers = &filtered
		}
	}

	if result.Commands == nil && result.Servers == nil {
		return nil
	}
	return result
}

// deepCopyMcpCommand returns a deep copy of cmd so that its Args and Env
// pointer fields do not alias the original.
func deepCopyMcpCommand(cmd workspace.McpCommand) workspace.McpCommand {
	if cmd.Args != nil {
		argsCopy := make([]string, len(*cmd.Args))
		copy(argsCopy, *cmd.Args)
		cmd.Args = &argsCopy
	}
	if cmd.Env != nil {
		envCopy := make(map[string]string, len(*cmd.Env))
		for k, v := range *cmd.Env {
			envCopy[k] = v
		}
		cmd.Env = &envCopy
	}
	return cmd
}

// deepCopyMcpServer returns a deep copy of srv so that its Headers pointer
// field does not alias the original.
func deepCopyMcpServer(srv workspace.McpServer) workspace.McpServer {
	if srv.Headers != nil {
		hdrs := make(map[string]string, len(*srv.Headers))
		for k, v := range *srv.Headers {
			hdrs[k] = v
		}
		srv.Headers = &hdrs
	}
	return srv
}

// mergeMCPCommands merges command slices, deduplicating by name (override wins).
func mergeMCPCommands(base, override *[]workspace.McpCommand) *[]workspace.McpCommand {
	if base == nil && override == nil {
		return nil
	}

	cmdMap := make(map[string]workspace.McpCommand)
	var order []string

	if base != nil {
		for _, cmd := range *base {
			cmdMap[cmd.Name] = cmd
			order = append(order, cmd.Name)
		}
	}
	if override != nil {
		for _, cmd := range *override {
			if _, exists := cmdMap[cmd.Name]; !exists {
				order = append(order, cmd.Name)
			}
			cmdMap[cmd.Name] = cmd
		}
	}

	if len(cmdMap) == 0 {
		return nil
	}

	result := make([]workspace.McpCommand, 0, len(order))
	for _, name := range order {
		result = append(result, deepCopyMcpCommand(cmdMap[name]))
	}
	return &result
}

// mergeMCPServers merges server slices, deduplicating by name (override wins).
func mergeMCPServers(base, override *[]workspace.McpServer) *[]workspace.McpServer {
	if base == nil && override == nil {
		return nil
	}

	srvMap := make(map[string]workspace.McpServer)
	var order []string

	if base != nil {
		for _, srv := range *base {
			srvMap[srv.Name] = srv
			order = append(order, srv.Name)
		}
	}
	if override != nil {
		for _, srv := range *override {
			if _, exists := srvMap[srv.Name]; !exists {
				order = append(order, srv.Name)
			}
			srvMap[srv.Name] = srv
		}
	}

	if len(srvMap) == 0 {
		return nil
	}

	result := make([]workspace.McpServer, 0, len(order))
	for _, name := range order {
		result = append(result, deepCopyMcpServer(srvMap[name]))
	}
	return &result
}

// copyMCP creates a deep copy of an McpConfiguration.
func copyMCP(mcp *workspace.McpConfiguration) *workspace.McpConfiguration {
	if mcp == nil {
		return nil
	}
	result := &workspace.McpConfiguration{}
	if mcp.Commands != nil {
		cmdsCopy := make([]workspace.McpCommand, len(*mcp.Commands))
		for i, cmd := range *mcp.Commands {
			cmdsCopy[i] = deepCopyMcpCommand(cmd)
		}
		if len(cmdsCopy) > 0 {
			result.Commands = &cmdsCopy
		}
	}
	if mcp.Servers != nil {
		srvsCopy := make([]workspace.McpServer, len(*mcp.Servers))
		for i, srv := range *mcp.Servers {
			srvsCopy[i] = deepCopyMcpServer(srv)
		}
		if len(srvsCopy) > 0 {
			result.Servers = &srvsCopy
		}
	}
	if result.Commands == nil && result.Servers == nil {
		return nil
	}
	return result
}

// mergeNetwork merges two NetworkConfiguration objects.
// The base network policy is dominant:
//   - if base has mode "allow", use base configuration
//   - if base has mode "deny" and override has mode "allow", use base configuration
//   - if both base and override have mode "deny", merge hosts from both
func mergeNetwork(base, override *workspace.NetworkConfiguration) *workspace.NetworkConfiguration {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		return copyNetwork(override)
	}
	if override == nil {
		return copyNetwork(base)
	}

	// Base with "allow" mode always wins, regardless of override
	if base.Mode != nil && *base.Mode == workspace.Allow {
		return copyNetwork(base)
	}

	// Base has "deny" (or default deny) and override has "allow": base wins
	if override.Mode != nil && *override.Mode == workspace.Allow {
		return copyNetwork(base)
	}

	// Both have "deny" mode: merge hosts
	result := &workspace.NetworkConfiguration{}
	mode := workspace.Deny
	result.Mode = &mode
	result.Hosts = mergeStringSlices(base.Hosts, override.Hosts)

	return result
}

// mergeStringSlices merges two optional string slices, deduplicating entries.
// Base entries come first, followed by new entries from override.
func mergeStringSlices(base, override *[]string) *[]string {
	if base == nil && override == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string

	for _, slice := range []*[]string{base, override} {
		if slice == nil {
			continue
		}
		for _, s := range *slice {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return &result
}

// copyNetwork creates a deep copy of a NetworkConfiguration.
func copyNetwork(net *workspace.NetworkConfiguration) *workspace.NetworkConfiguration {
	if net == nil {
		return nil
	}
	result := &workspace.NetworkConfiguration{}
	if net.Mode != nil {
		modeCopy := *net.Mode
		result.Mode = &modeCopy
	}
	if net.Hosts != nil {
		hostsCopy := make([]string, len(*net.Hosts))
		copy(hostsCopy, *net.Hosts)
		result.Hosts = &hostsCopy
	}
	return result
}

// copyConfig creates a deep copy of a WorkspaceConfiguration
func copyConfig(cfg *workspace.WorkspaceConfiguration) *workspace.WorkspaceConfiguration {
	if cfg == nil {
		return nil
	}

	result := &workspace.WorkspaceConfiguration{}

	// Copy environment variables
	if cfg.Environment != nil {
		envCopy := make([]workspace.EnvironmentVariable, len(*cfg.Environment))
		copy(envCopy, *cfg.Environment)
		result.Environment = &envCopy
	}

	// Copy mounts (deep copy each entry so Ro pointers are independent)
	if cfg.Mounts != nil {
		mountsCopy := make([]workspace.Mount, len(*cfg.Mounts))
		for i, m := range *cfg.Mounts {
			mountsCopy[i] = deepCopyMount(m)
		}
		result.Mounts = &mountsCopy
	}

	// Copy skills
	if cfg.Skills != nil {
		skillsCopy := make([]string, len(*cfg.Skills))
		copy(skillsCopy, *cfg.Skills)
		result.Skills = &skillsCopy
	}

	// Copy MCP configuration
	result.Mcp = copyMCP(cfg.Mcp)

	// Copy secrets
	if cfg.Secrets != nil {
		secretsCopy := make([]workspace.Secret, len(*cfg.Secrets))
		for i, s := range *cfg.Secrets {
			secretsCopy[i] = deepCopySecret(s)
		}
		result.Secrets = &secretsCopy
	}

	// Copy network configuration
	result.Network = copyNetwork(cfg.Network)

	return result
}
