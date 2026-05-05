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

// Package autoconf provides environment detection for automatic workspace configuration.
package autoconf

import (
	"os"

	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

// DetectedSecret represents a secret found in the environment for a specific service.
type DetectedSecret struct {
	ServiceName string
	EnvVarName  string
	Value       string
}

// SecretDetector scans the environment for secrets belonging to registered services.
// Detect takes no parameters — services are provided at construction time so that
// this interface stays consistent with future detectors (language, config-dir, etc.)
// that also own their own data sources. When the detector was created with
// NewFilteredSecretDetector, fully-configured secrets are returned in
// FilterResult.Configured so callers can display their status.
type SecretDetector interface {
	Detect() (FilterResult, error)
}

// envSecretDetector is the default implementation, reading from os.LookupEnv.
type envSecretDetector struct {
	lookupEnv func(string) (string, bool)
	services  []secretservice.SecretService
	filter    SecretFilter // nil = no filtering
}

var _ SecretDetector = (*envSecretDetector)(nil)

// NewSecretDetector returns a SecretDetector that reads from the process environment
// without any filtering of already-configured secrets.
func NewSecretDetector(services []secretservice.SecretService) SecretDetector {
	return &envSecretDetector{lookupEnv: os.LookupEnv, services: services}
}

// NewFilteredSecretDetector returns a SecretDetector that reads from the process
// environment and removes secrets that are already stored and referenced in any
// configuration source — callers receive only secrets that require action.
// projectID is the computed project identifier for the current directory;
// workspaceConfig is optional and covers the local .kaiden/workspace.json.
func NewFilteredSecretDetector(
	services []secretservice.SecretService,
	store secret.Store,
	loader config.ProjectConfigLoader,
	projectID string,
	workspaceConfig config.Config,
) SecretDetector {
	return &envSecretDetector{
		lookupEnv: os.LookupEnv,
		services:  services,
		filter:    NewAlreadyConfiguredFilter(store, loader, projectID, workspaceConfig),
	}
}

// newSecretDetectorWithLookup creates a SecretDetector with an injectable lookup
// function and no filter. Used in tests.
func newSecretDetectorWithLookup(lookupEnv func(string) (string, bool), services []secretservice.SecretService) SecretDetector {
	return &envSecretDetector{lookupEnv: lookupEnv, services: services}
}

// Detect iterates over services and returns one DetectedSecret per service whose
// env vars are set. For each service the first non-empty env var found is used.
// When the detector was created with NewFilteredSecretDetector, already-configured
// secrets are returned in FilterResult.Configured; secrets that require action are
// in FilterResult.NeedsAction.
func (d *envSecretDetector) Detect() (FilterResult, error) {
	var detected []DetectedSecret
	for _, svc := range d.services {
		for _, envVar := range svc.EnvVars() {
			value, ok := d.lookupEnv(envVar)
			if ok && value != "" {
				detected = append(detected, DetectedSecret{
					ServiceName: svc.Name(),
					EnvVarName:  envVar,
					Value:       value,
				})
				break
			}
		}
	}
	if d.filter != nil {
		return d.filter.Filter(detected)
	}
	return FilterResult{NeedsAction: detected}, nil
}
