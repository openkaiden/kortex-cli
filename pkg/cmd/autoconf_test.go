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

package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/autoconf"
	"github.com/openkaiden/kdn/pkg/cmd/testutil"
	"github.com/openkaiden/kdn/pkg/project"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/spf13/cobra"
)

// fakeAutoconfCmdDetector is a minimal SecretDetector for cmd-level tests.
type fakeAutoconfCmdDetector struct {
	result autoconf.FilterResult
}

func (f *fakeAutoconfCmdDetector) Detect() (autoconf.FilterResult, error) {
	return f.result, nil
}

// fakeProjectDetector is a minimal project.Detector for cmd-level tests.
type fakeProjectDetector struct{}

func (f *fakeProjectDetector) DetectProject(_ context.Context, dir string) string {
	return dir
}

var _ project.Detector = (*fakeProjectDetector)(nil)

func TestAutoconfCmd(t *testing.T) {
	t.Parallel()

	cmd := NewAutoconfCmd()
	if cmd == nil {
		t.Fatal("NewAutoconfCmd() returned nil")
	}
	if cmd.Use != "autoconf" {
		t.Errorf("expected Use %q, got %q", "autoconf", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestAutoconfCmd_Examples(t *testing.T) {
	t.Parallel()

	cmd := NewAutoconfCmd()
	if cmd.Example == "" {
		t.Fatal("Example field should not be empty")
	}

	commands, err := testutil.ParseExampleCommands(cmd.Example)
	if err != nil {
		t.Fatalf("failed to parse examples: %v", err)
	}

	expectedCount := 4
	if len(commands) != expectedCount {
		t.Errorf("expected %d example commands, got %d", expectedCount, len(commands))
	}

	rootCmd := NewRootCmd()
	if err := testutil.ValidateCommandExamples(rootCmd, cmd.Example); err != nil {
		t.Errorf("example validation failed: %v", err)
	}
}

// TestAutoconfCmd_PreRun verifies that preRun wires all dependencies from the
// --storage flag and leaves injectable fields set to non-nil defaults.
func TestAutoconfCmd_PreRun(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	c := &autoconfCmd{}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("storage", storageDir, "")

	if err := c.preRun(cmd, []string{}); err != nil {
		t.Fatalf("preRun returned error: %v", err)
	}

	if c.store == nil {
		t.Error("store not set by preRun")
	}
	if c.projectUpdater == nil {
		t.Error("projectUpdater not set by preRun")
	}
	if c.workspaceUpdater == nil {
		t.Error("workspaceUpdater not set by preRun")
	}
	if c.projectDetector == nil {
		t.Error("projectDetector not set by preRun")
	}
	if c.detector == nil {
		t.Error("detector not set by preRun")
	}
	if c.confirm == nil {
		t.Error("confirm not set by preRun")
	}
	if c.selectTarget == nil {
		t.Error("selectTarget not set by preRun")
	}
	if c.projectID == "" {
		t.Error("projectID not set by preRun")
	}
}

// TestAutoconfCmd_PreRun_PreservesInjectedDetector verifies the nil-guard:
// a pre-populated detector must not be overwritten by preRun.
func TestAutoconfCmd_PreRun_PreservesInjectedDetector(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	injected := &fakeAutoconfCmdDetector{}
	c := &autoconfCmd{detector: injected}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("storage", storageDir, "")

	if err := c.preRun(cmd, []string{}); err != nil {
		t.Fatalf("preRun returned error: %v", err)
	}

	if c.detector != injected {
		t.Error("preRun overwrote the pre-injected detector")
	}
}

// TestAutoconfCmd_PreRun_PreservesInjectedProjectDetector verifies the nil-guard:
// a pre-populated projectDetector must not be overwritten by preRun.
func TestAutoconfCmd_PreRun_PreservesInjectedProjectDetector(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	injected := &fakeProjectDetector{}
	c := &autoconfCmd{projectDetector: injected}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("storage", storageDir, "")

	if err := c.preRun(cmd, []string{}); err != nil {
		t.Fatalf("preRun returned error: %v", err)
	}

	if c.projectDetector != injected {
		t.Error("preRun overwrote the pre-injected projectDetector")
	}
}

// TestAutoconfCmd_PreRun_MissingStorageFlag verifies that preRun returns an
// error when the --storage flag has not been registered on the command.
func TestAutoconfCmd_PreRun_MissingStorageFlag(t *testing.T) {
	t.Parallel()

	c := &autoconfCmd{}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	// Intentionally do not register --storage; GetString must fail.

	err := c.preRun(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when --storage flag is missing, got nil")
	}
}

// TestAutoconfCmd_Run_NoSecrets exercises the run() path when the detector
// reports no detected secrets.
func TestAutoconfCmd_Run_NoSecrets(t *testing.T) {
	t.Parallel()

	c := &autoconfCmd{
		detector: &fakeAutoconfCmdDetector{},
		confirm:  func(string) (bool, error) { return true, nil },
		selectTarget: func(_ string, _ []autoconf.ConfigTargetOption) (autoconf.ConfigTarget, error) {
			return autoconf.ConfigTargetGlobal, nil
		},
	}

	cmd := NewAutoconfCmd()
	var out strings.Builder
	cmd.SetOut(&out)

	if err := c.run(cmd, []string{}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No secrets detected") {
		t.Errorf("expected 'No secrets detected' in output, got: %q", out.String())
	}
}

// TestAutoconfCmd_Run_YesFlag verifies that the --yes flag value is forwarded
// to the autoconf runner (confirm is never called when yes=true).
func TestAutoconfCmd_Run_YesFlag(t *testing.T) {
	t.Parallel()

	confirmCalled := false
	c := &autoconfCmd{
		yes: true,
		detector: &fakeAutoconfCmdDetector{
			result: autoconf.FilterResult{
				NeedsAction: []autoconf.DetectedSecret{
					{ServiceName: "github", EnvVarName: "GH_TOKEN", Value: "ghp_xyz"},
				},
			},
		},
		store:          fakeAutoconfCmdStore{},
		projectUpdater: &fakeAutoconfCmdUpdater{},
		confirm: func(string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
		selectTarget: func(_ string, _ []autoconf.ConfigTargetOption) (autoconf.ConfigTarget, error) {
			return autoconf.ConfigTargetGlobal, nil
		},
	}

	cmd := NewAutoconfCmd()
	cmd.SetOut(&strings.Builder{})

	if err := c.run(cmd, []string{}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if confirmCalled {
		t.Error("confirm must not be called when yes=true")
	}
}

// fakeAutoconfCmdStore satisfies secret.Store for cmd-level run() tests.
type fakeAutoconfCmdStore struct{}

func (fakeAutoconfCmdStore) Create(_ secret.CreateParams) error { return nil }
func (fakeAutoconfCmdStore) List() ([]secret.ListItem, error)   { return nil, nil }
func (fakeAutoconfCmdStore) Remove(_ string) error              { return nil }
func (fakeAutoconfCmdStore) Get(_ string) (secret.ListItem, string, error) {
	return secret.ListItem{}, "", nil
}

// fakeAutoconfCmdUpdater satisfies config.ProjectConfigUpdater for cmd-level tests.
type fakeAutoconfCmdUpdater struct{}

func (f *fakeAutoconfCmdUpdater) AddSecret(_, _ string) error { return nil }
