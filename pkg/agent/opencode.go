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
	"net/url"
	"strings"

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
func (o *openCodeAgent) SkipOnboarding(settings map[string]SettingsFile, _ string, _ []string) (map[string]SettingsFile, error) {
	return EnsureSettings(settings), nil
}

// SetModel configures the model ID in OpenCode settings.
// The modelID supports three formats:
//   - "model" — sets the model directly
//   - "provider::model" — sets provider/model; for native providers only sets the model field;
//     for non-native providers adds a provider block (with default baseURL for ollama/ramalama)
//   - "provider::model::baseURL" — sets provider/model and configures the provider with the given base URL
//
// All other fields in .config/opencode/opencode.json are preserved.
func (o *openCodeAgent) SetModel(settings map[string]SettingsFile, modelID string, containerHost string) (map[string]SettingsFile, error) {
	settings = EnsureSettings(settings)
	existingContent := GetContent(settings, OpenCodeConfigPath, []byte("{}"))

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
		if alias, ok := providerAliases[provider]; ok {
			provider = alias
		}
		resolvedURL := baseURL
		if resolvedURL == "" {
			resolvedURL = defaultProviderBaseURL(provider, containerHost)
		} else {
			resolvedURL = containerurl.RewriteURLWithHost(resolvedURL, containerHost)
		}
		realOpenAI := isRealOpenAI(provider, resolvedURL)
		if provider == "openai" && !realOpenAI {
			provider = "custom"
		}
		config["model"] = provider + "/" + modelName
		if (!nativeProviders[provider] && !realOpenAI) || (resolvedURL != "" && !realOpenAI) {
			providerName := provider
			if provider == "custom" && resolvedURL != "" {
				if parsed, err := url.Parse(resolvedURL); err == nil && parsed.Hostname() != "" {
					providerName = parsed.Hostname()
				}
			}
			if err := configureProvider(config, provider, providerName, modelName, resolvedURL); err != nil {
				return nil, err
			}
		}
	} else {
		config["model"] = modelID
	}

	modifiedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", OpenCodeConfigPath, err)
	}

	settings = SetContent(settings, OpenCodeConfigPath, modifiedContent)
	return settings, nil
}

// SkillsDir returns the container path under which skill directories are mounted for OpenCode.
func (o *openCodeAgent) SkillsDir() string {
	return "$HOME/.opencode/skills"
}

// SetMCPServers returns the settings unchanged, as OpenCode does not support MCP configuration
// through agent settings files.
func (o *openCodeAgent) SetMCPServers(settings map[string]SettingsFile, _ *workspace.McpConfiguration) (map[string]SettingsFile, error) {
	return settings, nil
}

// nativeProviders lists providers that OpenCode supports natively via bundled SDKs.
// These do not need the "npm": "@ai-sdk/openai-compatible" field.
var nativeProviders = map[string]bool{
	"anthropic": true,
	"mistral":   true,
	"google":    true,
}

// vertexAIProviders lists providers that use Vertex AI native SDK in OpenCode.
// These always need a provider block with empty model entries but no npm or name fields.
var vertexAIProviders = map[string]bool{
	"google-vertex-anthropic": true,
}

// configureProvider adds a provider block with the given base URL and registers the model.
// providerName is used as the display name in the provider entry.
func configureProvider(config map[string]interface{}, provider, providerName, modelName, baseURL string) error {
	providers, _ := config["provider"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}
	config["provider"] = providers

	providerEntry, _ := providers[provider].(map[string]interface{})
	if providerEntry == nil {
		providerEntry = make(map[string]interface{})
	}
	if !nativeProviders[provider] && !vertexAIProviders[provider] {
		providerEntry["name"] = providerName
		providerEntry["npm"] = "@ai-sdk/openai-compatible"
	}
	if baseURL != "" {
		options, _ := providerEntry["options"].(map[string]interface{})
		if options == nil {
			options = make(map[string]interface{})
		}
		options["baseURL"] = baseURL
		providerEntry["options"] = options
	}
	providers[provider] = providerEntry

	models, _ := providerEntry["models"].(map[string]interface{})
	if models == nil {
		models = make(map[string]interface{})
	}
	if _, exists := models[modelName]; !exists {
		if vertexAIProviders[provider] {
			models[modelName] = map[string]interface{}{}
		} else {
			models[modelName] = map[string]interface{}{
				"_launch": true,
				"name":    modelName,
			}
		}
	}
	providerEntry["models"] = models

	return nil
}

// isRealOpenAI returns true when the provider is "openai" pointing at the
// real OpenAI API (api.openai.com) rather than an openai-compatible endpoint.
func isRealOpenAI(provider, baseURL string) bool {
	if provider != "openai" {
		return false
	}
	if baseURL == "" {
		return true
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.openai.com")
}

// defaultProviderBaseURL returns the default base URL for a known provider,
// using the given containerHost to reach the host machine from inside the
// runtime environment. Returns "" for unknown providers.
func defaultProviderBaseURL(provider, containerHost string) string {
	switch provider {
	case "ollama":
		return "http://" + containerHost + ":11434/v1"
	case "ramalama":
		return "http://" + containerHost + ":8080/v1"
	default:
		return ""
	}
}

// providerAliases maps provider names used in kdn model IDs to the provider
// names expected by OpenCode. For example, "gemini" maps to "google" because
// OpenCode uses the "@ai-sdk/google" package under the "google" key.
var providerAliases = map[string]string{
	"gemini":   "google",
	"vertexai": "google-vertex-anthropic",
}
