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

func TestProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"GitHub Token", "kdn-github-token"},
		{"anthropic-key", "kdn-anthropic-key"},
		{"My  Custom--API", "kdn-my-custom-api"},
		{"  leading-trailing  ", "kdn-leading-trailing"},
		{"simple", "kdn-simple"},
		{"UPPER_CASE", "kdn-upper-case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := providerName(tt.input)
			if got != tt.want {
				t.Errorf("providerName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProviderType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"github", "github"},
		{"anthropic", "anthropic"},
		{"gemini", "generic"},
		{"other", "generic"},
		{"unknown", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := providerType(tt.input)
			if got != tt.want {
				t.Errorf("providerType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnsureProviders_NilConfig(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	names, err := rt.ensureProviders(context.Background(), nil)
	if err != nil {
		t.Fatalf("ensureProviders(nil) failed: %v", err)
	}
	if len(names) > 0 {
		t.Errorf("Expected no provider names, got %v", names)
	}
	if len(fakeExec.RunCalls) > 0 || len(fakeExec.OutputCalls) > 0 {
		t.Error("Expected no executor calls for nil config")
	}
}

func TestEnsureProviders_EmptySecrets(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	cfg := &workspace.WorkspaceConfiguration{}
	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders(empty) failed: %v", err)
	}
	if len(names) > 0 {
		t.Errorf("Expected no provider names, got %v", names)
	}
	if len(fakeExec.RunCalls) > 0 || len(fakeExec.OutputCalls) > 0 {
		t.Error("Expected no executor calls for empty secrets")
	}
}

func TestEnsureProviders_NilStoreOrRegistry(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	secrets := []string{"my-secret"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}
	if len(names) > 0 {
		t.Errorf("Expected no provider names, got %v", names)
	}
	if len(fakeExec.RunCalls) > 0 || len(fakeExec.OutputCalls) > 0 {
		t.Error("Expected no executor calls when store/registry are nil")
	}
}

func TestEnsureProviders_CreatesGitHubProvider(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "my-github", Type: "github"},
		},
		values: map[string]string{"my-github": "ghp_abc123"},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github": &fakeSecretService{
				hosts:   []string{"api.github.com"},
				envVars: []string{"GH_TOKEN", "GITHUB_TOKEN"},
			},
		},
	}

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) != 1 || names[0] != "kdn-my-github" {
		t.Errorf("Expected [kdn-my-github], got %v", names)
	}

	if len(fakeExec.OutputCalls) != 1 {
		t.Fatalf("Expected 1 Output call, got %d", len(fakeExec.OutputCalls))
	}
	if !slices.Contains(fakeExec.OutputCalls[0], "provider") || !slices.Contains(fakeExec.OutputCalls[0], "--names") {
		t.Errorf("Expected provider list --names call, got: %v", fakeExec.OutputCalls[0])
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}

	createCall := fakeExec.RunCalls[0]
	assertContainsAll(t, createCall, "provider", "create", "--name", "kdn-my-github", "--type", "github")
	assertContainsAll(t, createCall, "--credential", "GH_TOKEN=ghp_abc123")
	assertContainsAll(t, createCall, "GITHUB_TOKEN=ghp_abc123")
}

func TestEnsureProviders_SkipsExistingProvider(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-my-github\nother-provider\n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "my-github", Type: "github"},
		},
		values: map[string]string{"my-github": "ghp_abc123"},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github": &fakeSecretService{
				hosts:   []string{"api.github.com"},
				envVars: []string{"GH_TOKEN", "GITHUB_TOKEN"},
			},
		},
	}

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) != 1 || names[0] != "kdn-my-github" {
		t.Errorf("Expected [kdn-my-github] for existing provider, got %v", names)
	}

	if len(fakeExec.RunCalls) > 0 {
		t.Errorf("Expected no Run calls when provider already exists, got: %v", fakeExec.RunCalls)
	}
}

func TestEnsureProviders_GenericFallback(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "custom-api", Type: secret.TypeOther, Envs: []string{"MY_TOKEN"}},
		},
		values: map[string]string{"custom-api": "tok_xyz"},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{}

	secrets := []string{"custom-api"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) != 1 || names[0] != "kdn-custom-api" {
		t.Errorf("Expected [kdn-custom-api], got %v", names)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("Expected 1 Run call, got %d", len(fakeExec.RunCalls))
	}

	createCall := fakeExec.RunCalls[0]
	assertContainsAll(t, createCall, "--type", "generic")
	assertContainsAll(t, createCall, "--credential", "MY_TOKEN=tok_xyz")
}

func TestEnsureProviders_MultipleSecrets(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "gh-token", Type: "github"},
			{Name: "claude-key", Type: "anthropic"},
		},
		values: map[string]string{
			"gh-token":   "ghp_123",
			"claude-key": "sk-ant-456",
		},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github":    &fakeSecretService{hosts: []string{"api.github.com"}, envVars: []string{"GITHUB_TOKEN"}},
			"anthropic": &fakeSecretService{hosts: []string{"api.anthropic.com"}, envVars: []string{"ANTHROPIC_API_KEY"}},
		},
	}

	secrets := []string{"gh-token", "claude-key"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) != 2 {
		t.Errorf("Expected 2 provider names, got %v", names)
	}

	if len(fakeExec.RunCalls) != 2 {
		t.Fatalf("Expected 2 Run calls (one per secret), got %d", len(fakeExec.RunCalls))
	}
}

func TestEnsureProviders_SecretNotInStore(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{}

	secrets := []string{"nonexistent"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) > 0 {
		t.Errorf("Expected no provider names, got %v", names)
	}

	if len(fakeExec.RunCalls) > 0 {
		t.Errorf("Expected no Run calls for missing secret, got: %v", fakeExec.RunCalls)
	}
}

func TestEnsureProviders_ProviderCreateError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.RunFunc = func(_ context.Context, args ...string) error {
		return fmt.Errorf("provider create failed")
	}

	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "my-github", Type: "github"},
		},
		values: map[string]string{"my-github": "ghp_abc123"},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"github": &fakeSecretService{hosts: []string{"api.github.com"}, envVars: []string{"GITHUB_TOKEN"}},
		},
	}

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	_, err := rt.ensureProviders(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error when provider create fails")
	}
}

func TestEnsureProviders_ListProviderError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("list failed")
	}

	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "my-github", Type: "github"},
		},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{}

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	_, err := rt.ensureProviders(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error when provider list fails")
	}
}

func TestEnsureProviders_StoreListError(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{err: fmt.Errorf("store unavailable")}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{}

	secrets := []string{"my-github"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	_, err := rt.ensureProviders(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error when store.List() fails")
	}
}

func TestEnsureProviders_NoEnvVars(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.secretStore = &fakeSecretStore{
		items: []secret.ListItem{
			{Name: "empty-svc", Type: "empty"},
		},
		values: map[string]string{"empty-svc": "val"},
	}
	rt.secretServiceRegistry = &fakeSecretServiceRegistry{
		services: map[string]secretservice.SecretService{
			"empty": &fakeSecretService{hosts: []string{"example.com"}, envVars: []string{}},
		},
	}

	secrets := []string{"empty-svc"}
	cfg := &workspace.WorkspaceConfiguration{Secrets: &secrets}

	names, err := rt.ensureProviders(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ensureProviders() failed: %v", err)
	}

	if len(names) > 0 {
		t.Errorf("Expected no provider names, got %v", names)
	}

	if len(fakeExec.RunCalls) > 0 {
		t.Errorf("Expected no Run calls when service has no env vars, got: %v", fakeExec.RunCalls)
	}
}

func TestListExistingProviders(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	fakeExec.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("kdn-github-token\nkdn-anthropic-key\n\n  other-provider  \n"), nil
	}

	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	got, err := rt.listExistingProviders(context.Background())
	if err != nil {
		t.Fatalf("listExistingProviders() failed: %v", err)
	}

	want := map[string]bool{
		"kdn-github-token":  true,
		"kdn-anthropic-key": true,
		"other-provider":    true,
	}

	if len(got) != len(want) {
		t.Fatalf("listExistingProviders() returned %d providers, want %d: %v", len(got), len(want), got)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("Missing provider %q in result", name)
		}
	}
}

// assertContainsAll checks that the slice contains all specified values.
func assertContainsAll(t *testing.T, got []string, values ...string) {
	t.Helper()

	for _, v := range values {
		if !slices.Contains(got, v) {
			t.Errorf("Expected %q in %v", v, got)
		}
	}
}
