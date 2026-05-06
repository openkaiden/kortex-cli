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

package secretservicesetup

import (
	"errors"
	"testing"

	"github.com/openkaiden/kdn/pkg/secretservice"
)

// fakeSecretService is a test implementation of the SecretService interface
type fakeSecretService struct {
	name           string
	description    string
	hostsPatterns  []string
	path           string
	envVars        []string
	headerName     string
	headerTemplate string
}

func (f *fakeSecretService) Name() string            { return f.name }
func (f *fakeSecretService) Description() string     { return f.description }
func (f *fakeSecretService) HostsPatterns() []string { return f.hostsPatterns }
func (f *fakeSecretService) Path() string            { return f.path }
func (f *fakeSecretService) EnvVars() []string       { return f.envVars }
func (f *fakeSecretService) HeaderName() string      { return f.headerName }
func (f *fakeSecretService) HeaderTemplate() string  { return f.headerTemplate }

// fakeRegistrar implements SecretServiceRegistrar for testing
type fakeRegistrar struct {
	registered map[string]secretservice.SecretService
	failOn     string // service name to fail registration on
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{
		registered: make(map[string]secretservice.SecretService),
	}
}

func (f *fakeRegistrar) RegisterSecretService(service secretservice.SecretService) error {
	if service.Name() == f.failOn {
		return errors.New("registration failed")
	}
	f.registered[service.Name()] = service
	return nil
}

func TestRegisterAll(t *testing.T) {
	t.Parallel()

	t.Run("registers all secret services successfully", func(t *testing.T) {
		t.Parallel()

		registrar := newFakeRegistrar()

		err := RegisterAll(registrar)
		if err != nil {
			t.Errorf("RegisterAll() error = %v, want nil", err)
		}

		if len(registrar.registered) != 3 {
			t.Errorf("registered %d secret services, want 3", len(registrar.registered))
		}

		if _, exists := registrar.registered["github"]; !exists {
			t.Error("github secret service was not registered")
		}

		if _, exists := registrar.registered["gemini"]; !exists {
			t.Error("gemini secret service was not registered")
		}

		if _, exists := registrar.registered["anthropic"]; !exists {
			t.Error("anthropic secret service was not registered")
		}
	})
}

func TestRegisterAllWithFactories(t *testing.T) {
	t.Parallel()

	t.Run("registers secret services from custom factories", func(t *testing.T) {
		t.Parallel()

		registrar := newFakeRegistrar()

		factories := []secretServiceFactory{
			func() secretservice.SecretService {
				return &fakeSecretService{name: "github", hostsPatterns: []string{"github.com"}, headerName: "Authorization"}
			},
		}

		err := registerAllWithFactories(registrar, factories)
		if err != nil {
			t.Errorf("registerAllWithFactories() error = %v, want nil", err)
		}

		if len(registrar.registered) != 1 {
			t.Errorf("registered %d secret services, want 1", len(registrar.registered))
		}

		if _, exists := registrar.registered["github"]; !exists {
			t.Error("github secret service was not registered")
		}
	})

	t.Run("handles empty factory list", func(t *testing.T) {
		t.Parallel()

		registrar := newFakeRegistrar()
		factories := []secretServiceFactory{}

		err := registerAllWithFactories(registrar, factories)
		if err != nil {
			t.Errorf("registerAllWithFactories() with empty list error = %v, want nil", err)
		}

		if len(registrar.registered) != 0 {
			t.Errorf("registered %d secret services, want 0", len(registrar.registered))
		}
	})

	t.Run("stops on first registration error", func(t *testing.T) {
		t.Parallel()

		registrar := newFakeRegistrar()
		registrar.failOn = "github"

		factories := []secretServiceFactory{
			func() secretservice.SecretService {
				return &fakeSecretService{name: "github", headerName: "Authorization"}
			},
		}

		err := registerAllWithFactories(registrar, factories)
		if err == nil {
			t.Error("registerAllWithFactories() should return error when registration fails")
		}
	})

	t.Run("returns error for nil factory result", func(t *testing.T) {
		t.Parallel()

		registrar := newFakeRegistrar()

		factories := []secretServiceFactory{
			func() secretservice.SecretService {
				return nil
			},
		}

		err := registerAllWithFactories(registrar, factories)
		if err == nil {
			t.Error("registerAllWithFactories() should return error when factory returns nil")
		}
	})
}

func TestAvailableSecretServicesLoaded(t *testing.T) {
	t.Parallel()

	if len(availableSecretServices) != 3 {
		t.Errorf("availableSecretServices should have 3 entries, got %d", len(availableSecretServices))
	}
}

func TestAvailableSecretServicesContainGitHub(t *testing.T) {
	t.Parallel()

	if len(availableSecretServices) == 0 {
		t.Fatal("availableSecretServices is empty")
	}

	svc := availableSecretServices[0]()
	if svc == nil {
		t.Fatal("factory returned nil")
	}

	if svc.Name() != "github" {
		t.Errorf("Name() = %q, want %q", svc.Name(), "github")
	}
	if svc.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if len(svc.HostsPatterns()) == 0 || svc.HostsPatterns()[0] != "api.github.com" {
		t.Errorf("HostsPatterns() = %v, want %v", svc.HostsPatterns(), []string{"api.github.com"})
	}
	if svc.Path() != "" {
		t.Errorf("Path() = %q, want empty string", svc.Path())
	}
	if svc.HeaderName() != "Authorization" {
		t.Errorf("HeaderName() = %q, want %q", svc.HeaderName(), "Authorization")
	}
	if svc.HeaderTemplate() != "Bearer ${value}" {
		t.Errorf("HeaderTemplate() = %q, want %q", svc.HeaderTemplate(), "Bearer ${value}")
	}

	envVars := svc.EnvVars()
	if len(envVars) != 2 {
		t.Fatalf("EnvVars() has %d entries, want 2", len(envVars))
	}
	if envVars[0] != "GH_TOKEN" {
		t.Errorf("EnvVars()[0] = %q, want %q", envVars[0], "GH_TOKEN")
	}
	if envVars[1] != "GITHUB_TOKEN" {
		t.Errorf("EnvVars()[1] = %q, want %q", envVars[1], "GITHUB_TOKEN")
	}
}

func TestAvailableSecretServicesContainGemini(t *testing.T) {
	t.Parallel()

	if len(availableSecretServices) < 2 {
		t.Fatal("availableSecretServices has fewer than 2 entries")
	}

	svc := availableSecretServices[1]()
	if svc == nil {
		t.Fatal("factory returned nil")
	}

	if svc.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", svc.Name(), "gemini")
	}
	if svc.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if len(svc.HostsPatterns()) == 0 || svc.HostsPatterns()[0] != "generativelanguage.googleapis.com" {
		t.Errorf("HostsPatterns() = %v, want %v", svc.HostsPatterns(), []string{"generativelanguage.googleapis.com"})
	}
	if svc.Path() != "" {
		t.Errorf("Path() = %q, want empty string", svc.Path())
	}
	if svc.HeaderName() != "x-goog-api-key" {
		t.Errorf("HeaderName() = %q, want %q", svc.HeaderName(), "x-goog-api-key")
	}
	if svc.HeaderTemplate() != "${value}" {
		t.Errorf("HeaderTemplate() = %q, want %q", svc.HeaderTemplate(), "${value}")
	}

	envVars := svc.EnvVars()
	if len(envVars) != 3 {
		t.Fatalf("EnvVars() has %d entries, want 2", len(envVars))
	}
	if envVars[0] != "GEMINI_API_KEY" {
		t.Errorf("EnvVars()[0] = %q, want %q", envVars[0], "GEMINI_API_KEY")
	}
	if envVars[1] != "GOOGLE_API_KEY" {
		t.Errorf("EnvVars()[1] = %q, want %q", envVars[1], "GOOGLE_API_KEY")
	}
	if envVars[2] != "GOOGLE_GENERATIVE_AI_API_KEY" {
		t.Errorf("EnvVars()[2] = %q, want %q", envVars[2], "GOOGLE_GENERATIVE_AI_API_KEY")
	}
}

func TestAvailableSecretServicesContainAnthropic(t *testing.T) {
	t.Parallel()

	if len(availableSecretServices) < 3 {
		t.Fatal("availableSecretServices has fewer than 3 entries")
	}

	svc := availableSecretServices[2]()
	if svc == nil {
		t.Fatal("factory returned nil")
	}

	if svc.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", svc.Name(), "anthropic")
	}
	if svc.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if len(svc.HostsPatterns()) == 0 || svc.HostsPatterns()[0] != "api.anthropic.com" {
		t.Errorf("HostsPatterns() = %v, want %v", svc.HostsPatterns(), []string{"api.anthropic.com"})
	}
	if svc.Path() != "" {
		t.Errorf("Path() = %q, want empty string", svc.Path())
	}
	if svc.HeaderName() != "x-api-key" {
		t.Errorf("HeaderName() = %q, want %q", svc.HeaderName(), "x-api-key")
	}
	if svc.HeaderTemplate() != "" {
		t.Errorf("HeaderTemplate() = %q, want empty string", svc.HeaderTemplate())
	}

	envVars := svc.EnvVars()
	if len(envVars) != 1 {
		t.Fatalf("EnvVars() has %d entries, want 1", len(envVars))
	}
	if envVars[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("EnvVars()[0] = %q, want %q", envVars[0], "ANTHROPIC_API_KEY")
	}
}

func TestListServicesWithFactories(t *testing.T) {
	t.Parallel()

	t.Run("returns constructed services from factories", func(t *testing.T) {
		t.Parallel()

		factories := []secretServiceFactory{
			func() secretservice.SecretService {
				return &fakeSecretService{name: "svc-a", hostsPatterns: []string{"a.example.com"}}
			},
			func() secretservice.SecretService {
				return &fakeSecretService{name: "svc-b", hostsPatterns: []string{"b.example.com"}}
			},
		}

		services := listServicesWithFactories(factories)

		if len(services) != 2 {
			t.Fatalf("expected 2 services, got %d", len(services))
		}
		if services[0].Name() != "svc-a" {
			t.Errorf("services[0].Name() = %q, want %q", services[0].Name(), "svc-a")
		}
		if services[1].Name() != "svc-b" {
			t.Errorf("services[1].Name() = %q, want %q", services[1].Name(), "svc-b")
		}
	})

	t.Run("returns empty slice for empty factory list", func(t *testing.T) {
		t.Parallel()

		services := listServicesWithFactories([]secretServiceFactory{})
		if len(services) != 0 {
			t.Errorf("expected empty slice, got %d services", len(services))
		}
	})
}

func TestListAvailableWithFactories(t *testing.T) {
	t.Parallel()

	factories := []secretServiceFactory{
		func() secretservice.SecretService { return &fakeSecretService{name: "alpha"} },
		func() secretservice.SecretService { return &fakeSecretService{name: "beta"} },
	}

	names := listAvailableWithFactories(factories)

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "alpha" {
		t.Errorf("names[0] = %q, want %q", names[0], "alpha")
	}
	if names[1] != "beta" {
		t.Errorf("names[1] = %q, want %q", names[1], "beta")
	}
}

func TestListAvailable(t *testing.T) {
	t.Parallel()

	names := ListAvailable()

	if len(names) != 3 {
		t.Fatalf("ListAvailable() returned %d names, want 3", len(names))
	}
	// Order matches secretservices.json: github, gemini, anthropic.
	expected := []string{"github", "gemini", "anthropic"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestListServices(t *testing.T) {
	t.Parallel()

	services := ListServices()

	if len(services) != 3 {
		t.Fatalf("ListServices() returned %d services, want 3", len(services))
	}
	// Verify each service is a non-nil, fully-constructed instance.
	for i, svc := range services {
		if svc == nil {
			t.Errorf("services[%d] is nil", i)
			continue
		}
		if svc.Name() == "" {
			t.Errorf("services[%d].Name() is empty", i)
		}
		if len(svc.EnvVars()) == 0 {
			t.Errorf("services[%d] (%s) has no env vars", i, svc.Name())
		}
	}
	if services[0].Name() != "github" {
		t.Errorf("services[0].Name() = %q, want %q", services[0].Name(), "github")
	}
}

func TestLoadSecretServices(t *testing.T) {
	t.Parallel()

	factories, err := loadSecretServices()
	if err != nil {
		t.Fatalf("loadSecretServices() error = %v", err)
	}

	if len(factories) != 3 {
		t.Fatalf("loadSecretServices() returned %d factories, want 3", len(factories))
	}

	svc := factories[0]()
	if svc == nil {
		t.Fatal("factory returned nil")
	}

	if svc.Name() != "github" {
		t.Errorf("Name() = %q, want %q", svc.Name(), "github")
	}
}
