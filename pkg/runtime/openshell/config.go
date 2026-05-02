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
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configFile       = "config.json"
	gatewayStateFile = "gateway-state.json"

	// DriverPodman uses Podman as the sandbox container driver.
	DriverPodman = "podman"

	// DriverVM uses a VM as the sandbox driver.
	DriverVM = "vm"

	// defaultDriver is the driver used when none is specified.
	defaultDriver = DriverPodman
)

type gatewayConfig struct {
	Driver string `json:"driver"`
}

// loadConfig reads the gateway configuration from storage.
// Returns default config if the file does not exist.
func loadConfig(storageDir string) gatewayConfig {
	data, err := os.ReadFile(filepath.Join(storageDir, configFile))
	if err != nil {
		return gatewayConfig{Driver: defaultDriver}
	}

	var cfg gatewayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return gatewayConfig{Driver: defaultDriver}
	}

	if cfg.Driver == "" {
		cfg.Driver = defaultDriver
	}
	return cfg
}

// saveConfig writes the gateway configuration to storage.
func saveConfig(storageDir string, cfg gatewayConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(storageDir, configFile), data, 0644)
}

type gatewayState struct {
	PID    int    `json:"pid"`
	Driver string `json:"driver"`
}

func loadGatewayState(storageDir string) (gatewayState, error) {
	data, err := os.ReadFile(filepath.Join(storageDir, gatewayStateFile))
	if err != nil {
		return gatewayState{}, err
	}

	var state gatewayState
	if err := json.Unmarshal(data, &state); err != nil {
		return gatewayState{}, err
	}
	return state, nil
}

func saveGatewayState(storageDir string, state gatewayState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(storageDir, gatewayStateFile), data, 0644)
}

func removeGatewayState(storageDir string) {
	os.Remove(filepath.Join(storageDir, gatewayStateFile))
	os.Remove(filepath.Join(storageDir, gatewayPIDFile))
}
