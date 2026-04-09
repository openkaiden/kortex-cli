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
	"fmt"

	"github.com/goccy/go-yaml"
)

const (
	// GooseConfigPath is the relative path to the Goose configuration file.
	GooseConfigPath = ".config/goose/config.yaml"

	gooseTelemetryKey = "GOOSE_TELEMETRY_ENABLED"
	gooseModelKey     = "GOOSE_MODEL"
)

// gooseAgent is the implementation of Agent for Goose.
type gooseAgent struct{}

// Compile-time check to ensure gooseAgent implements Agent interface
var _ Agent = (*gooseAgent)(nil)

// NewGoose creates a new Goose agent implementation.
func NewGoose() Agent {
	return &gooseAgent{}
}

// Name returns the agent name.
func (g *gooseAgent) Name() string {
	return "goose"
}

// SkipOnboarding modifies Goose settings to disable telemetry prompts.
// It sets GOOSE_TELEMETRY_ENABLED to false in the goose config file if the
// value is not already defined. If the user has already set it in their own
// config file, the existing value is preserved.
func (g *gooseAgent) SkipOnboarding(settings map[string][]byte, _ string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var config map[string]interface{}
	if content, exists := settings[GooseConfigPath]; exists {
		if err := yaml.Unmarshal(content, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing %s: %w", GooseConfigPath, err)
		}
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// Only set telemetry if not already defined by the user
	if _, defined := config[gooseTelemetryKey]; !defined {
		config[gooseTelemetryKey] = false
	}

	modifiedContent, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", GooseConfigPath, err)
	}

	settings[GooseConfigPath] = modifiedContent
	return settings, nil
}

// SkillsDir returns the container path under which skill directories are mounted for Goose.
func (g *gooseAgent) SkillsDir() string {
	return "$HOME/.agents/skills"
}

// SetModel configures the model ID in Goose settings.
// It sets the GOOSE_MODEL key in the config file.
// All other fields in the settings file are preserved.
func (g *gooseAgent) SetModel(settings map[string][]byte, modelID string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}

	var config map[string]interface{}
	if content, exists := settings[GooseConfigPath]; exists {
		if err := yaml.Unmarshal(content, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing %s: %w", GooseConfigPath, err)
		}
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	config[gooseModelKey] = modelID

	modifiedContent, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified %s: %w", GooseConfigPath, err)
	}

	settings[GooseConfigPath] = modifiedContent
	return settings, nil
}
