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
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/secret"
)

// fakeFilterStore is a minimal secret.Store fake for filter tests.
type fakeFilterStore struct {
	existing map[string]struct{}
}

func (f *fakeFilterStore) Create(params secret.CreateParams) error { return nil }
func (f *fakeFilterStore) List() ([]secret.ListItem, error)        { return nil, nil }
func (f *fakeFilterStore) Remove(name string) error                { return nil }
func (f *fakeFilterStore) Get(name string) (secret.ListItem, string, error) {
	if _, ok := f.existing[name]; ok {
		return secret.ListItem{Name: name, Type: name}, "value", nil
	}
	return secret.ListItem{}, "", fmt.Errorf("secret %q: %w", name, secret.ErrSecretNotFound)
}

// fakeFilterLoader is a minimal config.ProjectConfigLoader fake for filter tests.
// It returns the configured secrets for any project ID (the routing between
// global/project is the loader's responsibility, tested in the config package).
type fakeFilterLoader struct {
	secrets []string
}

func (f *fakeFilterLoader) Load(_ string) (*workspace.WorkspaceConfiguration, error) {
	cfg := &workspace.WorkspaceConfiguration{}
	if len(f.secrets) > 0 {
		s := make([]string, len(f.secrets))
		copy(s, f.secrets)
		cfg.Secrets = &s
	}
	return cfg, nil
}

// fakeFilterConfig is a minimal config.Config fake for workspace config tests.
type fakeFilterConfig struct {
	secrets  []string
	notFound bool // simulate ErrConfigNotFound
}

func (f *fakeFilterConfig) Load() (*workspace.WorkspaceConfiguration, error) {
	if f.notFound {
		return nil, config.ErrConfigNotFound
	}
	cfg := &workspace.WorkspaceConfiguration{}
	if len(f.secrets) > 0 {
		s := make([]string, len(f.secrets))
		copy(s, f.secrets)
		cfg.Secrets = &s
	}
	return cfg, nil
}

// helpers to reduce boilerplate in call sites
func newFilter(store secret.Store, loader config.ProjectConfigLoader) SecretFilter {
	return NewAlreadyConfiguredFilter(store, loader, "", nil)
}

func newFilterWithWorkspace(store secret.Store, loader config.ProjectConfigLoader, ws config.Config) SecretFilter {
	return NewAlreadyConfiguredFilter(store, loader, "", ws)
}

func TestFilter_NothingInStoreOrConfig(t *testing.T) {
	t.Parallel()
	f := newFilter(&fakeFilterStore{}, &fakeFilterLoader{})
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Errorf("expected 1 in NeedsAction, got %d", len(got.NeedsAction))
	}
	if len(got.Configured) != 0 {
		t.Errorf("expected 0 in Configured, got %d", len(got.Configured))
	}
}

func TestFilter_InStoreNotInConfig(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"anthropic": {}}}
	f := newFilter(store, &fakeFilterLoader{})
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Errorf("expected 1 in NeedsAction (still needs config update), got %d", len(got.NeedsAction))
	}
}

func TestFilter_NotInStoreInConfig(t *testing.T) {
	t.Parallel()
	loader := &fakeFilterLoader{secrets: []string{"anthropic"}}
	f := newFilter(&fakeFilterStore{}, loader)
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Errorf("expected 1 in NeedsAction (still needs store creation), got %d", len(got.NeedsAction))
	}
}

func TestFilter_InStoreAndInConfig(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"anthropic": {}}}
	loader := &fakeFilterLoader{secrets: []string{"anthropic"}}
	f := newFilter(store, loader)
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 {
		t.Errorf("expected 0 in NeedsAction (fully configured), got %d", len(got.NeedsAction))
	}
	if len(got.Configured) != 1 || got.Configured[0].ServiceName != "anthropic" {
		t.Errorf("expected 'anthropic' in Configured, got %v", got.Configured)
	}
	if len(got.Configured[0].Locations) == 0 {
		t.Error("expected Configured entry to have at least one location")
	}
}

func TestFilter_PartiallyConfigured(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"anthropic": {}}}
	loader := &fakeFilterLoader{secrets: []string{"anthropic"}}
	f := newFilter(store, loader)
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
		{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 || got.NeedsAction[0].ServiceName != "github" {
		t.Errorf("expected only 'github' in NeedsAction, got %v", got.NeedsAction)
	}
	if len(got.Configured) != 1 || got.Configured[0].ServiceName != "anthropic" {
		t.Errorf("expected 'anthropic' in Configured, got %v", got.Configured)
	}
}

func TestFilter_EmptyInput(t *testing.T) {
	t.Parallel()
	f := newFilter(&fakeFilterStore{}, &fakeFilterLoader{})
	got, err := f.Filter(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 || len(got.Configured) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// TestFilter_InStoreAndInWorkspaceConfig verifies that a secret referenced in
// .kaiden/workspace.json is skipped even when absent from the project config.
func TestFilter_InStoreAndInWorkspaceConfig(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"github": {}}}
	ws := &fakeFilterConfig{secrets: []string{"github"}}
	f := newFilterWithWorkspace(store, &fakeFilterLoader{}, ws)
	detected := []DetectedSecret{
		{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 {
		t.Errorf("expected 0 in NeedsAction (secret in store + workspace config), got %d", len(got.NeedsAction))
	}
	if len(got.Configured) != 1 {
		t.Errorf("expected 1 in Configured, got %d", len(got.Configured))
	}
	if len(got.Configured[0].Locations) == 0 {
		t.Error("expected Configured entry to have at least one location")
	}
}

// TestFilter_NotInStoreInWorkspaceConfig verifies that workspace config alone
// (without the store) is not enough to filter out a secret.
func TestFilter_NotInStoreInWorkspaceConfig(t *testing.T) {
	t.Parallel()
	ws := &fakeFilterConfig{secrets: []string{"github"}}
	f := newFilterWithWorkspace(&fakeFilterStore{}, &fakeFilterLoader{}, ws)
	detected := []DetectedSecret{
		{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Errorf("expected 1 in NeedsAction (still needs store creation), got %d", len(got.NeedsAction))
	}
}

// TestFilter_WorkspaceConfigNotFound verifies that ErrConfigNotFound from the
// workspace config is treated as "no local config" and not returned as an error.
func TestFilter_WorkspaceConfigNotFound(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"github": {}}}
	ws := &fakeFilterConfig{notFound: true}
	f := newFilterWithWorkspace(store, &fakeFilterLoader{}, ws)
	detected := []DetectedSecret{
		{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error on ErrConfigNotFound: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Errorf("expected 1 in NeedsAction (workspace config missing = not configured), got %d", len(got.NeedsAction))
	}
}

// TestFilter_AnySourceSuffices verifies that a secret in the store is filtered
// out when referenced in ANY config source, even if absent from the others.
func TestFilter_AnySourceSuffices(t *testing.T) {
	t.Parallel()
	store := &fakeFilterStore{existing: map[string]struct{}{"anthropic": {}, "github": {}}}
	// anthropic is in project config, github is in workspace config.
	loader := &fakeFilterLoader{secrets: []string{"anthropic"}}
	ws := &fakeFilterConfig{secrets: []string{"github"}}
	f := newFilterWithWorkspace(store, loader, ws)
	detected := []DetectedSecret{
		{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
		{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp"},
	}
	got, err := f.Filter(detected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 {
		t.Errorf("expected 0 in NeedsAction (each secret in a different source), got %v", got.NeedsAction)
	}
	if len(got.Configured) != 2 {
		t.Errorf("expected both secrets in Configured, got %v", got.Configured)
	}
}
