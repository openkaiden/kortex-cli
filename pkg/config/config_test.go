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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates config with absolute path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Verify config was created successfully
		if cfg == nil {
			t.Error("Expected non-nil config")
		}
	})

	t.Run("converts relative path to absolute", func(t *testing.T) {
		t.Parallel()

		cfg, err := NewConfig(".")
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Verify config was created successfully
		if cfg == nil {
			t.Error("Expected non-nil config")
		}
	})

	t.Run("returns error for empty path", func(t *testing.T) {
		t.Parallel()

		_, err := NewConfig("")
		if err != ErrInvalidPath {
			t.Errorf("Expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestConfig_Load(t *testing.T) {
	t.Parallel()

	t.Run("loads valid workspace configuration", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write a valid workspace.json
		workspaceJSON := `{
  "environment": [
    {
      "name": "DEBUG",
      "value": "true"
    }
  ],
  "mounts": {
    "dependencies": ["../main"],
    "configs": [".ssh"]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load the configuration
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Verify the loaded configuration
		if workspaceCfg.Environment == nil {
			t.Fatal("Expected environment to be non-nil")
		}
		if len(*workspaceCfg.Environment) != 1 {
			t.Errorf("Expected 1 environment variable, got %d", len(*workspaceCfg.Environment))
		}
		if workspaceCfg.Mounts == nil {
			t.Fatal("Expected mounts to be non-nil")
		}
		if workspaceCfg.Mounts.Dependencies == nil || len(*workspaceCfg.Mounts.Dependencies) != 1 {
			t.Errorf("Expected 1 dependency mount")
		}
		if workspaceCfg.Mounts.Configs == nil || len(*workspaceCfg.Mounts.Configs) != 1 {
			t.Errorf("Expected 1 config mount")
		}
	})

	t.Run("returns ErrConfigNotFound when file doesn't exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory but no workspace.json
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		_, err = cfg.Load()
		if err != ErrConfigNotFound {
			t.Errorf("Expected ErrConfigNotFound, got %v", err)
		}
	})

	t.Run("returns ErrConfigNotFound when directory doesn't exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Don't create the directory
		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		_, err = cfg.Load()
		if err != ErrConfigNotFound {
			t.Errorf("Expected ErrConfigNotFound, got %v", err)
		}
	})

	t.Run("returns error for malformed JSON", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write invalid JSON
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte("{ invalid json }"), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		_, err = cfg.Load()
		if err == nil {
			t.Error("Expected error for malformed JSON, got nil")
		}
		if err == ErrConfigNotFound {
			t.Error("Expected JSON parsing error, got ErrConfigNotFound")
		}
	})

	t.Run("loads minimal configuration", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write a minimal workspace.json
		workspaceJSON := `{}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load the configuration
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Verify the loaded configuration
		if workspaceCfg.Environment != nil {
			t.Errorf("Expected nil environment, got %v", workspaceCfg.Environment)
		}
	})

	t.Run("rejects config with both value and secret set", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with both value and secret
		workspaceJSON := `{
  "environment": [
    {
      "name": "INVALID",
      "value": "some-value",
      "secret": "some-secret"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for invalid configuration, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("accepts config with only value set", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with only value
		workspaceJSON := `{
  "environment": [
    {
      "name": "WITH_VALUE",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should succeed
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if workspaceCfg.Environment == nil || len(*workspaceCfg.Environment) != 1 {
			t.Fatalf("Expected 1 environment variable")
		}
	})

	t.Run("accepts config with only secret set", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with only secret
		workspaceJSON := `{
  "environment": [
    {
      "name": "WITH_SECRET",
      "secret": "some-secret"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should succeed
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if workspaceCfg.Environment == nil || len(*workspaceCfg.Environment) != 1 {
			t.Fatalf("Expected 1 environment variable")
		}
	})

	t.Run("accepts config with empty value", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with empty value
		workspaceJSON := `{
  "environment": [
    {
      "name": "EMPTY_VALUE",
      "value": ""
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should succeed
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if workspaceCfg.Environment == nil || len(*workspaceCfg.Environment) != 1 {
			t.Fatalf("Expected 1 environment variable")
		}
	})

	t.Run("rejects config with empty secret", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with empty secret
		workspaceJSON := `{
  "environment": [
    {
      "name": "EMPTY_SECRET",
      "secret": ""
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for empty secret, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with neither value nor secret", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with neither value nor secret
		workspaceJSON := `{
  "environment": [
    {
      "name": "NO_VALUE_OR_SECRET"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for env var with neither value nor secret, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with empty env var name", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with empty name
		workspaceJSON := `{
  "environment": [
    {
      "name": "",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for empty env var name, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with env var name starting with digit", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with name starting with digit
		workspaceJSON := `{
  "environment": [
    {
      "name": "1INVALID",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for env var name starting with digit, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with env var name containing hyphen", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with name containing hyphen
		workspaceJSON := `{
  "environment": [
    {
      "name": "INVALID-NAME",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for env var name with hyphen, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with env var name containing space", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with name containing space
		workspaceJSON := `{
  "environment": [
    {
      "name": "INVALID NAME",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for env var name with space, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with env var name containing special characters", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with name containing special characters
		workspaceJSON := `{
  "environment": [
    {
      "name": "INVALID@NAME",
      "value": "some-value"
    }
  ]
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for env var name with special characters, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("accepts valid env var names", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name    string
			varName string
		}{
			{"uppercase", "DEBUG"},
			{"lowercase", "debug"},
			{"with underscore", "DEBUG_MODE"},
			{"starting with underscore", "_PRIVATE"},
			{"with numbers", "VAR_123"},
			{"mixed case", "MyVar_123"},
		}

		for _, tc := range testCases {
			tc := tc // capture range variable
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				tempDir := t.TempDir()
				configDir := filepath.Join(tempDir, ".kortex")

				// Create the config directory
				err := os.MkdirAll(configDir, 0755)
				if err != nil {
					t.Fatalf("os.MkdirAll() failed: %v", err)
				}

				// Write workspace.json with valid name
				workspaceJSON := fmt.Sprintf(`{
  "environment": [
    {
      "name": "%s",
      "value": "some-value"
    }
  ]
}`, tc.varName)
				workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
				err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
				if err != nil {
					t.Fatalf("os.WriteFile() failed: %v", err)
				}

				cfg, err := NewConfig(configDir)
				if err != nil {
					t.Fatalf("NewConfig() failed: %v", err)
				}

				// Load should succeed
				workspaceCfg, err := cfg.Load()
				if err != nil {
					t.Fatalf("Load() failed for valid name %q: %v", tc.varName, err)
				}

				if workspaceCfg.Environment == nil || len(*workspaceCfg.Environment) != 1 {
					t.Fatalf("Expected 1 environment variable")
				}
			})
		}
	})

	t.Run("rejects config with empty dependency path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with empty dependency path
		workspaceJSON := `{
  "mounts": {
    "dependencies": [""]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for empty dependency path, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with absolute dependency path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with absolute dependency path
		workspaceJSON := `{
  "mounts": {
    "dependencies": ["/absolute/path"]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for absolute dependency path, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with empty config path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with empty config path
		workspaceJSON := `{
  "mounts": {
    "configs": [""]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for empty config path, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("rejects config with absolute config path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with absolute config path
		workspaceJSON := `{
  "mounts": {
    "configs": ["/absolute/path"]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should fail with validation error
		_, err = cfg.Load()
		if err == nil {
			t.Fatal("Expected error for absolute config path, got nil")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("accepts valid relative mount paths", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".kortex")

		// Create the config directory
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("os.MkdirAll() failed: %v", err)
		}

		// Write workspace.json with valid relative paths
		workspaceJSON := `{
  "mounts": {
    "dependencies": ["../main", "../../lib"],
    "configs": [".ssh", ".config/gh"]
  }
}`
		workspacePath := filepath.Join(configDir, WorkspaceConfigFile)
		err = os.WriteFile(workspacePath, []byte(workspaceJSON), 0644)
		if err != nil {
			t.Fatalf("os.WriteFile() failed: %v", err)
		}

		cfg, err := NewConfig(configDir)
		if err != nil {
			t.Fatalf("NewConfig() failed: %v", err)
		}

		// Load should succeed
		workspaceCfg, err := cfg.Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if workspaceCfg.Mounts == nil {
			t.Fatal("Expected mounts to be non-nil")
		}
		if workspaceCfg.Mounts.Dependencies == nil || len(*workspaceCfg.Mounts.Dependencies) != 2 {
			t.Errorf("Expected 2 dependency mounts")
		}
		if workspaceCfg.Mounts.Configs == nil || len(*workspaceCfg.Mounts.Configs) != 2 {
			t.Errorf("Expected 2 config mounts")
		}
	})
}
