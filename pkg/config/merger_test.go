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
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestMerger_Merge_NilInputs(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()

		result := merger.Merge(nil, nil)
		if result != nil {
			t.Error("Expected nil result when both inputs are nil")
		}
	})

	t.Run("base nil", func(t *testing.T) {
		t.Parallel()

		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "TEST", Value: strPtr("value")},
			},
		}

		result := merger.Merge(nil, override)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Environment == nil || len(*result.Environment) != 1 {
			t.Error("Expected environment to be copied from override")
		}
	})

	t.Run("override nil", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "TEST", Value: strPtr("value")},
			},
		}

		result := merger.Merge(base, nil)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Environment == nil || len(*result.Environment) != 1 {
			t.Error("Expected environment to be copied from base")
		}
	})
}

func TestMerger_Merge_Environment(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("base1")},
				{Name: "VAR2", Value: strPtr("base2")},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR3", Value: strPtr("override3")},
				{Name: "VAR4", Value: strPtr("override4")},
			},
		}

		result := merger.Merge(base, override)

		if result.Environment == nil {
			t.Fatal("Expected environment to be set")
		}

		env := *result.Environment
		if len(env) != 4 {
			t.Errorf("Expected 4 environment variables, got %d", len(env))
		}

		// Check that all variables are present
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		if envMap["VAR1"] != "base1" {
			t.Error("VAR1 not preserved from base")
		}
		if envMap["VAR2"] != "base2" {
			t.Error("VAR2 not preserved from base")
		}
		if envMap["VAR3"] != "override3" {
			t.Error("VAR3 not added from override")
		}
		if envMap["VAR4"] != "override4" {
			t.Error("VAR4 not added from override")
		}
	})

	t.Run("override takes precedence", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("base-value")},
				{Name: "VAR2", Value: strPtr("keep-this")},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("override-value")},
				{Name: "VAR3", Value: strPtr("new-var")},
			},
		}

		result := merger.Merge(base, override)

		env := *result.Environment
		if len(env) != 3 {
			t.Errorf("Expected 3 environment variables, got %d", len(env))
		}

		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		if envMap["VAR1"] != "override-value" {
			t.Errorf("Expected VAR1='override-value', got '%s'", envMap["VAR1"])
		}
		if envMap["VAR2"] != "keep-this" {
			t.Error("VAR2 should be preserved")
		}
		if envMap["VAR3"] != "new-var" {
			t.Error("VAR3 should be added")
		}
	})

	t.Run("value vs secret", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("value1")},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Secret: strPtr("secret-ref")},
			},
		}

		result := merger.Merge(base, override)

		env := *result.Environment
		if len(env) != 1 {
			t.Fatalf("Expected 1 environment variable, got %d", len(env))
		}

		if env[0].Secret == nil || *env[0].Secret != "secret-ref" {
			t.Error("Expected secret to override value")
		}
		if env[0].Value != nil {
			t.Error("Expected value to be nil after secret override")
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "A", Value: strPtr("a")},
				{Name: "B", Value: strPtr("b")},
				{Name: "C", Value: strPtr("c")},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "B", Value: strPtr("b-override")},
				{Name: "D", Value: strPtr("d")},
			},
		}

		result := merger.Merge(base, override)

		env := *result.Environment
		// Order should be: A (base), B (base position but override value), C (base), D (override)
		if len(env) != 4 {
			t.Fatalf("Expected 4 variables, got %d", len(env))
		}

		if env[0].Name != "A" {
			t.Errorf("Expected first variable to be A, got %s", env[0].Name)
		}
		if env[1].Name != "B" {
			t.Errorf("Expected second variable to be B, got %s", env[1].Name)
		}
		if env[2].Name != "C" {
			t.Errorf("Expected third variable to be C, got %s", env[2].Name)
		}
		if env[3].Name != "D" {
			t.Errorf("Expected fourth variable to be D, got %s", env[3].Name)
		}
	})

	t.Run("empty base", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{}
		override := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("value1")},
			},
		}

		result := merger.Merge(base, override)

		if result.Environment == nil || len(*result.Environment) != 1 {
			t.Error("Expected environment from override")
		}
	})

	t.Run("empty override", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "VAR1", Value: strPtr("value1")},
			},
		}
		override := &workspace.WorkspaceConfiguration{}

		result := merger.Merge(base, override)

		if result.Environment == nil || len(*result.Environment) != 1 {
			t.Error("Expected environment from base")
		}
	})
}

func TestMerger_Merge_Mounts(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep1", Target: "/workspace/dep1"},
				{Host: "/host/dep2", Target: "/workspace/dep2"},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep3", Target: "/workspace/dep3"},
				{Host: "/host/dep4", Target: "/workspace/dep4"},
			},
		}

		result := merger.Merge(base, override)

		if result.Mounts == nil {
			t.Fatal("Expected mounts to be set")
		}

		mounts := *result.Mounts
		if len(mounts) != 4 {
			t.Errorf("Expected 4 mounts, got %d", len(mounts))
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep1", Target: "/workspace/dep1"},
				{Host: "/host/dep2", Target: "/workspace/dep2"},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep2", Target: "/workspace/dep2"},
				{Host: "/host/dep3", Target: "/workspace/dep3"},
			},
		}

		result := merger.Merge(base, override)

		mounts := *result.Mounts
		if len(mounts) != 3 {
			t.Errorf("Expected 3 unique mounts, got %d", len(mounts))
		}

		// Check order: dep1, dep2 (from base), dep3 (new from override)
		if mounts[0].Host != "/host/dep1" || mounts[1].Host != "/host/dep2" || mounts[2].Host != "/host/dep3" {
			t.Errorf("Unexpected order: %v", mounts)
		}
	})

	t.Run("override updates ro for same host+target", func(t *testing.T) {
		t.Parallel()

		roTrue := true
		base := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep1", Target: "/workspace/dep1"},
				{Host: "/host/dep2", Target: "/workspace/dep2"},
			},
		}

		override := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep2", Target: "/workspace/dep2", Ro: &roTrue},
			},
		}

		result := merger.Merge(base, override)

		mounts := *result.Mounts
		if len(mounts) != 2 {
			t.Fatalf("Expected 2 mounts, got %d", len(mounts))
		}

		// dep2 must stay at index 1 (base position) but have ro=true from override
		if mounts[1].Host != "/host/dep2" {
			t.Errorf("Expected dep2 at index 1, got %s", mounts[1].Host)
		}
		if mounts[1].Ro == nil || !*mounts[1].Ro {
			t.Error("Expected ro=true on dep2 from override")
		}

		// Verify the original base is not mutated
		baseMounts := *base.Mounts
		if baseMounts[1].Ro != nil {
			t.Error("Expected base dep2 Ro to remain nil (no mutation)")
		}
	})

	t.Run("ro pointer is not shared after merge", func(t *testing.T) {
		t.Parallel()

		roTrue := true
		base := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "/host/dep1", Target: "/workspace/dep1", Ro: &roTrue},
			},
		}

		result := merger.Merge(base, nil)

		mounts := *result.Mounts
		if mounts[0].Ro == nil || !*mounts[0].Ro {
			t.Fatal("Expected ro=true in copy")
		}

		// Mutate the copy — base must not be affected
		roFalse := false
		mounts[0].Ro = &roFalse
		if !*(*base.Mounts)[0].Ro {
			t.Error("Base Ro was mutated through shared pointer")
		}
	})

	t.Run("empty slices return nil", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{},
		}

		override := &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{},
		}

		result := merger.Merge(base, override)

		if result.Mounts != nil {
			t.Error("Expected mounts to be nil when all slices are empty")
		}
	})
}

func TestMerger_Merge_MultiLevel(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("three level merge", func(t *testing.T) {
		t.Parallel()

		// Workspace level
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "LEVEL", Value: strPtr("workspace")},
				{Name: "WORKSPACE_VAR", Value: strPtr("ws-value")},
			},
			Mounts: &[]workspace.Mount{
				{Host: "/host/workspace-dep", Target: "/workspace/workspace-dep"},
			},
		}

		// Project level
		projectCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "LEVEL", Value: strPtr("project")},
				{Name: "PROJECT_VAR", Value: strPtr("proj-value")},
			},
			Mounts: &[]workspace.Mount{
				{Host: "/host/project-dep", Target: "/workspace/project-dep"},
				{Host: "$HOME/.gitconfig", Target: "/workspace/.gitconfig"},
			},
		}

		// Agent level
		agentCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "LEVEL", Value: strPtr("agent")},
				{Name: "AGENT_VAR", Value: strPtr("agent-value")},
			},
			Mounts: &[]workspace.Mount{
				{Host: "$HOME/.claude", Target: "/workspace/.claude"},
			},
		}

		// Merge: workspace -> project -> agent
		merged1 := merger.Merge(workspaceCfg, projectCfg)
		result := merger.Merge(merged1, agentCfg)

		// Check environment variables
		if result.Environment == nil {
			t.Fatal("Expected environment to be set")
		}

		env := *result.Environment
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		// LEVEL should be from agent (highest precedence)
		if envMap["LEVEL"] != "agent" {
			t.Errorf("Expected LEVEL='agent', got '%s'", envMap["LEVEL"])
		}

		// All other vars should be present
		if envMap["WORKSPACE_VAR"] != "ws-value" {
			t.Error("WORKSPACE_VAR should be preserved")
		}
		if envMap["PROJECT_VAR"] != "proj-value" {
			t.Error("PROJECT_VAR should be preserved")
		}
		if envMap["AGENT_VAR"] != "agent-value" {
			t.Error("AGENT_VAR should be added")
		}

		// Check mounts: 1 (workspace) + 2 (project) + 1 (agent) = 4 unique mounts
		if result.Mounts == nil {
			t.Fatal("Expected mounts to be set")
		}

		mounts := *result.Mounts
		if len(mounts) != 4 {
			t.Errorf("Expected 4 mounts, got %d", len(mounts))
		}
	})
}

func TestMerger_Merge_EmptyConfigurations(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("both empty", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{}
		override := &workspace.WorkspaceConfiguration{}

		result := merger.Merge(base, override)

		if result == nil {
			t.Error("Expected non-nil result")
		}

		if result.Environment != nil {
			t.Error("Expected environment to be nil")
		}

		if result.Mounts != nil {
			t.Error("Expected mounts to be nil")
		}
	})
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}

func TestMergeSkills(t *testing.T) {
	t.Parallel()

	t.Run("both nil returns nil", func(t *testing.T) {
		t.Parallel()

		result := mergeSkills(nil, nil)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("base nil returns copy of override", func(t *testing.T) {
		t.Parallel()

		override := &[]string{"/path/a", "/path/b"}
		result := mergeSkills(nil, override)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if len(*result) != 2 {
			t.Errorf("Expected 2 skills, got %d", len(*result))
		}
		if (*result)[0] != "/path/a" || (*result)[1] != "/path/b" {
			t.Errorf("Unexpected skills: %v", *result)
		}
	})

	t.Run("override nil returns copy of base", func(t *testing.T) {
		t.Parallel()

		base := &[]string{"/path/a", "/path/b"}
		result := mergeSkills(base, nil)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if len(*result) != 2 {
			t.Errorf("Expected 2 skills, got %d", len(*result))
		}
	})

	t.Run("no overlap combines all", func(t *testing.T) {
		t.Parallel()

		base := &[]string{"/path/a"}
		override := &[]string{"/path/b"}
		result := mergeSkills(base, override)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if len(*result) != 2 {
			t.Errorf("Expected 2 skills, got %d", len(*result))
		}
		if (*result)[0] != "/path/a" || (*result)[1] != "/path/b" {
			t.Errorf("Unexpected skills order: %v", *result)
		}
	})

	t.Run("duplicates are deduplicated", func(t *testing.T) {
		t.Parallel()

		base := &[]string{"/path/a", "/path/b"}
		override := &[]string{"/path/b", "/path/c"}
		result := mergeSkills(base, override)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if len(*result) != 3 {
			t.Errorf("Expected 3 skills, got %d: %v", len(*result), *result)
		}
		if (*result)[0] != "/path/a" || (*result)[1] != "/path/b" || (*result)[2] != "/path/c" {
			t.Errorf("Unexpected skills: %v", *result)
		}
	})
}

func TestMerger_Merge_MCP_BothNil(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{}
	override := &workspace.WorkspaceConfiguration{}

	result := merger.Merge(base, override)
	if result.Mcp != nil {
		t.Error("Expected Mcp to be nil when both have no MCP config")
	}
}

func TestMerger_Merge_MCP_BaseOnly(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "tool-a", Command: "cmd-a"},
			},
			Servers: &[]workspace.McpServer{
				{Name: "srv-a", Url: "https://a.example.com"},
			},
		},
	}
	override := &workspace.WorkspaceConfiguration{}

	result := merger.Merge(base, override)
	if result.Mcp == nil {
		t.Fatal("Expected non-nil Mcp")
	}
	if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
		t.Errorf("Expected 1 command, got %v", result.Mcp.Commands)
	}
	if (*result.Mcp.Commands)[0].Name != "tool-a" {
		t.Errorf("Expected command name %q, got %q", "tool-a", (*result.Mcp.Commands)[0].Name)
	}
	if result.Mcp.Servers == nil || len(*result.Mcp.Servers) != 1 {
		t.Errorf("Expected 1 server, got %v", result.Mcp.Servers)
	}
}

func TestMerger_Merge_MCP_OverrideOnly(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{}
	override := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "tool-b", Command: "cmd-b"},
			},
		},
	}

	result := merger.Merge(base, override)
	if result.Mcp == nil {
		t.Fatal("Expected non-nil Mcp")
	}
	if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
		t.Errorf("Expected 1 command, got %v", result.Mcp.Commands)
	}
	if (*result.Mcp.Commands)[0].Name != "tool-b" {
		t.Errorf("Expected command name %q, got %q", "tool-b", (*result.Mcp.Commands)[0].Name)
	}
}

func TestMerger_Merge_MCP_CommandsMergedByName(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "tool-a", Command: "cmd-a"},
				{Name: "tool-b", Command: "cmd-b-base"},
			},
		},
	}
	override := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "tool-b", Command: "cmd-b-override"},
				{Name: "tool-c", Command: "cmd-c"},
			},
		},
	}

	result := merger.Merge(base, override)
	if result.Mcp == nil || result.Mcp.Commands == nil {
		t.Fatal("Expected non-nil Mcp.Commands")
	}

	cmds := *result.Mcp.Commands
	if len(cmds) != 3 {
		t.Fatalf("Expected 3 commands, got %d: %v", len(cmds), cmds)
	}

	cmdMap := make(map[string]string)
	for _, cmd := range cmds {
		cmdMap[cmd.Name] = cmd.Command
	}

	if cmdMap["tool-a"] != "cmd-a" {
		t.Errorf("tool-a command = %q, want %q", cmdMap["tool-a"], "cmd-a")
	}
	if cmdMap["tool-b"] != "cmd-b-override" {
		t.Errorf("tool-b should be overridden: got %q, want %q", cmdMap["tool-b"], "cmd-b-override")
	}
	if cmdMap["tool-c"] != "cmd-c" {
		t.Errorf("tool-c command = %q, want %q", cmdMap["tool-c"], "cmd-c")
	}
}

func TestMerger_Merge_MCP_ServersMergedByName(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Servers: &[]workspace.McpServer{
				{Name: "srv-a", Url: "https://a.example.com"},
				{Name: "srv-b", Url: "https://b-base.example.com"},
			},
		},
	}
	override := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Servers: &[]workspace.McpServer{
				{Name: "srv-b", Url: "https://b-override.example.com"},
				{Name: "srv-c", Url: "https://c.example.com"},
			},
		},
	}

	result := merger.Merge(base, override)
	if result.Mcp == nil || result.Mcp.Servers == nil {
		t.Fatal("Expected non-nil Mcp.Servers")
	}

	srvs := *result.Mcp.Servers
	if len(srvs) != 3 {
		t.Fatalf("Expected 3 servers, got %d: %v", len(srvs), srvs)
	}

	srvMap := make(map[string]string)
	for _, srv := range srvs {
		srvMap[srv.Name] = srv.Url
	}

	if srvMap["srv-a"] != "https://a.example.com" {
		t.Errorf("srv-a url = %q, want %q", srvMap["srv-a"], "https://a.example.com")
	}
	if srvMap["srv-b"] != "https://b-override.example.com" {
		t.Errorf("srv-b should be overridden: got %q, want %q", srvMap["srv-b"], "https://b-override.example.com")
	}
	if srvMap["srv-c"] != "https://c.example.com" {
		t.Errorf("srv-c url = %q, want %q", srvMap["srv-c"], "https://c.example.com")
	}
}

func TestMerger_Merge_MCP_DeepCopy(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("mutating merged command Args does not affect base", func(t *testing.T) {
		t.Parallel()

		args := []string{"--verbose"}
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "tool-a", Command: "cmd-a", Args: &args},
				},
			},
		}

		result := merger.Merge(base, nil)

		// Mutate the result's Args slice
		(*result.Mcp.Commands)[0].Args = &[]string{"--other"}

		// Base must be unaffected
		if (*(*base.Mcp.Commands)[0].Args)[0] != "--verbose" {
			t.Error("Mutating merged command Args affected the base input")
		}
	})

	t.Run("mutating merged command Env does not affect base", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"KEY": "original"}
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "tool-a", Command: "cmd-a", Env: &env},
				},
			},
		}

		result := merger.Merge(base, nil)

		// Mutate the result's Env map
		(*(*result.Mcp.Commands)[0].Env)["KEY"] = "mutated"

		// Base must be unaffected
		if env["KEY"] != "original" {
			t.Error("Mutating merged command Env affected the base input")
		}
	})

	t.Run("mutating merged server Headers does not affect base", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{"Authorization": "Bearer token"}
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Servers: &[]workspace.McpServer{
					{Name: "srv-a", Url: "https://a.example.com", Headers: &headers},
				},
			},
		}

		result := merger.Merge(base, nil)

		// Mutate the result's Headers map
		(*(*result.Mcp.Servers)[0].Headers)["Authorization"] = "Bearer other"

		// Base must be unaffected
		if headers["Authorization"] != "Bearer token" {
			t.Error("Mutating merged server Headers affected the base input")
		}
	})

	t.Run("override command Args is independent of override input", func(t *testing.T) {
		t.Parallel()

		args := []string{"--flag"}
		override := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "tool-a", Command: "cmd-a", Args: &args},
				},
			},
		}

		result := merger.Merge(nil, override)

		// Mutate the result's Args slice
		(*result.Mcp.Commands)[0].Args = &[]string{"--other"}

		// Override must be unaffected
		if (*(*override.Mcp.Commands)[0].Args)[0] != "--flag" {
			t.Error("Mutating merged command Args affected the override input")
		}
	})
}

func TestMerger_Merge_MCP_CrossTypeCollision(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("override command wins over base server with same name", func(t *testing.T) {
		t.Parallel()

		// base has a server named "foo"; override promotes it to a command – the
		// command (higher-precedence type) must win and the base server must be gone.
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Servers: &[]workspace.McpServer{
					{Name: "foo", Url: "https://base.example.com"},
					{Name: "bar", Url: "https://bar.example.com"},
				},
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "foo", Command: "foo-cmd"},
				},
			},
		}

		result := merger.Merge(base, override)
		if result.Mcp == nil {
			t.Fatal("Expected non-nil Mcp")
		}

		// "foo" command from override must be present
		if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
			t.Fatalf("Expected 1 command, got %v", result.Mcp.Commands)
		}
		if (*result.Mcp.Commands)[0].Name != "foo" {
			t.Errorf("Expected command name %q, got %q", "foo", (*result.Mcp.Commands)[0].Name)
		}

		// "foo" server from base must have been removed; only "bar" should remain
		if result.Mcp.Servers == nil || len(*result.Mcp.Servers) != 1 {
			t.Fatalf("Expected 1 server (bar), got %v", result.Mcp.Servers)
		}
		if (*result.Mcp.Servers)[0].Name != "bar" {
			t.Errorf("Expected server name %q, got %q", "bar", (*result.Mcp.Servers)[0].Name)
		}
	})

	t.Run("override server wins over base command with same name", func(t *testing.T) {
		t.Parallel()

		// base has a command named "foo"; override promotes it to a server – the
		// server (higher-precedence type) must win and the base command must be gone.
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "foo", Command: "foo-cmd-base"},
					{Name: "bar", Command: "bar-cmd"},
				},
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Servers: &[]workspace.McpServer{
					{Name: "foo", Url: "https://override.example.com"},
				},
			},
		}

		result := merger.Merge(base, override)
		if result.Mcp == nil {
			t.Fatal("Expected non-nil Mcp")
		}

		// "foo" server from override must be present
		if result.Mcp.Servers == nil || len(*result.Mcp.Servers) != 1 {
			t.Fatalf("Expected 1 server, got %v", result.Mcp.Servers)
		}
		if (*result.Mcp.Servers)[0].Name != "foo" {
			t.Errorf("Expected server name %q, got %q", "foo", (*result.Mcp.Servers)[0].Name)
		}

		// "foo" command from base must have been removed; only "bar" should remain
		if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
			t.Fatalf("Expected 1 command (bar), got %v", result.Mcp.Commands)
		}
		if (*result.Mcp.Commands)[0].Name != "bar" {
			t.Errorf("Expected command name %q, got %q", "bar", (*result.Mcp.Commands)[0].Name)
		}
	})

	t.Run("collision removes all base entries of losing type", func(t *testing.T) {
		t.Parallel()

		// When the override claims all server names as commands, the servers list
		// should become nil rather than an empty slice.
		base := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Servers: &[]workspace.McpServer{
					{Name: "foo", Url: "https://a.example.com"},
				},
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Mcp: &workspace.McpConfiguration{
				Commands: &[]workspace.McpCommand{
					{Name: "foo", Command: "foo-cmd"},
				},
			},
		}

		result := merger.Merge(base, override)
		if result.Mcp == nil {
			t.Fatal("Expected non-nil Mcp")
		}
		if result.Mcp.Servers != nil {
			t.Errorf("Expected Servers to be nil after all entries were displaced, got %v", result.Mcp.Servers)
		}
		if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
			t.Fatalf("Expected 1 command, got %v", result.Mcp.Commands)
		}
	})
}

func TestMerger_Merge_Secrets(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"gh-token", "slack-token"},
		}

		override := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"other-token"},
		}

		result := merger.Merge(base, override)

		if result.Secrets == nil {
			t.Fatal("Expected secrets to be set")
		}

		secrets := *result.Secrets
		if len(secrets) != 3 {
			t.Errorf("Expected 3 secrets, got %d", len(secrets))
		}

		if secrets[0] != "gh-token" {
			t.Errorf("Expected first secret to be %q, got %q", "gh-token", secrets[0])
		}
		if secrets[1] != "slack-token" {
			t.Errorf("Expected second secret to be %q, got %q", "slack-token", secrets[1])
		}
		if secrets[2] != "other-token" {
			t.Errorf("Expected third secret to be %q, got %q", "other-token", secrets[2])
		}
	})

	t.Run("deduplication by name", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"gh-token", "slack-token"},
		}

		override := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"gh-token", "other-token"},
		}

		result := merger.Merge(base, override)

		secrets := *result.Secrets
		if len(secrets) != 3 {
			t.Fatalf("Expected 3 secrets (deduplication), got %d", len(secrets))
		}

		if secrets[0] != "gh-token" {
			t.Errorf("Expected first to be %q, got %q", "gh-token", secrets[0])
		}
		if secrets[1] != "slack-token" {
			t.Errorf("Expected second to be %q, got %q", "slack-token", secrets[1])
		}
		if secrets[2] != "other-token" {
			t.Errorf("Expected third to be %q, got %q", "other-token", secrets[2])
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"a", "b", "c"},
		}

		override := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"b", "d"},
		}

		result := merger.Merge(base, override)

		secrets := *result.Secrets
		if len(secrets) != 4 {
			t.Fatalf("Expected 4 secrets, got %d", len(secrets))
		}

		expected := []string{"a", "b", "c", "d"}
		for i, want := range expected {
			if secrets[i] != want {
				t.Errorf("secrets[%d] = %q, want %q", i, secrets[i], want)
			}
		}
	})

	t.Run("empty base", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{}
		override := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"my-token"},
		}

		result := merger.Merge(base, override)

		if result.Secrets == nil || len(*result.Secrets) != 1 {
			t.Error("Expected secrets from override")
		}
	})

	t.Run("empty override", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"my-token"},
		}
		override := &workspace.WorkspaceConfiguration{}

		result := merger.Merge(base, override)

		if result.Secrets == nil || len(*result.Secrets) != 1 {
			t.Error("Expected secrets from base")
		}
	})

	t.Run("empty slices return nil", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{},
		}
		override := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{},
		}

		result := merger.Merge(base, override)

		if result.Secrets != nil {
			t.Error("Expected secrets to be nil when all slices are empty")
		}
	})
}

func TestMerger_Merge_Secrets_DeepCopy(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("mutating merged secrets does not affect base", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Secrets: &[]string{"original"},
		}

		result := merger.Merge(base, nil)

		// Mutate the result's slice
		(*result.Secrets)[0] = "mutated"

		// Base must be unaffected
		if (*base.Secrets)[0] != "original" {
			t.Error("Mutating merged secrets affected the base input")
		}
	})
}

func networkModePtr(m workspace.NetworkConfigurationMode) *workspace.NetworkConfigurationMode {
	return &m
}

func TestMerger_Merge_Network_BothNil(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{}
	override := &workspace.WorkspaceConfiguration{}

	result := merger.Merge(base, override)
	if result.Network != nil {
		t.Error("Expected Network to be nil when both have no network config")
	}
}

func TestMerger_Merge_Network_BaseOnly(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{
			Mode:  networkModePtr(workspace.Deny),
			Hosts: &[]string{"example.com"},
		},
	}
	override := &workspace.WorkspaceConfiguration{}

	result := merger.Merge(base, override)
	if result.Network == nil {
		t.Fatal("Expected non-nil Network")
	}
	if result.Network.Mode == nil || *result.Network.Mode != workspace.Deny {
		t.Errorf("Expected mode %q, got %v", workspace.Deny, result.Network.Mode)
	}
	if result.Network.Hosts == nil || len(*result.Network.Hosts) != 1 || (*result.Network.Hosts)[0] != "example.com" {
		t.Errorf("Expected hosts [example.com], got %v", result.Network.Hosts)
	}
}

func TestMerger_Merge_Network_OverrideOnly(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{}
	override := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{
			Mode:  networkModePtr(workspace.Deny),
			Hosts: &[]string{"override.com"},
		},
	}

	result := merger.Merge(base, override)
	if result.Network == nil {
		t.Fatal("Expected non-nil Network")
	}
	if result.Network.Hosts == nil || len(*result.Network.Hosts) != 1 || (*result.Network.Hosts)[0] != "override.com" {
		t.Errorf("Expected hosts [override.com], got %v", result.Network.Hosts)
	}
}

func TestMerger_Merge_Network_OverrideModeWins(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("override deny beats base allow", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode: networkModePtr(workspace.Allow),
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode:  networkModePtr(workspace.Deny),
				Hosts: &[]string{"restricted.com"},
			},
		}

		result := merger.Merge(base, override)
		if result.Network == nil {
			t.Fatal("Expected non-nil Network")
		}
		if result.Network.Mode == nil || *result.Network.Mode != workspace.Deny {
			t.Errorf("Expected override deny mode to win, got %v", result.Network.Mode)
		}
		if result.Network.Hosts == nil || len(*result.Network.Hosts) != 1 || (*result.Network.Hosts)[0] != "restricted.com" {
			t.Errorf("Expected merged hosts, got %v", result.Network.Hosts)
		}
	})

	t.Run("override allow beats base deny", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode:  networkModePtr(workspace.Deny),
				Hosts: &[]string{"allowed.com"},
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode: networkModePtr(workspace.Allow),
			},
		}

		result := merger.Merge(base, override)
		if result.Network == nil {
			t.Fatal("Expected non-nil Network")
		}
		if result.Network.Mode == nil || *result.Network.Mode != workspace.Allow {
			t.Errorf("Expected override allow mode to win, got %v", result.Network.Mode)
		}
		// Hosts from base are still present in the union, but runtime ignores them in allow mode.
		if result.Network.Hosts == nil || len(*result.Network.Hosts) != 1 || (*result.Network.Hosts)[0] != "allowed.com" {
			t.Errorf("Expected base hosts preserved in union, got %v", result.Network.Hosts)
		}
	})

	t.Run("both allow", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode: networkModePtr(workspace.Allow),
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode: networkModePtr(workspace.Allow),
			},
		}

		result := merger.Merge(base, override)
		if result.Network == nil {
			t.Fatal("Expected non-nil Network")
		}
		if result.Network.Mode == nil || *result.Network.Mode != workspace.Allow {
			t.Errorf("Expected allow mode, got %v", result.Network.Mode)
		}
	})
}

func TestMerger_Merge_Network_BothDenyMerged(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("hosts merged", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode:  networkModePtr(workspace.Deny),
				Hosts: &[]string{"base.com", "shared.com"},
			},
		}
		override := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode:  networkModePtr(workspace.Deny),
				Hosts: &[]string{"override.com", "shared.com"},
			},
		}

		result := merger.Merge(base, override)
		if result.Network == nil {
			t.Fatal("Expected non-nil Network")
		}
		if result.Network.Mode == nil || *result.Network.Mode != workspace.Deny {
			t.Errorf("Expected deny mode, got %v", result.Network.Mode)
		}

		// Hosts: base.com, shared.com (from base), override.com (new from override)
		hosts := *result.Network.Hosts
		if len(hosts) != 3 {
			t.Fatalf("Expected 3 hosts, got %d: %v", len(hosts), hosts)
		}
		if hosts[0] != "base.com" || hosts[1] != "shared.com" || hosts[2] != "override.com" {
			t.Errorf("Unexpected hosts order: %v", hosts)
		}
	})
}

func TestMerger_Merge_Network_DeepCopy(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("mutating merged hosts does not affect base", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode:  networkModePtr(workspace.Deny),
				Hosts: &[]string{"base.com"},
			},
		}

		result := merger.Merge(base, nil)

		// Mutate the result
		(*result.Network.Hosts)[0] = "mutated.com"

		// Base must be unaffected
		if (*base.Network.Hosts)[0] != "base.com" {
			t.Error("Mutating merged network hosts affected the base input")
		}
	})

	t.Run("mutating merged mode does not affect base", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{
				Mode: networkModePtr(workspace.Allow),
			},
		}

		result := merger.Merge(base, nil)

		// Mutate the result
		*result.Network.Mode = workspace.Deny

		// Base must be unaffected
		if *base.Network.Mode != workspace.Allow {
			t.Error("Mutating merged network mode affected the base input")
		}
	})
}

func TestMerger_Merge_MCP_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	merger := NewMerger()
	base := &workspace.WorkspaceConfiguration{
		Environment: &[]workspace.EnvironmentVariable{
			{Name: "FOO", Value: strPtr("bar")},
		},
		Mcp: &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "tool-a", Command: "cmd-a"},
			},
		},
	}
	override := &workspace.WorkspaceConfiguration{
		Mcp: &workspace.McpConfiguration{
			Servers: &[]workspace.McpServer{
				{Name: "srv-a", Url: "https://a.example.com"},
			},
		},
	}

	result := merger.Merge(base, override)

	if result.Environment == nil || len(*result.Environment) != 1 {
		t.Error("Environment was not preserved during MCP merge")
	}
	if result.Mcp == nil {
		t.Fatal("Expected non-nil Mcp")
	}
	if result.Mcp.Commands == nil || len(*result.Mcp.Commands) != 1 {
		t.Error("Commands from base were not preserved")
	}
	if result.Mcp.Servers == nil || len(*result.Mcp.Servers) != 1 {
		t.Error("Servers from override were not added")
	}
}

func TestMerger_Features(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("base features preserved when override has none", func(t *testing.T) {
		t.Parallel()

		baseFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/go:1": {"version": "1.21"},
		}
		base := &workspace.WorkspaceConfiguration{Features: &baseFeats}

		result := merger.Merge(base, &workspace.WorkspaceConfiguration{})
		if result.Features == nil {
			t.Fatal("Expected Features to be preserved from base")
		}
		if _, ok := (*result.Features)["ghcr.io/devcontainers/features/go:1"]; !ok {
			t.Error("Expected go feature to be present in merged result")
		}
	})

	t.Run("override features added when base has none", func(t *testing.T) {
		t.Parallel()

		overrideFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		}
		override := &workspace.WorkspaceConfiguration{Features: &overrideFeats}

		result := merger.Merge(&workspace.WorkspaceConfiguration{}, override)
		if result.Features == nil {
			t.Fatal("Expected Features to be present from override")
		}
		if _, ok := (*result.Features)["ghcr.io/devcontainers/features/node:1"]; !ok {
			t.Error("Expected node feature to be present in merged result")
		}
	})

	t.Run("override feature options replace base for same ID", func(t *testing.T) {
		t.Parallel()

		baseFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/go:1": {"version": "1.20"},
		}
		overrideFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/go:1": {"version": "1.21"},
		}
		base := &workspace.WorkspaceConfiguration{Features: &baseFeats}
		override := &workspace.WorkspaceConfiguration{Features: &overrideFeats}

		result := merger.Merge(base, override)
		if result.Features == nil {
			t.Fatal("Expected Features to be present")
		}
		opts, ok := (*result.Features)["ghcr.io/devcontainers/features/go:1"]
		if !ok {
			t.Fatal("Expected go feature to be present")
		}
		if opts["version"] != "1.21" {
			t.Errorf("Expected version 1.21 (override wins), got %v", opts["version"])
		}
	})

	t.Run("features from both base and override are combined", func(t *testing.T) {
		t.Parallel()

		baseFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/go:1": {"version": "1.21"},
		}
		overrideFeats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		}
		base := &workspace.WorkspaceConfiguration{Features: &baseFeats}
		override := &workspace.WorkspaceConfiguration{Features: &overrideFeats}

		result := merger.Merge(base, override)
		if result.Features == nil {
			t.Fatal("Expected Features to be present")
		}
		if len(*result.Features) != 2 {
			t.Errorf("Expected 2 features, got %d", len(*result.Features))
		}
		if _, ok := (*result.Features)["ghcr.io/devcontainers/features/go:1"]; !ok {
			t.Error("Expected go feature from base to be present")
		}
		if _, ok := (*result.Features)["ghcr.io/devcontainers/features/node:1"]; !ok {
			t.Error("Expected node feature from override to be present")
		}
	})

	t.Run("nil features on both sides returns nil", func(t *testing.T) {
		t.Parallel()

		result := merger.Merge(&workspace.WorkspaceConfiguration{}, &workspace.WorkspaceConfiguration{})
		if result.Features != nil {
			t.Error("Expected nil Features when both sides have none")
		}
	})

	t.Run("copyConfig preserves features", func(t *testing.T) {
		t.Parallel()

		feats := map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/go:1": {"version": "1.21"},
		}
		cfg := &workspace.WorkspaceConfiguration{Features: &feats}

		result := merger.Merge(cfg, nil)
		if result.Features == nil {
			t.Fatal("Expected Features to be copied")
		}
		if _, ok := (*result.Features)["ghcr.io/devcontainers/features/go:1"]; !ok {
			t.Error("Expected go feature to survive copyConfig")
		}
	})
}

func TestMerger_Ports(t *testing.T) {
	t.Parallel()

	merger := NewMerger()

	t.Run("base ports preserved when override has none", func(t *testing.T) {
		t.Parallel()

		ports := []int{8080, 9090}
		base := &workspace.WorkspaceConfiguration{Ports: &ports}

		result := merger.Merge(base, &workspace.WorkspaceConfiguration{})
		if result.Ports == nil {
			t.Fatal("Expected Ports to be preserved from base")
		}
		if len(*result.Ports) != 2 {
			t.Errorf("Expected 2 ports, got %d", len(*result.Ports))
		}
	})

	t.Run("override ports added when base has none", func(t *testing.T) {
		t.Parallel()

		ports := []int{3000}
		override := &workspace.WorkspaceConfiguration{Ports: &ports}

		result := merger.Merge(&workspace.WorkspaceConfiguration{}, override)
		if result.Ports == nil {
			t.Fatal("Expected Ports to be present from override")
		}
		if len(*result.Ports) != 1 || (*result.Ports)[0] != 3000 {
			t.Errorf("Expected [3000], got %v", *result.Ports)
		}
	})

	t.Run("ports are union-merged and deduplicated", func(t *testing.T) {
		t.Parallel()

		basePorts := []int{8080, 9090}
		overridePorts := []int{9090, 3000}
		base := &workspace.WorkspaceConfiguration{Ports: &basePorts}
		override := &workspace.WorkspaceConfiguration{Ports: &overridePorts}

		result := merger.Merge(base, override)
		if result.Ports == nil {
			t.Fatal("Expected Ports to be present")
		}
		if len(*result.Ports) != 3 {
			t.Errorf("Expected 3 unique ports, got %d: %v", len(*result.Ports), *result.Ports)
		}
	})

	t.Run("nil ports when both nil", func(t *testing.T) {
		t.Parallel()

		base := &workspace.WorkspaceConfiguration{}
		override := &workspace.WorkspaceConfiguration{}

		result := merger.Merge(base, override)
		if result.Ports != nil {
			t.Errorf("Expected Ports to be nil, got %v", *result.Ports)
		}
	})

	t.Run("copyConfig copies ports independently", func(t *testing.T) {
		t.Parallel()

		ports := []int{8080}
		cfg := &workspace.WorkspaceConfiguration{Ports: &ports}

		result := merger.Merge(nil, cfg)
		if result.Ports == nil {
			t.Fatal("Expected Ports to be copied")
		}
		// Modify original; copy must be independent
		(*cfg.Ports)[0] = 9999
		if (*result.Ports)[0] == 9999 {
			t.Error("Expected Ports copy to be independent of original")
		}
	})
}
