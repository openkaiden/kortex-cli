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

// Package secretservicesetup provides centralized registration of all available secret service implementations.
package secretservicesetup

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/openkaiden/kdn/pkg/secretservice"
)

// SecretServiceRegistrar is an interface for types that can register secret services.
// This is implemented by instances.Manager.
type SecretServiceRegistrar interface {
	RegisterSecretService(service secretservice.SecretService) error
}

// secretServiceFactory is a function that creates a new secret service instance.
type secretServiceFactory func() secretservice.SecretService

//go:embed secretservices.json
var secretServicesJSON []byte

// secretServiceDefinition represents a secret service entry in the embedded JSON file.
type secretServiceDefinition struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	HostsPatterns  []string `json:"hostsPatterns"`
	Path           string   `json:"path"`
	HeaderName     string   `json:"headerName"`
	HeaderTemplate string   `json:"headerTemplate"`
	EnvVars        []string `json:"envVars"`
}

// availableSecretServices is the list of all secret services loaded from the embedded JSON file.
var availableSecretServices = mustLoadSecretServices()

// mustLoadSecretServices loads secret service definitions from the embedded JSON and returns
// factory functions for each. It panics on error since embedded data corruption is a build defect.
func mustLoadSecretServices() []secretServiceFactory {
	factories, err := loadSecretServices()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded secret services: %v", err))
	}

	return factories
}

// loadSecretServices parses the embedded JSON and returns a factory function for each definition.
func loadSecretServices() ([]secretServiceFactory, error) {
	var definitions []secretServiceDefinition
	if err := json.Unmarshal(secretServicesJSON, &definitions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret services JSON: %w", err)
	}

	factories := make([]secretServiceFactory, 0, len(definitions))
	for _, def := range definitions {
		d := def // capture loop variable
		factories = append(factories, func() secretservice.SecretService {
			return secretservice.NewSecretService(
				d.Name,
				d.HostsPatterns,
				d.Path,
				d.EnvVars,
				d.HeaderName,
				d.HeaderTemplate,
				d.Description,
			)
		})
	}

	return factories, nil
}

// RegisterAll registers all available secret service implementations to the given registrar.
// Returns an error if any secret service fails to register.
func RegisterAll(registrar SecretServiceRegistrar) error {
	return registerAllWithFactories(registrar, availableSecretServices)
}

// ListAvailable returns the names of all available secret services.
func ListAvailable() []string {
	return listAvailableWithFactories(availableSecretServices)
}

// ListServices returns fully-constructed instances of all available secret services.
func ListServices() []secretservice.SecretService {
	return listServicesWithFactories(availableSecretServices)
}

// listServicesWithFactories returns fully-constructed secret services from the given factories.
func listServicesWithFactories(factories []secretServiceFactory) []secretservice.SecretService {
	services := make([]secretservice.SecretService, 0, len(factories))
	for _, factory := range factories {
		services = append(services, factory())
	}
	return services
}

// listAvailableWithFactories returns the names of secret services from the given factories.
// This function is internal and used for testing with custom secret service lists.
func listAvailableWithFactories(factories []secretServiceFactory) []string {
	names := make([]string, 0, len(factories))
	for _, factory := range factories {
		svc := factory()
		names = append(names, svc.Name())
	}
	return names
}

// registerAllWithFactories registers the given secret services to the registrar.
// This function is internal and used for testing with custom secret service lists.
func registerAllWithFactories(registrar SecretServiceRegistrar, factories []secretServiceFactory) error {
	for _, factory := range factories {
		svc := factory()
		if svc == nil {
			return fmt.Errorf("secret service factory returned nil")
		}
		if err := registrar.RegisterSecretService(svc); err != nil {
			return fmt.Errorf("failed to register secret service %q: %w", svc.Name(), err)
		}
	}

	return nil
}
