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
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/openkaiden/kdn/pkg/config"
)

// languageFeatureMap maps programming language names (as returned by alizer) to
// the OCI reference of the corresponding devcontainer feature. Only features
// compatible with Fedora (rpm-based distributions) are included.
//
// Ruby is intentionally omitted: the ghcr.io/devcontainers/features/ruby feature
// uses apt-get only and does not support dnf/yum.
var languageFeatureMap = map[string]string{
	"Go":         "ghcr.io/devcontainers/features/go:1",
	"Python":     "ghcr.io/devcontainers/features/python:1",
	"JavaScript": "ghcr.io/devcontainers/features/node:2",
	"TypeScript": "ghcr.io/devcontainers/features/node:2",
	"Java":       "ghcr.io/devcontainers/features/java:1",
}

// AlizerAutoconfOptions configures an AlizerAutoconf runner.
type AlizerAutoconfOptions struct {
	Detector AlizerDetector

	// WorkspaceUpdater writes to .kaiden/workspace.json. When nil the runner is a
	// no-op (features and ports cannot be persisted without a local config file).
	WorkspaceUpdater config.WorkspaceConfigUpdater

	// WorkspaceConfig reads .kaiden/workspace.json to check what is already configured.
	// May be nil (skips the "already configured" check).
	WorkspaceConfig config.Config

	Yes bool

	// Confirm is called to ask whether to apply a detected feature or set of ports.
	// Returning false skips the item.
	Confirm func(prompt string) (bool, error)
}

// AlizerAutoconf detects programming languages and ports in the workspace source
// directory and offers to add the corresponding devcontainer features and port
// forwarding configuration to the local workspace config.
type AlizerAutoconf interface {
	Run(out io.Writer) error
}

type alizerAutoconfRunner struct {
	detector         AlizerDetector
	workspaceUpdater config.WorkspaceConfigUpdater
	workspaceConfig  config.Config
	yes              bool
	confirm          func(string) (bool, error)
}

var _ AlizerAutoconf = (*alizerAutoconfRunner)(nil)

// NewAlizerAutoconf returns an AlizerAutoconf configured by opts.
func NewAlizerAutoconf(opts AlizerAutoconfOptions) AlizerAutoconf {
	return &alizerAutoconfRunner{
		detector:         opts.Detector,
		workspaceUpdater: opts.WorkspaceUpdater,
		workspaceConfig:  opts.WorkspaceConfig,
		yes:              opts.Yes,
		confirm:          opts.Confirm,
	}
}

func (r *alizerAutoconfRunner) Run(out io.Writer) error {
	if r.workspaceUpdater == nil {
		return nil
	}

	result, err := r.detector.Detect()
	if err != nil {
		return fmt.Errorf("alizer detection failed: %w", err)
	}

	if len(result.Languages) == 0 && len(result.Ports) == 0 {
		return nil
	}

	existingFeatures, existingPorts := r.loadExistingConfig()

	if err := r.processFeatures(out, result.Languages, existingFeatures); err != nil {
		return err
	}
	return r.processPorts(out, result.Ports, existingPorts)
}

// loadExistingConfig reads the current workspace config and returns the set of
// already-configured feature IDs and port numbers. Errors are silently ignored
// so that a missing or malformed config file is treated as "nothing configured".
func (r *alizerAutoconfRunner) loadExistingConfig() (map[string]bool, map[int]bool) {
	featureSet := make(map[string]bool)
	portSet := make(map[int]bool)

	if r.workspaceConfig == nil {
		return featureSet, portSet
	}

	cfg, err := r.workspaceConfig.Load()
	if err != nil {
		return featureSet, portSet
	}

	if cfg.Features != nil {
		for id := range *cfg.Features {
			featureSet[id] = true
		}
	}
	if cfg.Ports != nil {
		for _, p := range *cfg.Ports {
			portSet[p] = true
		}
	}
	return featureSet, portSet
}

func (r *alizerAutoconfRunner) processFeatures(out io.Writer, languages []string, existingFeatures map[string]bool) error {
	// Deduplicate: multiple languages may map to the same feature (e.g. JS + TS → node).
	// Collect all language names per feature so they can be shown together in output.
	type featureEntry struct {
		featureID   string
		langDisplay string
	}
	seenIdx := make(map[string]int) // featureID → index in entries
	var entries []featureEntry
	for _, lang := range languages {
		featureID, ok := languageFeatureMap[lang]
		if !ok {
			continue
		}
		if idx, exists := seenIdx[featureID]; exists {
			entries[idx].langDisplay += ", " + lang
		} else {
			seenIdx[featureID] = len(entries)
			entries = append(entries, featureEntry{featureID: featureID, langDisplay: lang})
		}
	}

	for _, e := range entries {
		if err := r.processFeature(out, e.featureID, e.langDisplay, existingFeatures); err != nil {
			return err
		}
	}
	return nil
}

func (r *alizerAutoconfRunner) processFeature(out io.Writer, featureID, langDisplay string, existingFeatures map[string]bool) error {
	if existingFeatures[featureID] {
		fmt.Fprintf(out, "%s %s already configured.\n", greenCheck, langDisplay)
		return nil
	}

	fmt.Fprintf(out, "Detected language: %s\n", langDisplay)

	if !r.yes {
		ok, err := r.confirm(fmt.Sprintf("Add feature %q to local workspace config?", featureID))
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !ok {
			fmt.Fprintf(out, "%s Skipped %s.\n", greyDash, langDisplay)
			return nil
		}
	}

	if err := r.workspaceUpdater.AddFeature(featureID, map[string]interface{}{}); err != nil {
		return fmt.Errorf("failed to add %s support to workspace config: %w", langDisplay, err)
	}
	fmt.Fprintf(out, "%s Added %s support to local workspace config.\n", greenCheck, langDisplay)
	return nil
}

func (r *alizerAutoconfRunner) processPorts(out io.Writer, ports []int, existingPorts map[int]bool) error {
	var newPorts []int
	for _, port := range ports {
		if !existingPorts[port] {
			newPorts = append(newPorts, port)
		}
	}

	// Report already-configured ports.
	for _, port := range ports {
		if existingPorts[port] {
			fmt.Fprintf(out, "%s Port %d already configured.\n", greenCheck, port)
		}
	}

	if len(newPorts) == 0 {
		return nil
	}

	sort.Ints(newPorts)
	fmt.Fprintf(out, "Detected ports: %s\n", formatPorts(newPorts))

	if !r.yes {
		ok, err := r.confirm(fmt.Sprintf("Add ports %s to local workspace config?", formatPorts(newPorts)))
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !ok {
			fmt.Fprintf(out, "%s Skipped ports %s.\n", greyDash, formatPorts(newPorts))
			return nil
		}
	}

	for _, port := range newPorts {
		if err := r.workspaceUpdater.AddPort(port); err != nil {
			return fmt.Errorf("failed to add port %d to workspace config: %w", port, err)
		}
	}
	fmt.Fprintf(out, "%s Added ports %s to local workspace config.\n", greenCheck, formatPorts(newPorts))
	return nil
}

func formatPorts(ports []int) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ", ")
}
