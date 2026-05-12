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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAgentConfigLoader(t *testing.T) {
	t.Parallel()

	t.Run("creates loader successfully", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if loader == nil {
			t.Error("Expected non-nil loader")
		}
	})

	t.Run("returns error for empty storage dir", func(t *testing.T) {
		t.Parallel()

		_, err := NewAgentConfigLoader("")
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("Expected ErrInvalidPath, got %v", err)
		}
	})

	t.Run("converts to absolute path", func(t *testing.T) {
		t.Parallel()

		loader, err := NewAgentConfigLoader(".")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Access the internal field to verify it's absolute
		impl := loader.(*agentConfigLoader)
		if !filepath.IsAbs(impl.storageDir) {
			t.Errorf("Expected absolute path, got %s", impl.storageDir)
		}
	})
}

func TestAgentConfigLoader_Load(t *testing.T) {
	t.Parallel()

	t.Run("returns empty config when file doesn't exist", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		cfg, err := loader.Load("claude")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Error("Expected non-nil config")
		}

		// Should be empty config
		if cfg.Environment != nil || cfg.Mounts != nil {
			t.Error("Expected empty config")
		}
	})

	t.Run("returns empty config when agent not found", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := filepath.Join(storageDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create agents.json with only "goose" agent
		agentsJSON := `{
  "goose": {
    "environment": [
      {
        "name": "GOOSE_VAR",
        "value": "goose-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, AgentsConfigFile), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		// Try to load "claude" which doesn't exist
		cfg, err := loader.Load("claude")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Error("Expected non-nil config")
		}

		// Should be empty config
		if cfg.Environment != nil || cfg.Mounts != nil {
			t.Error("Expected empty config")
		}
	})

	t.Run("loads agent config successfully", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := filepath.Join(storageDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		agentsJSON := `{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      },
      {
        "name": "API_KEY",
        "secret": "my-secret"
      }
    ],
    "mounts": [
      {"host": "$SOURCES/../shared", "target": "$SOURCES/../shared"},
      {"host": "$HOME/.claude", "target": "$HOME/.claude"}
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, AgentsConfigFile), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		cfg, err := loader.Load("claude")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected non-nil config")
		}

		// Check environment variables
		if cfg.Environment == nil || len(*cfg.Environment) != 2 {
			t.Fatalf("Expected 2 environment variables, got %v", cfg.Environment)
		}

		env := *cfg.Environment
		if env[0].Name != "DEBUG" || *env[0].Value != "true" {
			t.Error("Expected DEBUG=true")
		}
		if env[1].Name != "API_KEY" || *env[1].Secret != "my-secret" {
			t.Error("Expected API_KEY with secret")
		}

		// Check mounts
		if cfg.Mounts == nil {
			t.Fatal("Expected mounts to be set")
		}

		if len(*cfg.Mounts) != 2 {
			t.Errorf("Expected 2 mounts, got %d", len(*cfg.Mounts))
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := filepath.Join(storageDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Write invalid JSON
		if err := os.WriteFile(filepath.Join(configDir, AgentsConfigFile), []byte("not valid json"), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		_, err = loader.Load("claude")
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}

		if !errors.Is(err, ErrInvalidAgentConfig) {
			t.Errorf("Expected ErrInvalidAgentConfig, got %v", err)
		}
	})

	t.Run("returns error for invalid configuration", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := filepath.Join(storageDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create config with both value and secret (invalid)
		agentsJSON := `{
  "claude": {
    "environment": [
      {
        "name": "BAD_VAR",
        "value": "value",
        "secret": "secret"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, AgentsConfigFile), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		_, err = loader.Load("claude")
		if err == nil {
			t.Error("Expected error for invalid configuration")
		}

		if !errors.Is(err, ErrInvalidAgentConfig) {
			t.Errorf("Expected ErrInvalidAgentConfig, got %v", err)
		}
	})

	t.Run("returns error for empty agent name", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		_, err = loader.Load("")
		if err == nil {
			t.Error("Expected error for empty agent name")
		}

		if !errors.Is(err, ErrInvalidAgentConfig) {
			t.Errorf("Expected ErrInvalidAgentConfig, got %v", err)
		}
	})

	t.Run("loads multiple agents from same file", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := filepath.Join(storageDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		agentsJSON := `{
  "claude": {
    "environment": [{"name": "CLAUDE_VAR", "value": "claude-value"}]
  },
  "goose": {
    "environment": [{"name": "GOOSE_VAR", "value": "goose-value"}]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, AgentsConfigFile), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		loader, err := NewAgentConfigLoader(storageDir)
		if err != nil {
			t.Fatalf("Failed to create loader: %v", err)
		}

		// Load claude
		claudeCfg, err := loader.Load("claude")
		if err != nil {
			t.Errorf("Failed to load claude config: %v", err)
		}
		if claudeCfg.Environment == nil || len(*claudeCfg.Environment) != 1 {
			t.Error("Expected claude environment")
		}

		// Load goose
		gooseCfg, err := loader.Load("goose")
		if err != nil {
			t.Errorf("Failed to load goose config: %v", err)
		}
		if gooseCfg.Environment == nil || len(*gooseCfg.Environment) != 1 {
			t.Error("Expected goose environment")
		}
	})
}

func TestAgentConfigLoader_ModuleDesignPattern(t *testing.T) {
	t.Parallel()

	t.Run("interface can be implemented", func(t *testing.T) {
		t.Parallel()

		var _ AgentConfigLoader = (*agentConfigLoader)(nil)
	})
}

func TestNewAgentConfigUpdater(t *testing.T) {
	t.Parallel()

	t.Run("creates updater successfully", func(t *testing.T) {
		t.Parallel()

		u, err := NewAgentConfigUpdater(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u == nil {
			t.Error("expected non-nil updater")
		}
	})

	t.Run("returns error for empty storage dir", func(t *testing.T) {
		t.Parallel()

		_, err := NewAgentConfigUpdater("")
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestAgentConfigUpdater_AddEnvVar(t *testing.T) {
	t.Parallel()

	t.Run("creates file when absent", func(t *testing.T) {
		t.Parallel()

		u, _ := NewAgentConfigUpdater(t.TempDir())
		if err := u.AddEnvVar("claude", "MY_VAR", "hello"); err != nil {
			t.Fatalf("AddEnvVar: %v", err)
		}

		loader, _ := NewAgentConfigLoader(u.(*agentConfigUpdater).storageDir)
		cfg, err := loader.Load("claude")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Environment == nil || len(*cfg.Environment) != 1 {
			t.Fatalf("expected 1 env var, got %v", cfg.Environment)
		}
		if (*cfg.Environment)[0].Name != "MY_VAR" || *(*cfg.Environment)[0].Value != "hello" {
			t.Errorf("unexpected env var: %+v", (*cfg.Environment)[0])
		}
	})

	t.Run("appends to existing env vars", func(t *testing.T) {
		t.Parallel()

		u, _ := NewAgentConfigUpdater(t.TempDir())
		if err := u.AddEnvVar("claude", "A", "1"); err != nil {
			t.Fatalf("AddEnvVar A: %v", err)
		}
		if err := u.AddEnvVar("claude", "B", "2"); err != nil {
			t.Fatalf("AddEnvVar B: %v", err)
		}

		loader, _ := NewAgentConfigLoader(u.(*agentConfigUpdater).storageDir)
		cfg, _ := loader.Load("claude")
		if len(*cfg.Environment) != 2 {
			t.Errorf("expected 2 env vars, got %d", len(*cfg.Environment))
		}
	})

	t.Run("updates existing env var value", func(t *testing.T) {
		t.Parallel()

		u, _ := NewAgentConfigUpdater(t.TempDir())
		if err := u.AddEnvVar("claude", "MY_VAR", "old"); err != nil {
			t.Fatalf("AddEnvVar first: %v", err)
		}
		if err := u.AddEnvVar("claude", "MY_VAR", "new"); err != nil {
			t.Fatalf("AddEnvVar second: %v", err)
		}

		loader, _ := NewAgentConfigLoader(u.(*agentConfigUpdater).storageDir)
		cfg, _ := loader.Load("claude")
		if len(*cfg.Environment) != 1 {
			t.Errorf("expected 1 env var (no duplicate), got %d", len(*cfg.Environment))
		}
		if *(*cfg.Environment)[0].Value != "new" {
			t.Errorf("expected value=new, got %q", *(*cfg.Environment)[0].Value)
		}
	})

	t.Run("does not affect other agents", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		u, _ := NewAgentConfigUpdater(storageDir)
		if err := u.AddEnvVar("claude", "MY_VAR", "hello"); err != nil {
			t.Fatalf("AddEnvVar: %v", err)
		}

		loader, _ := NewAgentConfigLoader(storageDir)
		gooseCfg, _ := loader.Load("goose")
		if gooseCfg.Environment != nil {
			t.Error("expected goose to have no env vars")
		}
	})
}

func TestAgentConfigUpdater_AddMount(t *testing.T) {
	t.Parallel()

	t.Run("creates file when absent", func(t *testing.T) {
		t.Parallel()

		u, _ := NewAgentConfigUpdater(t.TempDir())
		if err := u.AddMount("claude", "$HOME/.foo", "$HOME/.foo", true); err != nil {
			t.Fatalf("AddMount: %v", err)
		}

		loader, _ := NewAgentConfigLoader(u.(*agentConfigUpdater).storageDir)
		cfg, _ := loader.Load("claude")
		if cfg.Mounts == nil || len(*cfg.Mounts) != 1 {
			t.Fatalf("expected 1 mount, got %v", cfg.Mounts)
		}
		m := (*cfg.Mounts)[0]
		if m.Host != "$HOME/.foo" || m.Target != "$HOME/.foo" {
			t.Errorf("unexpected mount: %+v", m)
		}
		if m.Ro == nil || !*m.Ro {
			t.Error("expected mount to be read-only")
		}
	})

	t.Run("idempotent on same host+target", func(t *testing.T) {
		t.Parallel()

		u, _ := NewAgentConfigUpdater(t.TempDir())
		if err := u.AddMount("claude", "$HOME/.foo", "$HOME/.foo", true); err != nil {
			t.Fatalf("AddMount first: %v", err)
		}
		if err := u.AddMount("claude", "$HOME/.foo", "$HOME/.foo", true); err != nil {
			t.Fatalf("AddMount second: %v", err)
		}

		loader, _ := NewAgentConfigLoader(u.(*agentConfigUpdater).storageDir)
		cfg, _ := loader.Load("claude")
		if len(*cfg.Mounts) != 1 {
			t.Errorf("expected 1 mount after duplicate, got %d", len(*cfg.Mounts))
		}
	})
}

// TestAgentConfigUpdater_Schema_AddedOnCreation verifies that $schema is written
// when agents.json is created for the first time.
func TestAgentConfigUpdater_Schema_AddedOnCreation(t *testing.T) {
	t.Parallel()

	u, _ := NewAgentConfigUpdater(t.TempDir())
	if err := u.AddEnvVar("claude", "MY_VAR", "hello"); err != nil {
		t.Fatalf("AddEnvVar: %v", err)
	}

	storageDir := u.(*agentConfigUpdater).storageDir
	data, err := os.ReadFile(filepath.Join(storageDir, "config", AgentsConfigFile))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var schemaURL string
	if err := json.Unmarshal(raw["$schema"], &schemaURL); err != nil {
		t.Fatalf("$schema missing or not a string: %v", err)
	}
	if schemaURL != AgentsSchemaURL {
		t.Errorf("expected %q, got %q", AgentsSchemaURL, schemaURL)
	}
}

// TestAgentConfigUpdater_Schema_PreservedOnUpdate verifies that $schema survives
// subsequent writes and is NOT added to files that were created without it.
func TestAgentConfigUpdater_Schema_PreservedOnUpdate(t *testing.T) {
	t.Parallel()

	// File created by kdn: $schema must survive a second write.
	u, _ := NewAgentConfigUpdater(t.TempDir())
	if err := u.AddEnvVar("claude", "A", "1"); err != nil {
		t.Fatalf("AddEnvVar A: %v", err)
	}
	if err := u.AddEnvVar("claude", "B", "2"); err != nil {
		t.Fatalf("AddEnvVar B: %v", err)
	}

	storageDir := u.(*agentConfigUpdater).storageDir
	data, _ := os.ReadFile(filepath.Join(storageDir, "config", AgentsConfigFile))
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw["$schema"]; !ok {
		t.Error("$schema must be preserved after subsequent writes")
	}

	// Pre-existing file without $schema: must NOT gain $schema on update.
	dir2 := t.TempDir()
	configDir2 := filepath.Join(dir2, "config")
	_ = os.MkdirAll(configDir2, 0700)
	existing := []byte(`{"claude":{"environment":[{"name":"X","value":"1"}]}}`)
	_ = os.WriteFile(filepath.Join(configDir2, AgentsConfigFile), existing, 0600)

	u2, _ := NewAgentConfigUpdater(dir2)
	if err := u2.AddEnvVar("claude", "Y", "2"); err != nil {
		t.Fatalf("AddEnvVar on pre-existing: %v", err)
	}

	data2, err := os.ReadFile(filepath.Join(dir2, "config", AgentsConfigFile))
	if err != nil {
		t.Fatalf("reading pre-existing agents.json: %v", err)
	}
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(data2, &raw2); err != nil {
		t.Fatalf("parsing pre-existing agents.json: %v", err)
	}
	if _, ok := raw2["$schema"]; ok {
		t.Error("$schema must not be added to a pre-existing file")
	}
}
