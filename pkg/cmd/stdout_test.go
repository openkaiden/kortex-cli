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
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
)

// TestCommands_OutputToStdout verifies that all commands output data to stdout (not stderr)
// when not in JSON mode. This is important for shell script compatibility.
func TestCommands_OutputToStdout(t *testing.T) {
	t.Parallel()

	t.Run("init command outputs ID to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", "--runtime", "fake", "--agent", "test-agent", sourcesDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		if stdoutContent == "" {
			t.Error("Expected output in stdout, got empty string")
		}

		// Should contain a workspace ID (64 character hex string)
		if len(strings.TrimSpace(stdoutContent)) != 64 {
			t.Errorf("Expected 64 character ID in stdout, got: %q", stdoutContent)
		}
	})

	t.Run("init command with verbose outputs to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", "--runtime", "fake", "--agent", "test-agent", "--verbose", sourcesDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		if !strings.Contains(stdoutContent, "Registered workspace:") {
			t.Errorf("Expected verbose output in stdout, got: %q", stdoutContent)
		}
		if !strings.Contains(stdoutContent, "ID:") {
			t.Errorf("Expected ID in stdout, got: %q", stdoutContent)
		}
		if !strings.Contains(stdoutContent, "Name:") {
			t.Errorf("Expected Name in stdout, got: %q", stdoutContent)
		}
	})

	t.Run("list command outputs to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// First, create a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
			Name:      "test-workspace",
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		_, err = manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now test list command
		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "list"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		if !strings.Contains(stdoutContent, "SHORT ID") {
			t.Errorf("Expected table header 'SHORT ID' in stdout, got: %q", stdoutContent)
		}
		if !strings.Contains(stdoutContent, "test-workspace") {
			t.Errorf("Expected workspace name in stdout, got: %q", stdoutContent)
		}
	})

	t.Run("list command with no workspaces outputs to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "list"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		if !strings.Contains(stdoutContent, "No workspaces registered") {
			t.Errorf("Expected message in stdout, got: %q", stdoutContent)
		}
	})

	t.Run("start command outputs ID to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// First, create a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now test start command
		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "start", added.GetID()})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		expectedID := added.GetID() + "\n"
		if stdoutContent != expectedID {
			t.Errorf("Expected ID in stdout, got: %q (expected: %q)", stdoutContent, expectedID)
		}
	})

	t.Run("stop command outputs ID to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// First, create and start a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}
		err = manager.Start(context.Background(), added.GetID())
		if err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		// Now test stop command
		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "stop", added.GetID()})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		expectedID := added.GetID() + "\n"
		if stdoutContent != expectedID {
			t.Errorf("Expected ID in stdout, got: %q (expected: %q)", stdoutContent, expectedID)
		}
	})

	t.Run("init JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "init", "--runtime", "fake", "--agent", "test-agent", "--output", "json", sourcesDir})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("list JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "list", "--output", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("info JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"info", "--output", "json"})

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("start JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "start", added.GetID(), "--output", "json"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("stop JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}
		err = manager.Start(context.Background(), added.GetID())
		if err != nil {
			t.Fatalf("Failed to start instance: %v", err)
		}

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "stop", added.GetID(), "--output", "json"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("remove JSON has clean stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "remove", added.GetID(), "--output", "json"})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		if stderr.Len() != 0 {
			t.Errorf("Expected clean stderr on JSON success, got: %q", stderr.String())
		}
	})

	t.Run("remove command outputs ID to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// First, create a workspace
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kaiden"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		added, err := manager.Add(context.Background(), instances.AddOptions{
			Instance:    instance,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now test remove command
		rootCmd := NewRootCmd()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"--storage", storageDir, "remove", added.GetID()})

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify output is in stdout
		stdoutContent := stdout.String()
		expectedID := added.GetID() + "\n"
		if stdoutContent != expectedID {
			t.Errorf("Expected ID in stdout, got: %q (expected: %q)", stdoutContent, expectedID)
		}
	})
}
