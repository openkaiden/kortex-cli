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
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_DefaultsWhenMissing(t *testing.T) {
	t.Parallel()

	cfg := loadConfig(t.TempDir())

	if cfg.Driver != DriverPodman {
		t.Errorf("Expected default driver %q, got %q", DriverPodman, cfg.Driver)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := gatewayConfig{Driver: DriverVM}
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatalf("saveConfig() failed: %v", err)
	}

	loaded := loadConfig(dir)
	if loaded.Driver != DriverVM {
		t.Errorf("Expected driver %q, got %q", DriverVM, loaded.Driver)
	}
}

func TestLoadConfig_DefaultsWhenEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Save config with empty driver
	cfg := gatewayConfig{Driver: ""}
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatalf("saveConfig() failed: %v", err)
	}

	loaded := loadConfig(dir)
	if loaded.Driver != DriverPodman {
		t.Errorf("Expected default driver %q for empty config, got %q", DriverPodman, loaded.Driver)
	}
}

func TestLoadConfig_DefaultsWhenCorruptJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte("not json{"), 0644); err != nil {
		t.Fatalf("Failed to write corrupt config: %v", err)
	}

	cfg := loadConfig(dir)
	if cfg.Driver != DriverPodman {
		t.Errorf("Expected default driver for corrupt JSON, got %q", cfg.Driver)
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, driver := range []string{DriverPodman, DriverVM} {
		cfg := gatewayConfig{Driver: driver}
		if err := saveConfig(dir, cfg); err != nil {
			t.Fatalf("saveConfig(%q) failed: %v", driver, err)
		}
		loaded := loadConfig(dir)
		if loaded.Driver != driver {
			t.Errorf("Round trip: expected %q, got %q", driver, loaded.Driver)
		}
	}
}

func TestSaveConfig_ErrorOnUnwritableDir(t *testing.T) {
	t.Parallel()

	err := saveConfig("/nonexistent/path/that/does/not/exist", gatewayConfig{Driver: DriverPodman})
	if err == nil {
		t.Error("Expected error when directory does not exist")
	}
}
