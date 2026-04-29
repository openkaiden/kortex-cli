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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/autoconf"
	"github.com/openkaiden/kdn/pkg/cmd/testutil"
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
// --storage flag and leaves injectable fields (confirm, selectTarget, detector)
// set to non-nil defaults.
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

// TestDetectProjectID_NonGitDir verifies that a directory that is not a git
// repository is returned as-is as the project identifier.
func TestDetectProjectID_NonGitDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got := detectProjectID(context.Background(), dir)
	if got != dir {
		t.Errorf("expected %q for non-git dir, got %q", dir, got)
	}
}

// TestDetectProjectID_GitRepo_NoRemote verifies that a git repository without
// a remote uses its root directory as the project identifier.
func TestDetectProjectID_GitRepo_NoRemote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	got := detectProjectID(context.Background(), dir)
	if got != dir {
		t.Errorf("expected root dir %q, got %q", dir, got)
	}
}

// TestDetectProjectID_GitRepo_WithRemote verifies that a git repository with a
// configured remote returns the remote URL (with trailing slash) as the
// project identifier.
func TestDetectProjectID_GitRepo_WithRemote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Resolve symlinks: git rev-parse --show-toplevel returns the real path, so
	// the dir we pass must also be canonical (on macOS /var → /private/var).
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "remote", "add", "origin", "https://github.com/example/repo"},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			t.Skipf("git setup failed: %v", err)
		}
	}

	got := detectProjectID(context.Background(), dir)
	want := "https://github.com/example/repo/"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestDetectProjectID_GitRepo_WithRemote_Subdir verifies that when the working
// directory is a subdirectory of the git root, the relative path is appended to
// the remote URL.
func TestDetectProjectID_GitRepo_WithRemote_Subdir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Resolve symlinks: git rev-parse --show-toplevel returns the real path, so
	// the root we pass must also be canonical (on macOS /var → /private/var).
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	subdir := filepath.Join(root, "sub", "dir")
	cmds := [][]string{
		{"git", "-C", root, "init"},
		{"git", "-C", root, "remote", "add", "origin", "https://github.com/example/repo"},
		{"mkdir", "-p", subdir},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			t.Skipf("git setup failed: %v", err)
		}
	}

	got := detectProjectID(context.Background(), subdir)
	want := "https://github.com/example/repo/sub/dir"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
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
