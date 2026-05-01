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
	"errors"
	"fmt"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/secret"
)

// ConfiguredSecret is a detected secret that is already fully set up — present
// in the store and referenced in at least one configuration source.
type ConfiguredSecret struct {
	DetectedSecret
	// Locations lists every config source where the secret is referenced.
	Locations []ConfigTarget
}

// FilterResult is the output of SecretFilter.Filter.
type FilterResult struct {
	// NeedsAction contains secrets that are missing from the store or from every
	// known config source and therefore require user action.
	NeedsAction []DetectedSecret
	// Configured contains secrets that are already fully set up. Each entry
	// includes the locations where the secret is referenced.
	Configured []ConfiguredSecret
}

// SecretFilter classifies DetectedSecrets into those needing action and those
// already fully configured.
type SecretFilter interface {
	Filter(detected []DetectedSecret) (FilterResult, error)
}

// alreadyConfiguredFilter classifies secrets by checking the store and every
// configuration source (global, project-specific, local workspace).
type alreadyConfiguredFilter struct {
	store           secret.Store
	loader          config.ProjectConfigLoader
	projectID       string        // used to load project-specific config
	workspaceConfig config.Config // nil = no local workspace config to check
}

var _ SecretFilter = (*alreadyConfiguredFilter)(nil)

// NewAlreadyConfiguredFilter returns a SecretFilter that classifies secrets
// across all configuration sources.
// loader.Load("") covers the global config; loader.Load(projectID) covers
// global + project-specific merged, from which project-specific secrets are
// derived. workspaceConfig is optional and covers .kaiden/workspace.json.
func NewAlreadyConfiguredFilter(
	store secret.Store,
	loader config.ProjectConfigLoader,
	projectID string,
	workspaceConfig config.Config,
) SecretFilter {
	return &alreadyConfiguredFilter{
		store:           store,
		loader:          loader,
		projectID:       projectID,
		workspaceConfig: workspaceConfig,
	}
}

// Filter classifies each detected secret. A secret is moved to Configured when
// it is present in the store AND referenced in at least one config source.
func (f *alreadyConfiguredFilter) Filter(detected []DetectedSecret) (FilterResult, error) {
	globalSecrets, err := f.loadGlobalSecrets()
	if err != nil {
		return FilterResult{}, err
	}

	projectSecrets, err := f.loadProjectSecrets(globalSecrets)
	if err != nil {
		return FilterResult{}, err
	}

	localSecrets, err := f.loadLocalSecrets()
	if err != nil {
		return FilterResult{}, err
	}

	var result FilterResult
	for _, d := range detected {
		_, _, storeErr := f.store.Get(d.ServiceName)
		if storeErr != nil && !errors.Is(storeErr, secret.ErrSecretNotFound) {
			return FilterResult{}, fmt.Errorf("failed to check secret %q in store: %w", d.ServiceName, storeErr)
		}
		inStore := storeErr == nil

		locations := f.locations(d.ServiceName, globalSecrets, projectSecrets, localSecrets)

		if inStore && len(locations) > 0 {
			result.Configured = append(result.Configured, ConfiguredSecret{
				DetectedSecret: d,
				Locations:      locations,
			})
		} else {
			result.NeedsAction = append(result.NeedsAction, d)
		}
	}
	return result, nil
}

func (f *alreadyConfiguredFilter) loadGlobalSecrets() (map[string]struct{}, error) {
	cfg, err := f.loader.Load("")
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	return secretSet(cfg), nil
}

// loadProjectSecrets returns secrets that appear in the project-specific config
// but NOT in the global config (i.e. exclusively project-level). When
// f.projectID is empty there is no project-specific scope to check.
func (f *alreadyConfiguredFilter) loadProjectSecrets(globalSecrets map[string]struct{}) (map[string]struct{}, error) {
	if f.projectID == "" {
		return nil, nil
	}
	merged, err := f.loader.Load(f.projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}
	mergedSet := secretSet(merged)
	projectOnly := make(map[string]struct{})
	for s := range mergedSet {
		if _, inGlobal := globalSecrets[s]; !inGlobal {
			projectOnly[s] = struct{}{}
		}
	}
	return projectOnly, nil
}

func (f *alreadyConfiguredFilter) loadLocalSecrets() (map[string]struct{}, error) {
	if f.workspaceConfig == nil {
		return nil, nil
	}
	cfg, err := f.workspaceConfig.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load workspace config: %w", err)
	}
	return secretSet(cfg), nil
}

func (f *alreadyConfiguredFilter) locations(
	name string,
	global, project, local map[string]struct{},
) []ConfigTarget {
	var locs []ConfigTarget
	if _, ok := global[name]; ok {
		locs = append(locs, ConfigTargetGlobal)
	}
	if _, ok := project[name]; ok {
		locs = append(locs, ConfigTargetProject)
	}
	if _, ok := local[name]; ok {
		locs = append(locs, ConfigTargetLocal)
	}
	return locs
}

func secretSet(cfg *workspace.WorkspaceConfiguration) map[string]struct{} {
	if cfg == nil || cfg.Secrets == nil {
		return nil
	}
	m := make(map[string]struct{}, len(*cfg.Secrets))
	for _, s := range *cfg.Secrets {
		m[s] = struct{}{}
	}
	return m
}
