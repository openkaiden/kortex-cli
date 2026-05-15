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

package openshell

import (
	"slices"
	"testing"
)

func TestExpandVertexAIWildcards(t *testing.T) {
	t.Parallel()

	t.Run("expands wildcard pattern", func(t *testing.T) {
		t.Parallel()

		input := []string{"oauth2.googleapis.com", "*-aiplatform.googleapis.com", "aiplatform.googleapis.com"}
		result := expandVertexAIWildcards(input)

		if slices.Contains(result, "*-aiplatform.googleapis.com") {
			t.Error("wildcard should have been expanded")
		}
		if !slices.Contains(result, "oauth2.googleapis.com") {
			t.Error("non-wildcard host should be preserved")
		}
		if !slices.Contains(result, "aiplatform.googleapis.com") {
			t.Error("non-wildcard host should be preserved")
		}
		if !slices.Contains(result, "us-central1-aiplatform.googleapis.com") {
			t.Error("expected us-central1 regional endpoint")
		}
		if !slices.Contains(result, "europe-west4-aiplatform.googleapis.com") {
			t.Error("expected europe-west4 regional endpoint")
		}
		if !slices.Contains(result, "storage.googleapis.com") {
			t.Error("expected storage.googleapis.com")
		}

		expectedLen := 2 + len(vertexAIRegions) + len(extraVertexAIHosts)
		if len(result) != expectedLen {
			t.Errorf("expected %d hosts, got %d", expectedLen, len(result))
		}
	})

	t.Run("passes through non-wildcard hosts", func(t *testing.T) {
		t.Parallel()

		input := []string{"example.com", "api.github.com"}
		result := expandVertexAIWildcards(input)

		if len(result) != 2 {
			t.Errorf("expected 2 hosts, got %d", len(result))
		}
	})

	t.Run("handles nil input", func(t *testing.T) {
		t.Parallel()

		result := expandVertexAIWildcards(nil)
		if len(result) != 0 {
			t.Errorf("expected empty result, got %v", result)
		}
	})
}
