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
	"errors"
	"fmt"
	"testing"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/instances"
	"github.com/spf13/cobra"
)

func TestInstanceToWorkspaceId(t *testing.T) {
	t.Parallel()

	t.Run("converts instance to workspace ID", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
			Name:      "test-workspace",
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Manually set ID (in real usage, Manager sets this)
		instanceData := instance.Dump()
		instanceData.ID = "test-id-123"
		instance, _ = instances.NewInstanceFromData(instanceData)

		result := instanceToWorkspaceId(instance)

		if result.Id != "test-id-123" {
			t.Errorf("Expected ID 'test-id-123', got '%s'", result.Id)
		}

		// Verify it only contains ID field by marshaling and checking JSON structure
		jsonData, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Failed to marshal result: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(jsonData, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["id"]; !exists {
			t.Error("Expected 'id' field in JSON")
		}
	})
}

func TestInstanceToWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("converts instance to full workspace", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
			Name:      "test-workspace",
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Manually set ID and Project (in real usage, Manager sets these)
		instanceData := instance.Dump()
		instanceData.ID = "test-id-456"
		instanceData.Project = "test-project"
		instance, _ = instances.NewInstanceFromData(instanceData)

		result := instanceToWorkspace(instance)

		if result.Id != "test-id-456" {
			t.Errorf("Expected ID 'test-id-456', got '%s'", result.Id)
		}

		if result.Name != "test-workspace" {
			t.Errorf("Expected name 'test-workspace', got '%s'", result.Name)
		}

		if result.Project != "test-project" {
			t.Errorf("Expected project 'test-project', got '%s'", result.Project)
		}

		if result.Paths.Source != sourceDir {
			t.Errorf("Expected source '%s', got '%s'", sourceDir, result.Paths.Source)
		}

		if result.Paths.Configuration != configDir {
			t.Errorf("Expected config '%s', got '%s'", configDir, result.Paths.Configuration)
		}
	})

	t.Run("sets model field when model is set", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "model-test-id",
			Name:  "model-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			Agent: "claude",
			Model: "claude-sonnet-4-20250514",
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		if result.Model == nil {
			t.Fatal("Expected Model to be set, got nil")
		}
		if *result.Model != "claude-sonnet-4-20250514" {
			t.Errorf("Expected Model %q, got %q", "claude-sonnet-4-20250514", *result.Model)
		}
	})

	t.Run("omits model field when model is empty", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "no-model-id",
			Name:  "no-model-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			Agent: "claude",
			// Model intentionally empty
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		if result.Model != nil {
			t.Errorf("Expected Model to be nil, got %q", *result.Model)
		}

		// Verify model field is absent from JSON (omitempty)
		jsonData, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Failed to marshal result: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(jsonData, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		if _, exists := parsed["model"]; exists {
			t.Error("Expected 'model' field to be absent from JSON when not set")
		}
	})

	t.Run("includes created timestamp from instance", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()
		createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

		instanceData := instances.InstanceData{
			ID:        "ts-id",
			Name:      "ts-workspace",
			Paths:     instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			CreatedAt: createdAt,
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		expectedMs := createdAt.UnixMilli()
		if result.Timestamps.Created != expectedMs {
			t.Errorf("Expected Timestamps.Created %d, got %d", expectedMs, result.Timestamps.Created)
		}
		if result.Timestamps.Started != nil {
			t.Errorf("Expected Timestamps.Started to be nil, got %d", *result.Timestamps.Started)
		}
	})

	t.Run("includes started timestamp when set", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()
		createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
		startedAt := time.Date(2026, 1, 15, 10, 5, 0, 0, time.UTC)

		instanceData := instances.InstanceData{
			ID:        "ts-running-id",
			Name:      "ts-running-workspace",
			Paths:     instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			CreatedAt: createdAt,
			StartedAt: startedAt,
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		expectedStartedMs := startedAt.UnixMilli()
		if result.Timestamps.Started == nil {
			t.Fatal("Expected Timestamps.Started to be set")
		}
		if *result.Timestamps.Started != expectedStartedMs {
			t.Errorf("Expected Timestamps.Started %d, got %d", expectedStartedMs, *result.Timestamps.Started)
		}
	})

	t.Run("includes all required fields", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
			Name:      "my-workspace",
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Set ID and Agent
		instanceData := instance.Dump()
		instanceData.ID = "full-test-id"
		instanceData.Agent = "claude"
		instance, _ = instances.NewInstanceFromData(instanceData)

		result := instanceToWorkspace(instance)

		// Marshal to JSON to verify structure
		jsonData, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Failed to marshal result: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(jsonData, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify all expected fields exist
		if _, exists := parsed["id"]; !exists {
			t.Error("Expected 'id' field in JSON")
		}
		if _, exists := parsed["name"]; !exists {
			t.Error("Expected 'name' field in JSON")
		}
		if _, exists := parsed["project"]; !exists {
			t.Error("Expected 'project' field in JSON")
		}
		if _, exists := parsed["agent"]; !exists {
			t.Error("Expected 'agent' field in JSON")
		}
		if _, exists := parsed["paths"]; !exists {
			t.Error("Expected 'paths' field in JSON")
		}
		if _, exists := parsed["timestamps"]; !exists {
			t.Error("Expected 'timestamps' field in JSON")
		}

		// Verify paths structure
		paths, ok := parsed["paths"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected 'paths' to be an object")
		}

		if _, exists := paths["source"]; !exists {
			t.Error("Expected 'paths.source' field in JSON")
		}
		if _, exists := paths["configuration"]; !exists {
			t.Error("Expected 'paths.configuration' field in JSON")
		}

		// Verify timestamps structure
		timestamps, ok := parsed["timestamps"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected 'timestamps' to be an object")
		}
		if _, exists := timestamps["created"]; !exists {
			t.Error("Expected 'timestamps.created' field in JSON")
		}
	})
}

func TestFormatErrorJSON(t *testing.T) {
	t.Parallel()

	t.Run("formats error as JSON", func(t *testing.T) {
		t.Parallel()

		err := errors.New("something went wrong")
		result, jsonErr := formatErrorJSON(err)

		if jsonErr != nil {
			t.Fatalf("formatErrorJSON failed: %v", jsonErr)
		}

		// Parse the JSON to verify structure
		var errorResponse api.Error
		if err := json.Unmarshal([]byte(result), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v", err)
		}

		if errorResponse.Error != "something went wrong" {
			t.Errorf("Expected error message 'something went wrong', got '%s'", errorResponse.Error)
		}
	})

	t.Run("includes only error field", func(t *testing.T) {
		t.Parallel()

		err := errors.New("test error")
		result, jsonErr := formatErrorJSON(err)

		if jsonErr != nil {
			t.Fatalf("formatErrorJSON failed: %v", jsonErr)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		if len(parsed) != 1 {
			t.Errorf("Expected only 1 field, got %d: %v", len(parsed), parsed)
		}

		if _, exists := parsed["error"]; !exists {
			t.Error("Expected 'error' field in JSON")
		}
	})

	t.Run("handles complex error messages", func(t *testing.T) {
		t.Parallel()

		complexErr := errors.New("failed to process: invalid input \"test\"\nUse --help for more information")
		result, jsonErr := formatErrorJSON(complexErr)

		if jsonErr != nil {
			t.Fatalf("formatErrorJSON failed: %v", jsonErr)
		}

		var errorResponse api.Error
		if err := json.Unmarshal([]byte(result), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal error JSON: %v", err)
		}

		expectedMsg := "failed to process: invalid input \"test\"\nUse --help for more information"
		if errorResponse.Error != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, errorResponse.Error)
		}
	})
}

func TestOutputErrorIfJSON(t *testing.T) {
	t.Parallel()

	t.Run("outputs JSON when output is json", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		testErr := errors.New("test error message")
		returnedErr := outputErrorIfJSON(cmd, "json", testErr)

		// Verify the error is returned unchanged
		if returnedErr != testErr {
			t.Errorf("Expected returned error to be the same as input error")
		}

		// Verify JSON was written to output
		var errorResponse api.Error
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal JSON output: %v\nOutput was: %s", err, buf.String())
		}

		if errorResponse.Error != "test error message" {
			t.Errorf("Expected error message 'test error message', got '%s'", errorResponse.Error)
		}
	})

	t.Run("does not output JSON when output is empty", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		testErr := errors.New("test error message")
		returnedErr := outputErrorIfJSON(cmd, "", testErr)

		// Verify the error is returned unchanged
		if returnedErr != testErr {
			t.Errorf("Expected returned error to be the same as input error")
		}

		// Verify nothing was written to output
		if buf.Len() != 0 {
			t.Errorf("Expected no output, got: %s", buf.String())
		}
	})

	t.Run("does not output JSON when output is text", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		testErr := errors.New("test error message")
		returnedErr := outputErrorIfJSON(cmd, "text", testErr)

		// Verify the error is returned unchanged
		if returnedErr != testErr {
			t.Errorf("Expected returned error to be the same as input error")
		}

		// Verify nothing was written to output
		if buf.Len() != 0 {
			t.Errorf("Expected no output, got: %s", buf.String())
		}
	})

	t.Run("formats complex error messages correctly", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		testErr := errors.New("workspace not found: abc123\nUse 'workspace list' to see available workspaces")
		returnedErr := outputErrorIfJSON(cmd, "json", testErr)

		// Verify the error is returned unchanged
		if returnedErr != testErr {
			t.Errorf("Expected returned error to be the same as input error")
		}

		// Verify JSON was written with full error message
		var errorResponse api.Error
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal JSON output: %v", err)
		}

		expectedMsg := "workspace not found: abc123\nUse 'workspace list' to see available workspaces"
		if errorResponse.Error != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, errorResponse.Error)
		}
	})

	t.Run("handles wrapped errors", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		baseErr := errors.New("base error")
		wrappedErr := fmt.Errorf("wrapped error: %w", baseErr)
		returnedErr := outputErrorIfJSON(cmd, "json", wrappedErr)

		// Verify the error is returned unchanged
		if returnedErr != wrappedErr {
			t.Errorf("Expected returned error to be the same as input error")
		}

		// Verify JSON contains the full wrapped error message
		var errorResponse api.Error
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			t.Fatalf("Failed to unmarshal JSON output: %v", err)
		}

		if errorResponse.Error != "wrapped error: base error" {
			t.Errorf("Expected error message 'wrapped error: base error', got '%s'", errorResponse.Error)
		}
	})
}

func TestInstanceToWorkspace_Forwards(t *testing.T) {
	t.Parallel()

	t.Run("populates forwards from runtime info", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "fwd-test-id",
			Name:  "fwd-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			Runtime: instances.RuntimeData{
				Info: map[string]string{
					"forwards": `[{"bind":"127.0.0.1","port":54321,"target":8080}]`,
				},
			},
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		if len(result.Forwards) != 1 {
			t.Fatalf("Expected 1 forward, got %d", len(result.Forwards))
		}
		fwd := result.Forwards[0]
		if fwd.Bind != "127.0.0.1" {
			t.Errorf("Expected Bind '127.0.0.1', got '%s'", fwd.Bind)
		}
		if fwd.Port != 54321 {
			t.Errorf("Expected Port 54321, got %d", fwd.Port)
		}
		if fwd.Target != 8080 {
			t.Errorf("Expected Target 8080, got %d", fwd.Target)
		}
	})

	t.Run("returns empty forwards when no runtime info", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "no-fwd-id",
			Name:  "no-fwd-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		if result.Forwards == nil {
			t.Fatal("Expected Forwards to be non-nil (empty slice)")
		}
		if len(result.Forwards) != 0 {
			t.Errorf("Expected 0 forwards, got %d", len(result.Forwards))
		}
	})

	t.Run("returns empty forwards when forwards JSON is invalid", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "bad-fwd-id",
			Name:  "bad-fwd-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			Runtime: instances.RuntimeData{
				Info: map[string]string{
					"forwards": "not-valid-json",
				},
			},
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		if result.Forwards == nil {
			t.Fatal("Expected Forwards to be non-nil (empty slice)")
		}
		if len(result.Forwards) != 0 {
			t.Errorf("Expected 0 forwards when JSON is invalid, got %d", len(result.Forwards))
		}
	})

	t.Run("forwards field appears in JSON output", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		configDir := t.TempDir()

		instanceData := instances.InstanceData{
			ID:    "json-fwd-id",
			Name:  "json-fwd-workspace",
			Paths: instances.InstancePaths{Source: sourceDir, Configuration: configDir},
			Runtime: instances.RuntimeData{
				Info: map[string]string{
					"forwards": `[{"bind":"127.0.0.1","port":12345,"target":3000},{"bind":"127.0.0.1","port":12346,"target":9090}]`,
				},
			},
		}
		instance, err := instances.NewInstanceFromData(instanceData)
		if err != nil {
			t.Fatalf("Failed to create instance from data: %v", err)
		}

		result := instanceToWorkspace(instance)

		jsonData, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Failed to marshal result: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(jsonData, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		forwards, ok := parsed["forwards"].([]interface{})
		if !ok {
			t.Fatal("Expected 'forwards' field to be an array in JSON")
		}
		if len(forwards) != 2 {
			t.Errorf("Expected 2 forwards in JSON, got %d", len(forwards))
		}
	})
}
