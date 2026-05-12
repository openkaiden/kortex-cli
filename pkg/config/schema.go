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
	"fmt"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

const (
	// WorkspaceSchemaURL is the JSON Schema URL for .kaiden/workspace.json.
	WorkspaceSchemaURL = "https://github.com/openkaiden/kdn/releases/latest/download/workspace.json"
	// AgentsSchemaURL is the JSON Schema URL for ~/.kdn/config/agents.json.
	AgentsSchemaURL = "https://github.com/openkaiden/kdn/releases/latest/download/agents.json"
	// ProjectsSchemaURL is the JSON Schema URL for ~/.kdn/config/projects.json.
	ProjectsSchemaURL = "https://github.com/openkaiden/kdn/releases/latest/download/projects.json"
)

// parseWorkspaceConfigMap parses a JSON object that may contain a "$schema" key
// alongside workspace configuration entries. The "$schema" value is returned
// separately; all other keys are parsed as workspace.WorkspaceConfiguration values.
func parseWorkspaceConfigMap(data []byte) (map[string]workspace.WorkspaceConfiguration, string, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, "", err
	}

	var schemaURL string
	if s, ok := rawMap["$schema"]; ok {
		_ = json.Unmarshal(s, &schemaURL)
		delete(rawMap, "$schema")
	}

	result := make(map[string]workspace.WorkspaceConfiguration, len(rawMap))
	for k, v := range rawMap {
		var cfg workspace.WorkspaceConfiguration
		if err := json.Unmarshal(v, &cfg); err != nil {
			return nil, "", fmt.Errorf("key %q: %w", k, err)
		}
		result[k] = cfg
	}
	return result, schemaURL, nil
}

// marshalWorkspaceConfigMap marshals the map with an optional "$schema" key.
// Because Go sorts map keys alphabetically and "$" sorts before all lowercase
// letters, "$schema" always appears first in the output.
func marshalWorkspaceConfigMap(configs map[string]workspace.WorkspaceConfiguration, schemaURL string) ([]byte, error) {
	if schemaURL == "" {
		return json.MarshalIndent(configs, "", "  ")
	}

	rawMap := make(map[string]json.RawMessage, len(configs)+1)
	// json.Marshal never fails for a plain string value, so the error is ignored.
	schemaRaw, _ := json.Marshal(schemaURL)
	rawMap["$schema"] = schemaRaw
	for k, v := range configs {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		rawMap[k] = raw
	}
	return json.MarshalIndent(rawMap, "", "  ")
}
