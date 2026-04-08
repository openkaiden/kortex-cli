/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmd_Initialization(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	if rootCmd.Use != "kdn" {
		t.Errorf("Expected Use to be 'kdn', got '%s'", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestExecute_WithHelp(t *testing.T) {
	t.Parallel()

	// Redirect output to avoid cluttering test output
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	// Call Execute() and verify it succeeds
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
}

func TestExecute_NoArgs(t *testing.T) {
	t.Parallel()

	// Redirect output to avoid cluttering test output
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{})

	// Call Execute() and verify it succeeds
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
}

func TestRootCmd_StorageFlag(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()

	// Check that the flag exists
	flag := rootCmd.PersistentFlags().Lookup("storage")
	if flag == nil {
		t.Fatal("Expected --storage flag to exist")
	}

	// Verify the flag has a default value
	if flag.DefValue == "" {
		t.Error("Expected --storage flag to have a default value")
	}

	// Verify the default value ends with .kdn
	if !strings.HasSuffix(flag.DefValue, ".kdn") {
		t.Errorf("Expected default value to end with '.kdn', got '%s'", flag.DefValue)
	}
}

func TestRootCmd_StorageFlagCustomValue(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom", "path", "storage")
	rootCmd.SetArgs([]string{"--storage", customPath, "version"})

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Verify the flag value was set correctly
	storagePath, err := rootCmd.PersistentFlags().GetString("storage")
	if err != nil {
		t.Fatalf("Failed to get storage flag: %v", err)
	}

	if storagePath != customPath {
		t.Errorf("Expected storage to be '%s', got '%s'", customPath, storagePath)
	}
}

func TestRootCmd_StorageFlagInSubcommand(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()

	// Find the version subcommand
	versionCmd, _, err := rootCmd.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Failed to find version command: %v", err)
	}

	// Verify the flag is inherited by subcommands
	flag := versionCmd.InheritedFlags().Lookup("storage")
	if flag == nil {
		t.Error("Expected --storage flag to be inherited by subcommands")
	}
}

func TestRootCmd_StorageFlagMissingValue(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Provide the flag without a value
	rootCmd.SetArgs([]string{"--storage"})

	// Execute the command and expect an error
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected Execute() to fail when --storage flag is provided without a value")
	}

	// Verify the error message indicates a flag parsing error
	errMsg := err.Error()
	if !strings.Contains(errMsg, "flag") && !strings.Contains(errMsg, "argument") {
		t.Errorf("Expected error message to contain 'flag' or 'argument', got: %s", errMsg)
	}
}

func TestRootCmd_StorageEnvVariable(t *testing.T) {
	t.Run("env variable sets default", func(t *testing.T) {
		// Set the environment variable
		envPath := filepath.Join(t.TempDir(), "from-env")
		t.Setenv("KDN_STORAGE", envPath)

		rootCmd := NewRootCmd()
		flag := rootCmd.PersistentFlags().Lookup("storage")
		if flag == nil {
			t.Fatal("Expected --storage flag to exist")
		}

		// Verify the default value is from the environment variable
		if flag.DefValue != envPath {
			t.Errorf("Expected default value to be '%s' (from env var), got '%s'", envPath, flag.DefValue)
		}
	})

	t.Run("flag overrides env variable", func(t *testing.T) {
		// Set the environment variable
		envPath := filepath.Join(t.TempDir(), "from-env")
		t.Setenv("KDN_STORAGE", envPath)

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// Set the flag explicitly
		flagPath := filepath.Join(t.TempDir(), "from-flag")
		rootCmd.SetArgs([]string{"--storage", flagPath, "version"})

		// Execute the command
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify the flag value overrides the env var
		storagePath, err := rootCmd.PersistentFlags().GetString("storage")
		if err != nil {
			t.Fatalf("Failed to get storage flag: %v", err)
		}

		if storagePath != flagPath {
			t.Errorf("Expected storage to be '%s' (from flag), got '%s'", flagPath, storagePath)
		}
	})

	t.Run("default used when env var not set", func(t *testing.T) {
		// Explicitly unset the environment variable (in case it was set in the shell)
		t.Setenv("KDN_STORAGE", "")

		rootCmd := NewRootCmd()
		flag := rootCmd.PersistentFlags().Lookup("storage")
		if flag == nil {
			t.Fatal("Expected --storage flag to exist")
		}

		// Verify the default value ends with .kdn
		if !strings.HasSuffix(flag.DefValue, ".kdn") {
			t.Errorf("Expected default value to end with '.kdn', got '%s'", flag.DefValue)
		}
	})
}
