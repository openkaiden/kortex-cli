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
	"fmt"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/secret"
)

// fakeAutoconfStore is a minimal secret.Store fake for Autoconf tests.
type fakeAutoconfStore struct {
	created  []secret.CreateParams
	existing map[string]struct{}
	getErr   error
}

func (f *fakeAutoconfStore) Create(params secret.CreateParams) error {
	f.created = append(f.created, params)
	return nil
}

func (f *fakeAutoconfStore) Remove(name string) error { return nil }

func (f *fakeAutoconfStore) List() ([]secret.ListItem, error) { return nil, nil }

func (f *fakeAutoconfStore) Get(name string) (secret.ListItem, string, error) {
	if f.getErr != nil {
		return secret.ListItem{}, "", f.getErr
	}
	if _, ok := f.existing[name]; ok {
		return secret.ListItem{Name: name, Type: name}, "existing-value", nil
	}
	return secret.ListItem{}, "", fmt.Errorf("secret %q: %w", name, secret.ErrSecretNotFound)
}

// fakeAutoconfUpdater records AddSecret calls for the project updater.
type fakeAutoconfUpdater struct {
	calls []struct{ projectID, secretName string }
	err   error
}

func (f *fakeAutoconfUpdater) AddSecret(projectID, secretName string) error {
	f.calls = append(f.calls, struct{ projectID, secretName string }{projectID, secretName})
	return f.err
}

func (f *fakeAutoconfUpdater) AddMount(_, _, _ string, _ bool) error {
	return f.err
}

// fakeWorkspaceUpdater records calls for the workspace updater.
type fakeWorkspaceUpdater struct {
	added   []string
	envVars []struct{ name, value string }
	mounts  []struct {
		host, target string
		ro           bool
	}
}

func (f *fakeWorkspaceUpdater) AddSecret(name string) error {
	f.added = append(f.added, name)
	return nil
}

func (f *fakeWorkspaceUpdater) AddEnvVar(name, value string) error {
	f.envVars = append(f.envVars, struct{ name, value string }{name, value})
	return nil
}

func (f *fakeWorkspaceUpdater) AddMount(host, target string, ro bool) error {
	f.mounts = append(f.mounts, struct {
		host, target string
		ro           bool
	}{host, target, ro})
	return nil
}

// fakeAutoconfDetector returns a fixed FilterResult.
type fakeAutoconfDetector struct {
	detected   []DetectedSecret   // mapped to NeedsAction
	configured []ConfiguredSecret // mapped to Configured
}

func (f *fakeAutoconfDetector) Detect() (FilterResult, error) {
	return FilterResult{NeedsAction: f.detected, Configured: f.configured}, nil
}

// alwaysGlobal is a selectTarget stub that always picks the global target.
func alwaysGlobal(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
	return ConfigTargetGlobal, nil
}

func TestAutoconf_NoSecrets(t *testing.T) {
	t.Parallel()

	runner := New(Options{
		Detector:       &fakeAutoconfDetector{},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget:   alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "No secrets detected") {
		t.Errorf("expected 'No secrets detected' in output, got: %s", buf.String())
	}
}

func TestAutoconf_SecretsDetected_Confirmed(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk-ant-abc"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget:   alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(store.created) != 1 || store.created[0].Name != "anthropic" {
		t.Errorf("expected secret 'anthropic' to be created, got %v", store.created)
	}
	if len(updater.calls) != 1 || updater.calls[0].secretName != "anthropic" || updater.calls[0].projectID != "" {
		t.Errorf("expected AddSecret('', 'anthropic'), got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), `Created secret "anthropic"`) {
		t.Errorf("expected creation message in output, got: %q", buf.String())
	}
}

func TestAutoconf_SecretsDetected_Declined(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return false, nil },
		SelectTarget:   alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(store.created) != 0 {
		t.Errorf("expected no secrets created after decline, got %v", store.created)
	}
	if len(updater.calls) != 0 {
		t.Errorf("expected no config updates after decline, got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), "Skipped") {
		t.Errorf("expected 'Skipped' in output, got: %s", buf.String())
	}
}

func TestAutoconf_PerSecretConfirmation(t *testing.T) {
	t.Parallel()

	// First secret confirmed, second declined.
	answers := []bool{true, false}
	call := 0
	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk-ant-abc"},
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm: func(string) (bool, error) {
			ans := answers[call]
			call++
			return ans, nil
		},
		SelectTarget: alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if call != 2 {
		t.Errorf("expected confirm called twice, got %d", call)
	}
	if len(store.created) != 1 || store.created[0].Name != "anthropic" {
		t.Errorf("expected only 'anthropic' created, got %v", store.created)
	}
	if len(updater.calls) != 1 || updater.calls[0].secretName != "anthropic" {
		t.Errorf("expected only 'anthropic' config update, got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), `Skipped "github"`) {
		t.Errorf("expected 'Skipped \"github\"' in output, got: %s", buf.String())
	}
}

// TestAutoconf_SecretExistsNotInConfig covers the case where the secret is in
// the store but NOT yet in the project config — it should still be processed so
// the config reference can be added.
func TestAutoconf_SecretExistsNotInConfig(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{
		existing: map[string]struct{}{"anthropic": {}},
	}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk-ant-new"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget:   alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(store.created) != 0 {
		t.Errorf("expected no creation for existing secret, got %v", store.created)
	}
	if len(updater.calls) != 1 || updater.calls[0].secretName != "anthropic" {
		t.Errorf("expected config update even for existing secret, got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), "already exists") {
		t.Errorf("expected 'already exists' message, got: %s", buf.String())
	}
}

// TestAutoconf_AlreadyFullyConfigured covers the case where Detect() returns
// nothing (the filtered detector already removed fully-configured secrets).
func TestAutoconf_AlreadyFullyConfigured(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	confirmCalled := false
	runner := New(Options{
		Detector:       &fakeAutoconfDetector{},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm: func(string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
		SelectTarget: alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if confirmCalled {
		t.Error("expected confirm not to be called when everything is already configured")
	}
	if !strings.Contains(buf.String(), "No secrets detected") {
		t.Errorf("expected 'No secrets detected' in output, got: %s", buf.String())
	}
}

func TestAutoconf_YesFlag_SkipsConfirmationAndDefaultsToGlobal(t *testing.T) {
	t.Parallel()

	confirmCalled := false
	selectCalled := false
	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Yes:            true,
		Confirm: func(string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
		SelectTarget: func(string, []ConfigTargetOption) (ConfigTarget, error) {
			selectCalled = true
			return ConfigTargetGlobal, nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if confirmCalled {
		t.Error("expected confirm not to be called when yes=true")
	}
	if selectCalled {
		t.Error("expected selectTarget not to be called when yes=true")
	}
	if len(store.created) != 1 {
		t.Errorf("expected 1 secret created, got %d", len(store.created))
	}
	if len(updater.calls) != 1 || updater.calls[0].projectID != "" {
		t.Errorf("expected global config update, got %v", updater.calls)
	}
}

func TestAutoconf_SelectProjectTarget(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk-ant-abc"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "https://github.com/user/repo/",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return ConfigTargetProject, nil
		},
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(updater.calls) != 1 || updater.calls[0].projectID != "https://github.com/user/repo/" {
		t.Errorf("expected project-specific update, got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), "project config") {
		t.Errorf("expected 'project config' in output, got: %s", buf.String())
	}
}

func TestAutoconf_SelectLocalTarget(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	projectUpdater := &fakeAutoconfUpdater{}
	workspaceUpdater := &fakeWorkspaceUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:            store,
		ProjectUpdater:   projectUpdater,
		WorkspaceUpdater: workspaceUpdater,
		ProjectID:        "test-project",
		Confirm:          func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return ConfigTargetLocal, nil
		},
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(projectUpdater.calls) != 0 {
		t.Errorf("expected no project config update for local target, got %v", projectUpdater.calls)
	}
	if len(workspaceUpdater.added) != 1 || workspaceUpdater.added[0] != "github" {
		t.Errorf("expected workspace updater to record 'github', got %v", workspaceUpdater.added)
	}
	if !strings.Contains(buf.String(), "local workspace config") {
		t.Errorf("expected 'local workspace config' in output, got: %s", buf.String())
	}
}

func TestAutoconf_SkipAddToConfig(t *testing.T) {
	t.Parallel()

	store := &fakeAutoconfStore{}
	updater := &fakeAutoconfUpdater{}
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          store,
		ProjectUpdater: updater,
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return 0, ErrSkipped
		},
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(store.created) != 1 {
		t.Errorf("expected secret to be created even when config target is skipped, got %d", len(store.created))
	}
	if len(updater.calls) != 0 {
		t.Errorf("expected no config update when target skipped, got %v", updater.calls)
	}
	if !strings.Contains(buf.String(), "Skipped adding") {
		t.Errorf("expected 'Skipped adding' in output, got: %s", buf.String())
	}
}

// TestAutoconf_DisplaysAlreadyConfiguredSecrets verifies that Configured secrets
// from the detector are printed with their location before NeedsAction processing.
func TestAutoconf_DisplaysAlreadyConfiguredSecrets(t *testing.T) {
	t.Parallel()

	runner := New(Options{
		Detector: &fakeAutoconfDetector{
			detected: []DetectedSecret{
				{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
			},
			configured: []ConfiguredSecret{
				{
					DetectedSecret: DetectedSecret{ServiceName: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", Value: "sk"},
					Locations:      []ConfigTarget{ConfigTargetGlobal},
				},
			},
		},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget:   alwaysGlobal,
	})

	buf := &bytes.Buffer{}
	if err := runner.Run(buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"anthropic"`) || !strings.Contains(out, "global") {
		t.Errorf("expected already-configured anthropic with global location in output, got: %s", out)
	}
	if !strings.Contains(out, `"github"`) {
		t.Errorf("expected github (needs action) to be processed, got: %s", out)
	}
}

func TestAutoconf_LocalOptionOnlyOfferedWhenWorkspaceUpdaterSet(t *testing.T) {
	t.Parallel()

	var capturedOptions []ConfigTargetOption
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "test-project",
		// No WorkspaceUpdater → local option must not appear.
		Confirm: func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, opts []ConfigTargetOption) (ConfigTarget, error) {
			capturedOptions = opts
			return ConfigTargetGlobal, nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for _, opt := range capturedOptions {
		if opt.Target == ConfigTargetLocal {
			t.Error("local target should not be offered when WorkspaceUpdater is nil")
		}
	}
	if len(capturedOptions) != 2 {
		t.Errorf("expected 2 options (global + project), got %d", len(capturedOptions))
	}
}

// TestAutoconf_ProjectOptionHiddenWhenNoProjectID verifies that the project target
// is not offered when no project ID was detected.
func TestAutoconf_ProjectOptionHiddenWhenNoProjectID(t *testing.T) {
	t.Parallel()

	var capturedOptions []ConfigTargetOption
	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "", // no project detected
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, opts []ConfigTargetOption) (ConfigTarget, error) {
			capturedOptions = opts
			return ConfigTargetGlobal, nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for _, opt := range capturedOptions {
		if opt.Target == ConfigTargetProject {
			t.Error("project target should not be offered when projectID is empty")
		}
	}
	if len(capturedOptions) != 1 {
		t.Errorf("expected 1 option (global only), got %d", len(capturedOptions))
	}
}

// TestAutoconf_ProjectTarget_EmptyProjectID verifies that injecting ConfigTargetProject
// when projectID is empty returns an error instead of writing to global scope silently.
func TestAutoconf_ProjectTarget_EmptyProjectID(t *testing.T) {
	t.Parallel()

	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return ConfigTargetProject, nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error when ConfigTargetProject selected with empty projectID, got nil")
	}
}

// TestAutoconf_LocalTarget_NilWorkspaceUpdater verifies that selecting the local
// target when no WorkspaceUpdater is set returns an error rather than panicking.
func TestAutoconf_LocalTarget_NilWorkspaceUpdater(t *testing.T) {
	t.Parallel()

	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "test-project",
		// WorkspaceUpdater intentionally nil.
		Confirm: func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return ConfigTargetLocal, nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error when ConfigTargetLocal selected with nil WorkspaceUpdater, got nil")
	}
}

// TestAutoconf_UnknownConfigTarget verifies that an unknown ConfigTarget value
// returned by SelectTarget is rejected with an error.
func TestAutoconf_UnknownConfigTarget(t *testing.T) {
	t.Parallel()

	runner := New(Options{
		Detector: &fakeAutoconfDetector{detected: []DetectedSecret{
			{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
		}},
		Store:          &fakeAutoconfStore{},
		ProjectUpdater: &fakeAutoconfUpdater{},
		ProjectID:      "test-project",
		Confirm:        func(string) (bool, error) { return true, nil },
		SelectTarget: func(_ string, _ []ConfigTargetOption) (ConfigTarget, error) {
			return ConfigTarget(99), nil
		},
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error for unknown ConfigTarget value, got nil")
	}
}
