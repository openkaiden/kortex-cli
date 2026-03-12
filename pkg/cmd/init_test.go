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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/cmd/testutil"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

func TestInitCmd_PreRun(t *testing.T) {
	t.Run("default arguments", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != "." {
			t.Errorf("Expected sourcesDir to be '.', got %s", c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(".")
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		expectedConfigDir := filepath.Join(".", ".kortex")
		if c.workspaceConfigDir != expectedConfigDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(expectedConfigDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("with sources directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcesDir := t.TempDir()

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{sourcesDir}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != sourcesDir {
			t.Errorf("Expected sourcesDir to be %s, got %s", sourcesDir, c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		expectedConfigDir := filepath.Join(sourcesDir, ".kortex")
		if c.workspaceConfigDir != expectedConfigDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(expectedConfigDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("with workspace configuration flag", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := t.TempDir()

		c := &initCmd{
			workspaceConfigDir: configDir,
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().Set("workspace-configuration", configDir)
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != "." {
			t.Errorf("Expected sourcesDir to be '.', got %s", c.sourcesDir)
		}

		if c.workspaceConfigDir != configDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", configDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("with both arguments", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcesDir := t.TempDir()
		configDir := t.TempDir()

		c := &initCmd{
			workspaceConfigDir: configDir,
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().Set("workspace-configuration", configDir)
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{sourcesDir}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != sourcesDir {
			t.Errorf("Expected sourcesDir to be %s, got %s", sourcesDir, c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		if c.workspaceConfigDir != configDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", configDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("relative sources directory", func(t *testing.T) {
		// Note: Not using t.Parallel() because this test changes the working directory,
		// which affects the entire process and could interfere with other parallel tests.

		storageDir := t.TempDir()
		workDir := t.TempDir()
		relativePath := filepath.Join(".", "relative", "path")

		// Save current working directory
		origWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current working directory: %v", err)
		}

		// Change to temp directory
		if err := os.Chdir(workDir); err != nil {
			t.Fatalf("Failed to change to temp directory: %v", err)
		}
		defer func() {
			// Restore original working directory
			if err := os.Chdir(origWd); err != nil {
				t.Errorf("Failed to restore working directory: %v", err)
			}
		}()

		// Create the relative directory in the temp working directory
		if err := os.MkdirAll(relativePath, 0755); err != nil {
			t.Fatalf("Failed to create relative directory: %v", err)
		}

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", storageDir, "test storage flag")

		args := []string{relativePath}

		err = c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != relativePath {
			t.Errorf("Expected sourcesDir to be %s, got %s", relativePath, c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(relativePath)
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		expectedConfigDir := filepath.Join(relativePath, ".kortex")
		if c.workspaceConfigDir != expectedConfigDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
		}
	})

	t.Run("fails when sources directory does not exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		nonExistentDir := filepath.Join(tempDir, "does-not-exist")

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{nonExistentDir}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with non-existent directory")
		}

		if !strings.Contains(err.Error(), "sources directory does not exist") {
			t.Errorf("Expected error to contain 'sources directory does not exist', got: %v", err)
		}
	})

	t.Run("fails when sources path is a file not a directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		regularFile := filepath.Join(tempDir, "regular-file.txt")

		// Create a regular file
		if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{regularFile}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail when sources path is a file")
		}

		if !strings.Contains(err.Error(), "sources path is not a directory") {
			t.Errorf("Expected error to contain 'sources path is not a directory', got: %v", err)
		}
	})

	t.Run("accepts empty output flag", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		c := &initCmd{
			output: "", // Default empty output
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.output != "" {
			t.Errorf("Expected output to be empty, got %s", c.output)
		}
	})

	t.Run("accepts json output format", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		c := &initCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.output != "json" {
			t.Errorf("Expected output to be 'json', got %s", c.output)
		}
	})

	t.Run("rejects invalid output format", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		c := &initCmd{
			output: "xml",
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid output format")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
		if !strings.Contains(err.Error(), "xml") {
			t.Errorf("Expected error to mention 'xml', got: %v", err)
		}
	})

	t.Run("outputs JSON error when manager creation fails with json output", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		// Create a file and try to use it as a parent directory - will fail cross-platform
		notADir := filepath.Join(tempDir, "file")
		if err := os.WriteFile(notADir, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		invalidStorage := filepath.Join(notADir, "subdir")

		c := &initCmd{
			output: "json",
		}
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", invalidStorage, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with invalid storage path")
		}

		// Verify JSON error was output
		var errorResponse api.Error
		if jsonErr := json.Unmarshal(buf.Bytes(), &errorResponse); jsonErr != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v\nOutput was: %s", jsonErr, buf.String())
		}

		if !strings.Contains(errorResponse.Error, "failed to create manager") {
			t.Errorf("Expected error to contain 'failed to create manager', got: %s", errorResponse.Error)
		}
	})
}

func TestInitCmd_E2E(t *testing.T) {
	t.Parallel()

	t.Run("registers workspace with default arguments", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance was created
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		// Verify instance has a non-empty ID
		if inst.GetID() == "" {
			t.Error("Expected instance to have a non-empty ID")
		}

		// Verify instance has a non-empty Name
		if inst.GetName() == "" {
			t.Error("Expected instance to have a non-empty Name")
		}

		// Verify output contains only the ID (default non-verbose output)
		output := strings.TrimSpace(buf.String())
		if output != inst.GetID() {
			t.Errorf("Expected output to be just the ID %s, got: %s", inst.GetID(), output)
		}

		// Verify sources directory is current directory (absolute)
		expectedAbsSourcesDir, _ := filepath.Abs(".")
		if inst.GetSourceDir() != expectedAbsSourcesDir {
			t.Errorf("Expected source dir %s, got %s", expectedAbsSourcesDir, inst.GetSourceDir())
		}

		// Verify config directory defaults to .kortex in current directory
		expectedConfigDir := filepath.Join(expectedAbsSourcesDir, ".kortex")
		if inst.GetConfigDir() != expectedConfigDir {
			t.Errorf("Expected config dir %s, got %s", expectedConfigDir, inst.GetConfigDir())
		}

		// Verify paths are absolute
		if !filepath.IsAbs(inst.GetSourceDir()) {
			t.Errorf("Expected source dir to be absolute, got %s", inst.GetSourceDir())
		}
		if !filepath.IsAbs(inst.GetConfigDir()) {
			t.Errorf("Expected config dir to be absolute, got %s", inst.GetConfigDir())
		}
	})

	t.Run("registers workspace with custom sources directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance was created with correct paths
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		// Verify instance has a non-empty ID
		if inst.GetID() == "" {
			t.Error("Expected instance to have a non-empty ID")
		}

		// Verify output contains only the ID (default non-verbose output)
		output := strings.TrimSpace(buf.String())
		if output != inst.GetID() {
			t.Errorf("Expected output to be just the ID %s, got: %s", inst.GetID(), output)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if inst.GetSourceDir() != expectedAbsSourcesDir {
			t.Errorf("Expected source dir %s, got %s", expectedAbsSourcesDir, inst.GetSourceDir())
		}

		expectedConfigDir := filepath.Join(expectedAbsSourcesDir, ".kortex")
		if inst.GetConfigDir() != expectedConfigDir {
			t.Errorf("Expected config dir %s, got %s", expectedConfigDir, inst.GetConfigDir())
		}

		// Verify paths are absolute
		if !filepath.IsAbs(inst.GetSourceDir()) {
			t.Errorf("Expected source dir to be absolute, got %s", inst.GetSourceDir())
		}
		if !filepath.IsAbs(inst.GetConfigDir()) {
			t.Errorf("Expected config dir to be absolute, got %s", inst.GetConfigDir())
		}
	})

	t.Run("registers workspace with custom configuration directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		configDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", "--workspace-configuration", configDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance was created with correct paths
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		// Verify instance has a non-empty ID
		if inst.GetID() == "" {
			t.Error("Expected instance to have a non-empty ID")
		}

		// Verify output contains only the ID (default non-verbose output)
		output := strings.TrimSpace(buf.String())
		if output != inst.GetID() {
			t.Errorf("Expected output to be just the ID %s, got: %s", inst.GetID(), output)
		}

		// Verify sources directory defaults to current directory
		expectedAbsSourcesDir, _ := filepath.Abs(".")
		if inst.GetSourceDir() != expectedAbsSourcesDir {
			t.Errorf("Expected source dir %s, got %s", expectedAbsSourcesDir, inst.GetSourceDir())
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if inst.GetConfigDir() != expectedAbsConfigDir {
			t.Errorf("Expected config dir %s, got %s", expectedAbsConfigDir, inst.GetConfigDir())
		}

		// Verify paths are absolute
		if !filepath.IsAbs(inst.GetSourceDir()) {
			t.Errorf("Expected source dir to be absolute, got %s", inst.GetSourceDir())
		}
		if !filepath.IsAbs(inst.GetConfigDir()) {
			t.Errorf("Expected config dir to be absolute, got %s", inst.GetConfigDir())
		}
	})

	t.Run("registers workspace with both custom directories", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		configDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--workspace-configuration", configDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance was created with correct paths
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		// Verify instance has a non-empty ID
		if inst.GetID() == "" {
			t.Error("Expected instance to have a non-empty ID")
		}

		// Verify output contains only the ID (default non-verbose output)
		output := strings.TrimSpace(buf.String())
		if output != inst.GetID() {
			t.Errorf("Expected output to be just the ID %s, got: %s", inst.GetID(), output)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if inst.GetSourceDir() != expectedAbsSourcesDir {
			t.Errorf("Expected source dir %s, got %s", expectedAbsSourcesDir, inst.GetSourceDir())
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if inst.GetConfigDir() != expectedAbsConfigDir {
			t.Errorf("Expected config dir %s, got %s", expectedAbsConfigDir, inst.GetConfigDir())
		}

		// Verify paths are absolute
		if !filepath.IsAbs(inst.GetSourceDir()) {
			t.Errorf("Expected source dir to be absolute, got %s", inst.GetSourceDir())
		}
		if !filepath.IsAbs(inst.GetConfigDir()) {
			t.Errorf("Expected config dir to be absolute, got %s", inst.GetConfigDir())
		}
	})

	t.Run("registers multiple workspaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir1 := t.TempDir()
		sourcesDir2 := t.TempDir()

		// Register first workspace
		rootCmd1 := NewRootCmd()
		buf1 := new(bytes.Buffer)
		rootCmd1.SetOut(buf1)
		rootCmd1.SetArgs([]string{"--storage", storageDir, "init", sourcesDir1})

		err := rootCmd1.Execute()
		if err != nil {
			t.Fatalf("Execute() failed for first workspace: %v", err)
		}

		// Register second workspace
		rootCmd2 := NewRootCmd()
		buf2 := new(bytes.Buffer)
		rootCmd2.SetOut(buf2)
		rootCmd2.SetArgs([]string{"--storage", storageDir, "init", sourcesDir2})

		err = rootCmd2.Execute()
		if err != nil {
			t.Fatalf("Execute() failed for second workspace: %v", err)
		}

		// Verify both instances exist
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 2 {
			t.Fatalf("Expected 2 instances, got %d", len(instancesList))
		}

		// Verify both instances have unique IDs
		if instancesList[0].GetID() == "" || instancesList[1].GetID() == "" {
			t.Error("Expected both instances to have non-empty IDs")
		}
		if instancesList[0].GetID() == instancesList[1].GetID() {
			t.Error("Expected instances to have unique IDs")
		}

		// Verify both instances have correct source directories
		expectedAbsSourcesDir1, _ := filepath.Abs(sourcesDir1)
		expectedAbsSourcesDir2, _ := filepath.Abs(sourcesDir2)

		foundDir1 := false
		foundDir2 := false
		for _, inst := range instancesList {
			if inst.GetSourceDir() == expectedAbsSourcesDir1 {
				foundDir1 = true
				// Verify config dir for first workspace
				expectedConfigDir1 := filepath.Join(expectedAbsSourcesDir1, ".kortex")
				if inst.GetConfigDir() != expectedConfigDir1 {
					t.Errorf("Expected config dir %s for first workspace, got %s", expectedConfigDir1, inst.GetConfigDir())
				}
			}
			if inst.GetSourceDir() == expectedAbsSourcesDir2 {
				foundDir2 = true
				// Verify config dir for second workspace
				expectedConfigDir2 := filepath.Join(expectedAbsSourcesDir2, ".kortex")
				if inst.GetConfigDir() != expectedConfigDir2 {
					t.Errorf("Expected config dir %s for second workspace, got %s", expectedConfigDir2, inst.GetConfigDir())
				}
			}

			// Verify paths are absolute
			if !filepath.IsAbs(inst.GetSourceDir()) {
				t.Errorf("Expected source dir to be absolute, got %s", inst.GetSourceDir())
			}
			if !filepath.IsAbs(inst.GetConfigDir()) {
				t.Errorf("Expected config dir to be absolute, got %s", inst.GetConfigDir())
			}
		}

		if !foundDir1 {
			t.Errorf("Expected to find instance with source dir %s", expectedAbsSourcesDir1)
		}
		if !foundDir2 {
			t.Errorf("Expected to find instance with source dir %s", expectedAbsSourcesDir2)
		}
	})

	t.Run("registers workspace with verbose flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--verbose"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		output := buf.String()

		// Verify verbose output contains expected strings
		if !strings.Contains(output, "Registered workspace:") {
			t.Errorf("Expected verbose output to contain 'Registered workspace:', got: %s", output)
		}
		if !strings.Contains(output, "ID:") {
			t.Errorf("Expected verbose output to contain 'ID:', got: %s", output)
		}
		if !strings.Contains(output, "Sources directory:") {
			t.Errorf("Expected verbose output to contain 'Sources directory:', got: %s", output)
		}
		if !strings.Contains(output, "Configuration directory:") {
			t.Errorf("Expected verbose output to contain 'Configuration directory:', got: %s", output)
		}

		// Verify instance was created with correct paths
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		// Verify verbose output contains the actual values
		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if !strings.Contains(output, expectedAbsSourcesDir) {
			t.Errorf("Expected verbose output to contain sources directory %s, got: %s", expectedAbsSourcesDir, output)
		}

		expectedConfigDir := filepath.Join(expectedAbsSourcesDir, ".kortex")
		if !strings.Contains(output, expectedConfigDir) {
			t.Errorf("Expected verbose output to contain config directory %s, got: %s", expectedConfigDir, output)
		}

		if !strings.Contains(output, inst.GetID()) {
			t.Errorf("Expected verbose output to contain instance ID %s, got: %s", inst.GetID(), output)
		}
	})

	t.Run("generates default name from source directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance name is generated from source directory
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]
		expectedName := filepath.Base(sourcesDir)

		if inst.GetName() != expectedName {
			t.Errorf("Expected name %s, got %s", expectedName, inst.GetName())
		}
	})

	t.Run("uses custom name from flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		customName := "my-workspace"

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--name", customName})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify instance name is the custom name
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instancesList))
		}

		inst := instancesList[0]

		if inst.GetName() != customName {
			t.Errorf("Expected name %s, got %s", customName, inst.GetName())
		}
	})

	t.Run("generates unique names with increments", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		// Create three temp directories with the same base name pattern
		parentDir := t.TempDir()
		sourcesDir1 := filepath.Join(parentDir, "project")
		sourcesDir2 := filepath.Join(parentDir, "project-other")
		sourcesDir3 := filepath.Join(parentDir, "project-another")

		// Create the directories
		if err := os.MkdirAll(sourcesDir1, 0755); err != nil {
			t.Fatalf("Failed to create sourcesDir1: %v", err)
		}
		if err := os.MkdirAll(sourcesDir2, 0755); err != nil {
			t.Fatalf("Failed to create sourcesDir2: %v", err)
		}
		if err := os.MkdirAll(sourcesDir3, 0755); err != nil {
			t.Fatalf("Failed to create sourcesDir3: %v", err)
		}

		// Register first workspace with name "project"
		rootCmd1 := NewRootCmd()
		buf1 := new(bytes.Buffer)
		rootCmd1.SetOut(buf1)
		rootCmd1.SetArgs([]string{"--storage", storageDir, "init", sourcesDir1})

		err := rootCmd1.Execute()
		if err != nil {
			t.Fatalf("Execute() failed for first workspace: %v", err)
		}

		// Register second workspace with the same name "project" (should become "project-2")
		rootCmd2 := NewRootCmd()
		buf2 := new(bytes.Buffer)
		rootCmd2.SetOut(buf2)
		rootCmd2.SetArgs([]string{"--storage", storageDir, "init", sourcesDir2, "--name", "project"})

		err = rootCmd2.Execute()
		if err != nil {
			t.Fatalf("Execute() failed for second workspace: %v", err)
		}

		// Register third workspace with the same name "project" (should become "project-3")
		rootCmd3 := NewRootCmd()
		buf3 := new(bytes.Buffer)
		rootCmd3.SetOut(buf3)
		rootCmd3.SetArgs([]string{"--storage", storageDir, "init", sourcesDir3, "--name", "project"})

		err = rootCmd3.Execute()
		if err != nil {
			t.Fatalf("Execute() failed for third workspace: %v", err)
		}

		// Verify all three instances have unique names
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 3 {
			t.Fatalf("Expected 3 instances, got %d", len(instancesList))
		}

		// Verify names are unique
		names := make(map[string]bool)
		for _, inst := range instancesList {
			if names[inst.GetName()] {
				t.Errorf("Duplicate name found: %s", inst.GetName())
			}
			names[inst.GetName()] = true
		}

		// Verify expected names are present
		expectedNames := []string{"project", "project-2", "project-3"}
		for _, expectedName := range expectedNames {
			if !names[expectedName] {
				t.Errorf("Expected name %s not found in instances", expectedName)
			}
		}
	})

	t.Run("verbose output includes name", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		customName := "my-workspace"

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--name", customName, "--verbose"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		output := buf.String()

		// Verify verbose output contains the name
		if !strings.Contains(output, "Name:") {
			t.Errorf("Expected verbose output to contain 'Name:', got: %s", output)
		}
		if !strings.Contains(output, customName) {
			t.Errorf("Expected verbose output to contain name %s, got: %s", customName, output)
		}
	})

	t.Run("fails when sources directory does not exist", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		nonExistentDir := filepath.Join(storageDir, "does-not-exist")

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", nonExistentDir})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected Execute() to fail with non-existent directory")
		}

		if !strings.Contains(err.Error(), "sources directory does not exist") {
			t.Errorf("Expected error to contain 'sources directory does not exist', got: %v", err)
		}

		// Verify no instance was created
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances, got %d", len(instancesList))
		}
	})

	t.Run("fails when sources path is a file not a directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		regularFile := filepath.Join(storageDir, "regular-file.txt")

		// Create a regular file
		if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", regularFile})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected Execute() to fail when sources path is a file")
		}

		if !strings.Contains(err.Error(), "sources path is not a directory") {
			t.Errorf("Expected error to contain 'sources path is not a directory', got: %v", err)
		}

		// Verify no instance was created
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instancesList, err := manager.List()
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		if len(instancesList) != 0 {
			t.Errorf("Expected 0 instances, got %d", len(instancesList))
		}
	})

	t.Run("json output returns workspace ID by default", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--output", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Parse JSON output
		var workspaceId api.WorkspaceId
		if err := json.Unmarshal(buf.Bytes(), &workspaceId); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify ID is not empty
		if workspaceId.Id == "" {
			t.Error("Expected non-empty ID in JSON output")
		}

		// Verify only ID field exists
		var parsed map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal to map: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field in JSON, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["id"]; !exists {
			t.Error("Expected 'id' field in JSON")
		}

		// Verify instance was actually created
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := manager.Get(workspaceId.Id)
		if err != nil {
			t.Fatalf("Failed to get instance: %v", err)
		}

		if instance.GetID() != workspaceId.Id {
			t.Errorf("Expected instance ID %s, got %s", workspaceId.Id, instance.GetID())
		}
	})

	t.Run("json output with verbose returns full workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--output", "json", "--verbose"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Parse JSON output
		var workspace api.Workspace
		if err := json.Unmarshal(buf.Bytes(), &workspace); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify all fields are populated
		if workspace.Id == "" {
			t.Error("Expected non-empty ID in JSON output")
		}

		if workspace.Name == "" {
			t.Error("Expected non-empty Name in JSON output")
		}

		if workspace.Paths.Source == "" {
			t.Error("Expected non-empty Source path in JSON output")
		}

		if workspace.Paths.Configuration == "" {
			t.Error("Expected non-empty Configuration path in JSON output")
		}

		// Verify paths are absolute
		if !filepath.IsAbs(workspace.Paths.Source) {
			t.Errorf("Expected absolute source path, got %s", workspace.Paths.Source)
		}

		if !filepath.IsAbs(workspace.Paths.Configuration) {
			t.Errorf("Expected absolute configuration path, got %s", workspace.Paths.Configuration)
		}

		// Verify all expected fields exist
		var parsed map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal to map: %v", err)
		}

		if _, exists := parsed["id"]; !exists {
			t.Error("Expected 'id' field in JSON")
		}
		if _, exists := parsed["name"]; !exists {
			t.Error("Expected 'name' field in JSON")
		}
		if _, exists := parsed["paths"]; !exists {
			t.Error("Expected 'paths' field in JSON")
		}
	})

	t.Run("json output error for non-existent directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		nonExistentDir := filepath.Join(storageDir, "does-not-exist")

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", nonExistentDir, "--output", "json"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected Execute() to fail with non-existent directory")
		}

		// Parse JSON error output
		var errorResponse api.Error
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v", err)
		}

		// Verify error message
		if !strings.Contains(errorResponse.Error, "sources directory does not exist") {
			t.Errorf("Expected error to contain 'sources directory does not exist', got: %s", errorResponse.Error)
		}

		// Verify only error field exists
		var parsed map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal to map: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field in error JSON, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["error"]; !exists {
			t.Error("Expected 'error' field in JSON")
		}
	})

	t.Run("json output uses custom name", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		customName := "my-custom-workspace"

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", sourcesDir, "--name", customName, "--output", "json", "--verbose"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Parse JSON output
		var workspace api.Workspace
		if err := json.Unmarshal(buf.Bytes(), &workspace); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		if workspace.Name != customName {
			t.Errorf("Expected name %s in JSON output, got %s", customName, workspace.Name)
		}
	})
}

func TestInitCmd_Examples(t *testing.T) {
	t.Parallel()

	// Get the init command
	initCmd := NewInitCmd()

	// Verify Example field is not empty
	if initCmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	// Parse the examples
	commands, err := testutil.ParseExampleCommands(initCmd.Example)
	if err != nil {
		t.Fatalf("Failed to parse examples: %v", err)
	}

	// Verify we have the expected number of examples
	expectedCount := 4
	if len(commands) != expectedCount {
		t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
	}

	// Validate all examples against the root command
	rootCmd := NewRootCmd()
	err = testutil.ValidateCommandExamples(rootCmd, initCmd.Example)
	if err != nil {
		t.Errorf("Example validation failed: %v", err)
	}
}
