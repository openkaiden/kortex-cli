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

package autoconf

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
)

// fakeAlizerDetector returns a fixed AlizerResult.
type fakeAlizerDetector struct {
	result AlizerResult
	err    error
}

func (f *fakeAlizerDetector) Detect() (AlizerResult, error) {
	return f.result, f.err
}

// fakeAlizerWorkspaceConfig returns a fixed *workspace.WorkspaceConfiguration.
type fakeAlizerWorkspaceConfig struct {
	cfg *workspace.WorkspaceConfiguration
}

func (f *fakeAlizerWorkspaceConfig) Load() (*workspace.WorkspaceConfiguration, error) {
	if f.cfg != nil {
		return f.cfg, nil
	}
	return nil, config.ErrConfigNotFound
}

func TestAlizerAutoconf_NilUpdater_ReturnsNil(t *testing.T) {
	t.Parallel()

	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}}},
		WorkspaceUpdater: nil,
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("expected nil when updater is nil, got: %v", err)
	}
}

func TestAlizerAutoconf_DetectorError_Propagates(t *testing.T) {
	t.Parallel()

	want := errors.New("detect failed")
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{err: want},
		WorkspaceUpdater: &fakeWorkspaceUpdater{},
	})

	err := runner.Run(&bytes.Buffer{})
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}

func TestAlizerAutoconf_NoResults_ReturnsNil(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{},
		WorkspaceUpdater: wu,
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 0 || len(wu.ports) != 0 {
		t.Error("expected no updater calls when nothing detected")
	}
}

func TestAlizerAutoconf_AddsFeature_Yes(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}}},
		WorkspaceUpdater: wu,
		Yes:              true,
		Confirm:          func(string) (bool, error) { t.Fatal("confirm must not be called"); return false, nil },
	})

	var out bytes.Buffer
	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 1 || wu.features[0].featureID != "ghcr.io/devcontainers/features/go:1" {
		t.Errorf("expected Go feature added, got %v", wu.features)
	}
	if !strings.Contains(out.String(), "Go") {
		t.Errorf("expected language name in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_AddsFeature_Confirmed(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Python"}}},
		WorkspaceUpdater: wu,
		Yes:              false,
		Confirm:          func(string) (bool, error) { return true, nil },
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 1 || wu.features[0].featureID != "ghcr.io/devcontainers/features/python:1" {
		t.Errorf("expected Python feature added, got %v", wu.features)
	}
}

func TestAlizerAutoconf_SkipsFeature_Declined(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	var out bytes.Buffer
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}}},
		WorkspaceUpdater: wu,
		Yes:              false,
		Confirm:          func(string) (bool, error) { return false, nil },
	})

	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 0 {
		t.Errorf("expected no feature added when declined, got %v", wu.features)
	}
	if !strings.Contains(out.String(), "Skipped Go") {
		t.Errorf("expected 'Skipped Go' in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_SkipsUnknownLanguage(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Ruby", "COBOL"}}},
		WorkspaceUpdater: wu,
		Yes:              true,
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 0 {
		t.Errorf("expected no features for unmapped languages, got %v", wu.features)
	}
}

func TestAlizerAutoconf_DeduplicatesFeatures_JSAndTS(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector: &fakeAlizerDetector{result: AlizerResult{
			Languages: []string{"JavaScript", "TypeScript"},
		}},
		WorkspaceUpdater: wu,
		Yes:              true,
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 1 {
		t.Errorf("expected 1 feature (node deduplicated), got %d: %v", len(wu.features), wu.features)
	}
	if wu.features[0].featureID != "ghcr.io/devcontainers/features/node:2" {
		t.Errorf("expected node feature, got %q", wu.features[0].featureID)
	}
}

func TestAlizerAutoconf_FeatureAlreadyConfigured(t *testing.T) {
	t.Parallel()

	existingFeatureID := "ghcr.io/devcontainers/features/go:1"
	features := map[string]map[string]interface{}{existingFeatureID: {}}
	wu := &fakeWorkspaceUpdater{}
	var out bytes.Buffer
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}}},
		WorkspaceUpdater: wu,
		WorkspaceConfig: &fakeAlizerWorkspaceConfig{cfg: &workspace.WorkspaceConfiguration{
			Features: &features,
		}},
		Yes: true,
	})

	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 0 {
		t.Errorf("expected no AddFeature call when already configured, got %v", wu.features)
	}
	if !strings.Contains(out.String(), "already configured") {
		t.Errorf("expected 'already configured' in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_AddsPorts_Yes(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Ports: []int{8080, 3000}}},
		WorkspaceUpdater: wu,
		Yes:              true,
	})

	var out bytes.Buffer
	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.ports) != 2 {
		t.Errorf("expected 2 ports added, got %d: %v", len(wu.ports), wu.ports)
	}
	if !strings.Contains(out.String(), "Added ports") {
		t.Errorf("expected 'Added ports' in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_AddsPorts_Confirmed(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Ports: []int{5000}}},
		WorkspaceUpdater: wu,
		Yes:              false,
		Confirm:          func(string) (bool, error) { return true, nil },
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.ports) != 1 || wu.ports[0] != 5000 {
		t.Errorf("expected port 5000 added, got %v", wu.ports)
	}
}

func TestAlizerAutoconf_SkipsPorts_Declined(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	var out bytes.Buffer
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Ports: []int{8080}}},
		WorkspaceUpdater: wu,
		Yes:              false,
		Confirm:          func(string) (bool, error) { return false, nil },
	})

	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.ports) != 0 {
		t.Errorf("expected no ports added when declined, got %v", wu.ports)
	}
	if !strings.Contains(out.String(), "Skipped ports") {
		t.Errorf("expected 'Skipped ports' in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_PortAlreadyConfigured(t *testing.T) {
	t.Parallel()

	ports := []int{8080}
	wu := &fakeWorkspaceUpdater{}
	var out bytes.Buffer
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Ports: []int{8080}}},
		WorkspaceUpdater: wu,
		WorkspaceConfig: &fakeAlizerWorkspaceConfig{cfg: &workspace.WorkspaceConfiguration{
			Ports: &ports,
		}},
		Yes: true,
	})

	if err := runner.Run(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.ports) != 0 {
		t.Errorf("expected no AddPort call when already configured, got %v", wu.ports)
	}
	if !strings.Contains(out.String(), "already configured") {
		t.Errorf("expected 'already configured' in output, got: %q", out.String())
	}
}

func TestAlizerAutoconf_ConfirmError_Propagates(t *testing.T) {
	t.Parallel()

	want := errors.New("confirm interrupted")
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}}},
		WorkspaceUpdater: &fakeWorkspaceUpdater{},
		Yes:              false,
		Confirm:          func(string) (bool, error) { return false, want },
	})

	err := runner.Run(&bytes.Buffer{})
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}

func TestAlizerAutoconf_NilWorkspaceConfig_TreatedAsEmpty(t *testing.T) {
	t.Parallel()

	wu := &fakeWorkspaceUpdater{}
	runner := NewAlizerAutoconf(AlizerAutoconfOptions{
		Detector:         &fakeAlizerDetector{result: AlizerResult{Languages: []string{"Go"}, Ports: []int{8080}}},
		WorkspaceUpdater: wu,
		WorkspaceConfig:  nil,
		Yes:              true,
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wu.features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(wu.features))
	}
	if len(wu.ports) != 1 {
		t.Errorf("expected 1 port, got %d", len(wu.ports))
	}
}
