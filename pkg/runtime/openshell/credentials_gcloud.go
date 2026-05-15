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

// vertexAIRegions lists the Vertex AI regional endpoint prefixes.
// OpenShell policy does not support "*-" wildcards (only "*." or "**."),
// so we enumerate them explicitly.
var vertexAIRegions = []string{
	"us-central1",
	"us-east4",
	"us-east5",
	"us-west1",
	"us-west4",
	"europe-west1",
	"europe-west2",
	"europe-west4",
	"europe-west9",
	"asia-southeast1",
	"asia-northeast1",
	"asia-northeast3",
	"me-west1",
	"northamerica-northeast1",
}

// extraVertexAIHosts lists additional Google Cloud hosts required by the
// Vertex AI authentication and model serving flow beyond the regional
// aiplatform endpoints.
var extraVertexAIHosts = []string{
	"storage.googleapis.com",
}

func init() {
	registerHostExpander(expandVertexAIWildcards)
}

// expandVertexAIWildcards replaces the unsupported wildcard pattern
// "*-aiplatform.googleapis.com" with explicit regional endpoints and
// appends additional hosts required by the Vertex AI flow.
func expandVertexAIWildcards(hosts []string) []string {
	var result []string
	expanded := false
	for _, h := range hosts {
		if h == "*-aiplatform.googleapis.com" {
			for _, region := range vertexAIRegions {
				result = append(result, region+"-aiplatform.googleapis.com")
			}
			expanded = true
			continue
		}
		result = append(result, h)
	}
	if expanded {
		result = append(result, extraVertexAIHosts...)
	}
	return result
}
