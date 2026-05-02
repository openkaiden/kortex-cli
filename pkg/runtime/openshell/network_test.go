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
	"context"
	"fmt"
	"slices"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

func TestApplyNetworkPolicy_NilConfig_NoRegistry(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	err := rt.applyNetworkPolicy(context.Background(), "kdn-test", nil, nil)
	if err != nil {
		t.Fatalf("applyNetworkPolicy() failed: %v", err)
	}

	// No registry and no allow hosts means no endpoints added
	for _, call := range fakeExec.RunCalls {
		for _, arg := range call {
			if arg == "--add-endpoint" {
				t.Errorf("Expected no --add-endpoint calls without registry, got: %v", call)
			}
		}
	}
}

func TestApplyNetworkPolicy_AllowMode_WithRegistry(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"anthropic": &fakeSecretService{hosts: []string{"api.anthropic.com"}},
			"github":    &fakeSecretService{hosts: []string{"api.github.com"}},
		},
	}

	mode := workspace.Allow
	cfg := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{
			Mode: &mode,
		},
	}

	err := rt.applyNetworkPolicy(context.Background(), "kdn-test", cfg, nil)
	if err != nil {
		t.Fatalf("applyNetworkPolicy() failed: %v", err)
	}

	assertPolicyUpdateContains(t, fakeExec.RunCalls, "api.anthropic.com:443:full", "api.github.com:443:full")
}

func TestApplyNetworkPolicy_AllowMode_WithAllowHosts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"anthropic": &fakeSecretService{hosts: []string{"api.anthropic.com"}},
		},
	}

	err := rt.applyNetworkPolicy(context.Background(), "kdn-test", nil, []string{"github.com", "registry.npmjs.org"})
	if err != nil {
		t.Fatalf("applyNetworkPolicy() failed: %v", err)
	}

	assertPolicyUpdateContains(t, fakeExec.RunCalls, "api.anthropic.com:443:full", "github.com:443:full", "registry.npmjs.org:443:full")
}

func TestApplyNetworkPolicy_DenyWithHosts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	mode := workspace.Deny
	hosts := []string{"api.github.com", "*.example.com"}
	cfg := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{
			Mode:  &mode,
			Hosts: &hosts,
		},
	}

	err := rt.applyNetworkPolicy(context.Background(), "kdn-test", cfg, nil)
	if err != nil {
		t.Fatalf("applyNetworkPolicy() failed: %v", err)
	}

	assertPolicyUpdateContains(t, fakeExec.RunCalls, "api.github.com:80:full", "api.github.com:443:full", "*.example.com:80:full", "*.example.com:443:full")
	assertPolicyUpdateNotContains(t, fakeExec.RunCalls, "**:80:full")
}

func TestApplyNetworkPolicy_DenyNoHosts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	mode := workspace.Deny
	cfg := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{
			Mode: &mode,
		},
	}

	err := rt.applyNetworkPolicy(context.Background(), "kdn-test", cfg, nil)
	if err != nil {
		t.Fatalf("applyNetworkPolicy() failed: %v", err)
	}

	// Should only have the remove-rule call, no add-endpoint
	for _, call := range fakeExec.RunCalls {
		for _, arg := range call {
			if arg == "--add-endpoint" {
				t.Errorf("Expected no --add-endpoint calls for deny with no hosts, got: %v", call)
			}
		}
	}
}

func TestConfigureNetworkPolicy_RemovesExistingRule(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	err := rt.configureNetworkPolicy(context.Background(), "kdn-test", []string{"example.com"})
	if err != nil {
		t.Fatalf("configureNetworkPolicy() failed: %v", err)
	}

	if len(fakeExec.RunCalls) < 2 {
		t.Fatalf("Expected at least 2 Run calls, got %d", len(fakeExec.RunCalls))
	}

	// First call should be remove-rule
	removeCall := fakeExec.RunCalls[0]
	if !slices.Contains(removeCall, "--remove-rule") || !slices.Contains(removeCall, networkRuleName) {
		t.Errorf("First call should remove existing rule, got: %v", removeCall)
	}
}

func TestConfigureNetworkPolicy_EmptyHosts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	err := rt.configureNetworkPolicy(context.Background(), "kdn-test", nil)
	if err != nil {
		t.Fatalf("configureNetworkPolicy() failed: %v", err)
	}

	// Should only have the remove-rule call
	if len(fakeExec.RunCalls) != 1 {
		t.Errorf("Expected 1 Run call (remove-rule only), got %d", len(fakeExec.RunCalls))
	}
}

func TestConfigureNetworkPolicy_PolicyUpdateError(t *testing.T) {
	t.Parallel()

	callCount := 0
	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("policy update failed")
		}
		return nil
	}
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	err := rt.configureNetworkPolicy(context.Background(), "kdn-test", []string{"example.com"})
	if err == nil {
		t.Error("Expected error when policy update fails")
	}
}

func TestConfigureNetworkPolicy_IncludesBinaryAndWait(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	err := rt.configureNetworkPolicy(context.Background(), "kdn-test", []string{"example.com"})
	if err != nil {
		t.Fatalf("configureNetworkPolicy() failed: %v", err)
	}

	// Find the policy update call (second call, after remove-rule)
	if len(fakeExec.RunCalls) < 2 {
		t.Fatalf("Expected at least 2 Run calls, got %d", len(fakeExec.RunCalls))
	}

	updateCall := fakeExec.RunCalls[1]
	if !slices.Contains(updateCall, "--binary") || !slices.Contains(updateCall, "/**") {
		t.Errorf("Expected --binary /** in update call, got: %v", updateCall)
	}
	if !slices.Contains(updateCall, "--wait") {
		t.Errorf("Expected --wait in update call, got: %v", updateCall)
	}
}

func TestMergeHosts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "nil inputs",
			want: nil,
		},
		{
			name: "a only",
			a:    []string{"a.com", "b.com"},
			want: []string{"a.com", "b.com"},
		},
		{
			name: "b only",
			b:    []string{"c.com"},
			want: []string{"c.com"},
		},
		{
			name: "merge with dedup",
			a:    []string{"a.com", "b.com"},
			b:    []string{"b.com", "c.com"},
			want: []string{"a.com", "b.com", "c.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mergeHosts(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("mergeHosts() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mergeHosts()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCollectSecretHosts_NilInputs(t *testing.T) {
	t.Parallel()

	got, err := collectSecretHosts(nil, nil, nil)
	if err != nil {
		t.Fatalf("collectSecretHosts() failed: %v", err)
	}
	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

func TestCollectSecretHosts_NoSecrets(t *testing.T) {
	t.Parallel()

	cfg := &workspace.WorkspaceConfiguration{}
	got, err := collectSecretHosts(cfg, nil, nil)
	if err != nil {
		t.Fatalf("collectSecretHosts() failed: %v", err)
	}
	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

func TestCollectSecretHosts_NilStoreWithSecrets(t *testing.T) {
	t.Parallel()

	secrets := []string{"my-secret"}
	cfg := &workspace.WorkspaceConfiguration{
		Secrets: &secrets,
	}

	got, err := collectSecretHosts(cfg, nil, nil)
	if err != nil {
		t.Fatalf("collectSecretHosts() failed: %v", err)
	}
	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

func TestCollectSecretHosts_StoreListError(t *testing.T) {
	t.Parallel()

	secrets := []string{"my-secret"}
	cfg := &workspace.WorkspaceConfiguration{
		Secrets: &secrets,
	}

	store := &fakeSecretStore{err: fmt.Errorf("store unavailable")}
	registry := &fakeSecretServiceRegistry{}

	_, err := collectSecretHosts(cfg, store, registry)
	if err == nil {
		t.Error("Expected error when store.List() fails")
	}
}

func TestCollectSecretHosts_KnownType(t *testing.T) {
	t.Parallel()

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{
		Secrets: &secrets,
	}

	store := &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "my-github", Type: "github"},
		},
	}

	registry := &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github": &fakeSecretService{hosts: []string{"api.github.com", "github.com"}},
		},
	}

	got, err := collectSecretHosts(cfg, store, registry)
	if err != nil {
		t.Fatalf("collectSecretHosts() failed: %v", err)
	}

	want := []string{"api.github.com", "github.com"}
	if len(got) != len(want) {
		t.Fatalf("collectSecretHosts() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("collectSecretHosts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCollectSecretHosts_OtherType(t *testing.T) {
	t.Parallel()

	secrets := []string{"custom-api"}
	cfg := &workspace.WorkspaceConfiguration{
		Secrets: &secrets,
	}

	store := &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "custom-api", Type: secret.TypeOther, Hosts: []string{"custom.api.com"}},
		},
	}

	registry := &fakeSecretServiceRegistry{}

	got, err := collectSecretHosts(cfg, store, registry)
	if err != nil {
		t.Fatalf("collectSecretHosts() failed: %v", err)
	}

	if len(got) != 1 || got[0] != "custom.api.com" {
		t.Errorf("collectSecretHosts() = %v, want [custom.api.com]", got)
	}
}

func TestCollectAllRegistryHosts_NilRegistry(t *testing.T) {
	t.Parallel()

	got := collectAllRegistryHosts(nil)
	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

func TestCollectAllRegistryHosts_EmptyRegistry(t *testing.T) {
	t.Parallel()

	registry := &fakeSecretServiceRegistry{}
	got := collectAllRegistryHosts(registry)
	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

func TestCollectAllRegistryHosts_MultipleServices(t *testing.T) {
	t.Parallel()

	registry := &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github":    &fakeSecretService{hosts: []string{"api.github.com"}},
			"anthropic": &fakeSecretService{hosts: []string{"api.anthropic.com"}},
		},
	}

	got := collectAllRegistryHosts(registry)
	if len(got) != 2 {
		t.Fatalf("Expected 2 hosts, got %v", got)
	}

	seen := map[string]bool{}
	for _, h := range got {
		seen[h] = true
	}
	for _, want := range []string{"api.github.com", "api.anthropic.com"} {
		if !seen[want] {
			t.Errorf("Missing expected host %q in %v", want, got)
		}
	}
}

func TestCollectAllRegistryHosts_Deduplication(t *testing.T) {
	t.Parallel()

	registry := &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"svc-a": &fakeSecretService{hosts: []string{"shared.com", "a-only.com"}},
			"svc-b": &fakeSecretService{hosts: []string{"shared.com", "b-only.com"}},
		},
	}

	got := collectAllRegistryHosts(registry)
	if len(got) != 3 {
		t.Fatalf("Expected 3 hosts (deduplicated), got %v", got)
	}

	seen := map[string]bool{}
	for _, h := range got {
		seen[h] = true
	}
	for _, want := range []string{"shared.com", "a-only.com", "b-only.com"} {
		if !seen[want] {
			t.Errorf("Missing expected host %q in %v", want, got)
		}
	}
}

// assertPolicyUpdateContains checks that at least one RunCall contains all the specified endpoint strings.
func assertPolicyUpdateContains(t *testing.T, calls [][]string, endpoints ...string) {
	t.Helper()

	for _, ep := range endpoints {
		found := false
		for _, call := range calls {
			for _, arg := range call {
				if arg == ep {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("Expected endpoint %q in RunCalls, not found. Calls: %v", ep, calls)
		}
	}
}

// assertPolicyUpdateNotContains checks that no RunCall contains any of the specified endpoint strings.
func assertPolicyUpdateNotContains(t *testing.T, calls [][]string, endpoints ...string) {
	t.Helper()

	for _, ep := range endpoints {
		for _, call := range calls {
			for _, arg := range call {
				if arg == ep {
					t.Errorf("Unexpected endpoint %q in RunCalls: %v", ep, call)
				}
			}
		}
	}
}

// fakeSecretStore implements secret.Store for testing.
type fakeSecretStore struct {
	items []secret.ListItem
	err   error
}

func (f *fakeSecretStore) Create(secret.CreateParams) error { return nil }
func (f *fakeSecretStore) List() ([]secret.ListItem, error) { return f.items, f.err }
func (f *fakeSecretStore) Get(string) (secret.ListItem, string, error) {
	return secret.ListItem{}, "", nil
}
func (f *fakeSecretStore) Remove(string) error { return nil }

// fakeSecretServiceRegistry implements secretservice.Registry for testing.
type fakeSecretServiceRegistry struct {
	services map[string]secretservice.SecretService
}

func (f *fakeSecretServiceRegistry) Register(svc secretservice.SecretService) error { return nil }

func (f *fakeSecretServiceRegistry) Get(name string) (secretservice.SecretService, error) {
	svc, ok := f.services[name]
	if !ok {
		return nil, fmt.Errorf("not found: %s", name)
	}
	return svc, nil
}

func (f *fakeSecretServiceRegistry) List() []string {
	if f.services == nil {
		return nil
	}
	names := make([]string, 0, len(f.services))
	for name := range f.services {
		names = append(names, name)
	}
	return names
}

// fakeSecretService implements secretservice.SecretService for testing.
type fakeSecretService struct {
	hosts []string
}

func (f *fakeSecretService) Name() string            { return "fake" }
func (f *fakeSecretService) Description() string     { return "" }
func (f *fakeSecretService) HostsPatterns() []string { return f.hosts }
func (f *fakeSecretService) Path() string            { return "" }
func (f *fakeSecretService) EnvVars() []string       { return nil }
func (f *fakeSecretService) HeaderName() string      { return "" }
func (f *fakeSecretService) HeaderTemplate() string  { return "" }

func TestConfigureNetworkPolicy_NoSpec(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		for _, arg := range args {
			if arg == "--add-endpoint" {
				return fmt.Errorf("exit status 1\nopenshell stderr:\nsandbox has no spec")
			}
		}
		return nil
	}

	rt := newWithDeps(fakeExec, "/fake/openshell-gateway", t.TempDir())

	err := rt.configureNetworkPolicy(context.Background(), "kdn-test", []string{"github.com"})
	if err != nil {
		t.Errorf("Expected no error for 'sandbox has no spec', got: %v", err)
	}
}
