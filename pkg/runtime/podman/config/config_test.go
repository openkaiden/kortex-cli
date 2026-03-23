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

func TestNewConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates config with valid path", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected non-nil config")
		}
	})

	t.Run("returns error for empty path", func(t *testing.T) {
		t.Parallel()

		_, err := NewConfig("")
		if err == nil {
			t.Fatal("Expected error for empty path")
		}

		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("Expected ErrInvalidPath, got: %v", err)
		}
	})

	t.Run("converts to absolute path", func(t *testing.T) {
		t.Parallel()

		cfg, err := NewConfig(".")
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// The internal path should be absolute
		c := cfg.(*config)
		if !filepath.IsAbs(c.path) {
			t.Errorf("Expected absolute path, got: %s", c.path)
		}
	})
}

func TestGenerateDefaults(t *testing.T) {
	t.Parallel()

	t.Run("creates config directory if missing", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, "config")

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		err = cfg.GenerateDefaults()
		if err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("Config directory was not created")
		}
	})

	t.Run("creates default image config", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		err = cfg.GenerateDefaults()
		if err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Verify image.json exists
		imageConfigPath := filepath.Join(configDir, ImageConfigFileName)
		if _, err := os.Stat(imageConfigPath); os.IsNotExist(err) {
			t.Error("image.json was not created")
		}

		// Verify content is valid JSON
		data, err := os.ReadFile(imageConfigPath)
		if err != nil {
			t.Fatalf("Failed to read image config: %v", err)
		}

		var imageConfig ImageConfig
		if err := json.Unmarshal(data, &imageConfig); err != nil {
			t.Fatalf("Failed to parse image config: %v", err)
		}

		// Verify default values
		if imageConfig.Version != DefaultVersion {
			t.Errorf("Expected version %s, got: %s", DefaultVersion, imageConfig.Version)
		}
	})

	t.Run("creates default claude config", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		err = cfg.GenerateDefaults()
		if err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Verify claude.json exists
		claudeConfigPath := filepath.Join(configDir, ClaudeConfigFileName)
		if _, err := os.Stat(claudeConfigPath); os.IsNotExist(err) {
			t.Error("claude.json was not created")
		}

		// Verify content is valid JSON
		data, err := os.ReadFile(claudeConfigPath)
		if err != nil {
			t.Fatalf("Failed to read claude config: %v", err)
		}

		var agentConfig AgentConfig
		if err := json.Unmarshal(data, &agentConfig); err != nil {
			t.Fatalf("Failed to parse claude config: %v", err)
		}

		// Verify terminal command is set
		if len(agentConfig.TerminalCommand) == 0 {
			t.Error("Expected terminal command to be set")
		}
	})

	t.Run("does not overwrite existing configs", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Create a custom image config
		customImageConfig := &ImageConfig{
			Version:  "40",
			Packages: []string{"custom-package"},
			Sudo:     []string{"/usr/bin/custom"},
		}
		imageConfigPath := filepath.Join(configDir, ImageConfigFileName)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}
		data, _ := json.MarshalIndent(customImageConfig, "", "  ")
		if err := os.WriteFile(imageConfigPath, data, 0644); err != nil {
			t.Fatalf("Failed to write custom config: %v", err)
		}

		// Call GenerateDefaults
		err = cfg.GenerateDefaults()
		if err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Verify the custom config was not overwritten
		loadedConfig, err := cfg.LoadImage()
		if err != nil {
			t.Fatalf("LoadImage() failed: %v", err)
		}

		if loadedConfig.Version != "40" {
			t.Errorf("Expected version 40, got: %s (config was overwritten)", loadedConfig.Version)
		}
	})
}

func TestLoadImage(t *testing.T) {
	t.Parallel()

	t.Run("loads valid image config", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Generate defaults
		if err := cfg.GenerateDefaults(); err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Load the config
		imageConfig, err := cfg.LoadImage()
		if err != nil {
			t.Fatalf("LoadImage() failed: %v", err)
		}

		if imageConfig == nil {
			t.Fatal("Expected non-nil image config")
		}

		if imageConfig.Version == "" {
			t.Error("Expected version to be set")
		}
	})

	t.Run("returns ErrConfigNotFound if file missing", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Don't generate defaults - file doesn't exist
		_, err = cfg.LoadImage()
		if err == nil {
			t.Fatal("Expected error for missing config file")
		}

		if !errors.Is(err, ErrConfigNotFound) {
			t.Errorf("Expected ErrConfigNotFound, got: %v", err)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Create directory and write invalid JSON
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		imageConfigPath := filepath.Join(configDir, ImageConfigFileName)
		if err := os.WriteFile(imageConfigPath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		// Attempt to load
		_, err = cfg.LoadImage()
		if err == nil {
			t.Fatal("Expected error for invalid JSON")
		}
	})

	t.Run("returns ErrInvalidConfig for validation failure", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Create invalid config (empty version)
		invalidConfig := &ImageConfig{
			Version:  "", // Invalid - empty version
			Packages: []string{},
			Sudo:     []string{},
		}

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		imageConfigPath := filepath.Join(configDir, ImageConfigFileName)
		data, _ := json.MarshalIndent(invalidConfig, "", "  ")
		if err := os.WriteFile(imageConfigPath, data, 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		// Attempt to load
		_, err = cfg.LoadImage()
		if err == nil {
			t.Fatal("Expected error for invalid config")
		}

		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got: %v", err)
		}
	})
}

func TestLoadAgent(t *testing.T) {
	t.Parallel()

	t.Run("loads valid agent config", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Generate defaults
		if err := cfg.GenerateDefaults(); err != nil {
			t.Fatalf("GenerateDefaults() failed: %v", err)
		}

		// Load the agent config
		agentConfig, err := cfg.LoadAgent("claude")
		if err != nil {
			t.Fatalf("LoadAgent() failed: %v", err)
		}

		if agentConfig == nil {
			t.Fatal("Expected non-nil agent config")
		}

		if len(agentConfig.TerminalCommand) == 0 {
			t.Error("Expected terminal command to be set")
		}
	})

	t.Run("returns error for empty agent name", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Try to load with empty agent name
		_, err = cfg.LoadAgent("")
		if err == nil {
			t.Fatal("Expected error for empty agent name")
		}

		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got: %v", err)
		}
	})

	t.Run("returns error for invalid agent name characters", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Test various invalid agent names
		invalidNames := []string{
			"../etc/passwd",     // Path traversal
			"agent-name",        // Hyphen not allowed
			"agent.name",        // Dot not allowed
			"agent/name",        // Slash not allowed
			"agent\\name",       // Backslash not allowed
			"agent name",        // Space not allowed
			"agent@name",        // Special char not allowed
			".",                 // Current directory
			"..",                // Parent directory
			"agent-1",           // Hyphen not allowed
			"my-agent",          // Hyphen not allowed
		}

		for _, name := range invalidNames {
			_, err = cfg.LoadAgent(name)
			if err == nil {
				t.Errorf("Expected error for invalid agent name %q", name)
				continue
			}

			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("Expected ErrInvalidConfig for %q, got: %v", name, err)
			}
		}
	})

	t.Run("accepts valid agent names", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Create config directory
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Test various valid agent names
		validNames := []string{
			"claude",
			"goose",
			"cursor",
			"agent123",
			"my_agent",
			"AGENT",
			"Agent_1",
			"_agent",
			"agent_",
		}

		for _, name := range validNames {
			// Create a valid config file for this agent
			agentConfig := &AgentConfig{
				Packages:        []string{},
				RunCommands:     []string{},
				TerminalCommand: []string{name},
			}
			agentConfigPath := filepath.Join(configDir, name+".json")
			data, _ := json.MarshalIndent(agentConfig, "", "  ")
			if err := os.WriteFile(agentConfigPath, data, 0644); err != nil {
				t.Fatalf("Failed to write config for %q: %v", name, err)
			}

			// Try to load it - should succeed
			_, err = cfg.LoadAgent(name)
			if err != nil {
				t.Errorf("Expected valid agent name %q to succeed, got error: %v", name, err)
			}
		}
	})

	t.Run("returns ErrConfigNotFound if file missing", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Don't generate defaults - file doesn't exist
		_, err = cfg.LoadAgent("nonexistent")
		if err == nil {
			t.Fatal("Expected error for missing config file")
		}

		if !errors.Is(err, ErrConfigNotFound) {
			t.Errorf("Expected ErrConfigNotFound, got: %v", err)
		}
	})

	t.Run("returns ErrInvalidConfig for validation failure", func(t *testing.T) {
		t.Parallel()

		configDir := t.TempDir()

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Create invalid config (empty terminal command)
		invalidConfig := &AgentConfig{
			Packages:        []string{},
			RunCommands:     []string{},
			TerminalCommand: []string{}, // Invalid - must have at least one element
		}

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		agentConfigPath := filepath.Join(configDir, "test.json")
		data, _ := json.MarshalIndent(invalidConfig, "", "  ")
		if err := os.WriteFile(agentConfigPath, data, 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		// Attempt to load
		_, err = cfg.LoadAgent("test")
		if err == nil {
			t.Fatal("Expected error for invalid config")
		}

		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got: %v", err)
		}
	})
}
