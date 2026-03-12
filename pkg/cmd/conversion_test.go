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

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
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

		// Manually set ID (in real usage, Manager sets this)
		instanceData := instance.Dump()
		instanceData.ID = "test-id-456"
		instance, _ = instances.NewInstanceFromData(instanceData)

		result := instanceToWorkspace(instance)

		if result.Id != "test-id-456" {
			t.Errorf("Expected ID 'test-id-456', got '%s'", result.Id)
		}

		if result.Name != "test-workspace" {
			t.Errorf("Expected name 'test-workspace', got '%s'", result.Name)
		}

		if result.Paths.Source != sourceDir {
			t.Errorf("Expected source '%s', got '%s'", sourceDir, result.Paths.Source)
		}

		if result.Paths.Configuration != configDir {
			t.Errorf("Expected config '%s', got '%s'", configDir, result.Paths.Configuration)
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

		// Set ID
		instanceData := instance.Dump()
		instanceData.ID = "full-test-id"
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
		if _, exists := parsed["paths"]; !exists {
			t.Error("Expected 'paths' field in JSON")
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
