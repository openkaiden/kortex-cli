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

	return result
}
