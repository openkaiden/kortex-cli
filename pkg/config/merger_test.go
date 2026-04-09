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
