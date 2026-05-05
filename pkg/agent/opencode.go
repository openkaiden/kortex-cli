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

package agent

import (
	"encoding/json"
	"fmt"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	kdnconfig "github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/containerurl"
)

const (
	// OpenCodeConfigPath is the relative path to the OpenCode configuration file.
	OpenCodeConfigPath = ".config/opencode/opencode.json"
)

// openCodeAgent is the implementation of Agent for OpenCode.
type openCodeAgent struct{}

// Compile-time check to ensure openCodeAgent implements Agent interface
var _ Agent = (*openCodeAgent)(nil)

// NewOpenCode creates a new OpenCode agent implementation.
func NewOpenCode() Agent {
	return &openCodeAgent{}
}

// Name returns the agent name.
func (o *openCodeAgent) Name() string {
	return "opencode"
}

// SkipOnboarding returns the settings unchanged since OpenCode does not
// require onboarding configuration.
func (o *openCodeAgent) SkipOnboarding(settings map[string][]byte, _ string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}
	return settings, nil
}

// SetModel configures the model ID in OpenCode settings.
// The modelID supports three formats:
//   - "model" — sets the model directly
//   - "provider::model" — sets provider/model and auto-configures the provider with its default base URL
//   - "provider::model::baseURL" — sets provider/model and configures the provider with the given base URL
//
// All other fields in .config/opencode/opencode.json are preserved.
func (o *openCodeAgent) SetModel(settings map[string][]byte, modelID string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var existingContent []byte
	var exists bool
	if existingContent, exists = settings[OpenCodeConfigPath]; !exists {
		existingContent = []byte("{}")
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse existing %s: %w", OpenCodeConfigPath, err)
	}

	if config == nil {
		config = make(map[string]interface{})
	}

	// Parse provider::model[::baseURL] format
	provider, modelName, baseURL := kdnconfig.ParseModelID(modelID)
	if provider != "" {
		if modelName == "" {
			return nil, fmt.Errorf("invalid model ID %q: expected provider::model or provider::model::baseURL", modelID)
		}
		resolvedURL := baseURL
		if resolvedURL == "" {
			defaultURL, known := defaultProviderBaseURLs[provider]
			if !known {
				return nil, fmt.Errorf("unknown provider %q: append ::baseURL to specify the endpoint", provider)
			}
			resolvedURL = defaultURL
		} else {
			resolvedURL = containerurl.RewriteURL(resolvedURL)
		}
		config["model"] = provider + "/" + modelName
		if err := configureProvider(config, provider, modelName, resolvedURL); err != nil {
			return nil, err
		}
	} else {
		config["model"] = modelID
	}

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", OpenCodeConfigPath, err)
	}

	settings[OpenCodeConfigPath] = modifiedContent
	return settings, nil
}

// SkillsDir returns the container path under which skill directories are mounted for OpenCode.
func (o *openCodeAgent) SkillsDir() string {
	return "$HOME/.opencode/skills"
}

// SetMCPServers returns the settings unchanged, as OpenCode does not support MCP configuration
// through agent settings files.
func (o *openCodeAgent) SetMCPServers(settings map[string][]byte, _ *workspace.McpConfiguration) (map[string][]byte, error) {
	return settings, nil
}

// configureProvider adds a provider block with the given base URL and registers the model.
func configureProvider(config map[string]interface{}, provider, modelName, baseURL string) error {
	providers, _ := config["provider"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}
	config["provider"] = providers

	providerEntry, _ := providers[provider].(map[string]interface{})
	if providerEntry == nil {
		providerEntry = map[string]interface{}{
			"name": provider,
			"npm":  "@ai-sdk/openai-compatible",
		}
	}
	options, _ := providerEntry["options"].(map[string]interface{})
	if options == nil {
		options = make(map[string]interface{})
	}
	options["baseURL"] = baseURL
	providerEntry["options"] = options
	providers[provider] = providerEntry

	models, _ := providerEntry["models"].(map[string]interface{})
	if models == nil {
		models = make(map[string]interface{})
	}
	if _, exists := models[modelName]; !exists {
		models[modelName] = map[string]interface{}{
			"_launch": true,
			"name":    modelName,
		}
	}
	providerEntry["models"] = models

	return nil
}

// defaultProviderBaseURLs maps known provider names to their default base URLs.
var defaultProviderBaseURLs = map[string]string{
	"ollama":   "http://" + containerurl.ContainerHost + ":11434/v1",
	"ramalama": "http://" + containerurl.ContainerHost + ":8080/v1",
}
