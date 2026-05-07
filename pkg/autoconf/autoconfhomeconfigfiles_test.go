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
	"errors"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
)

// fakeHomeConfigFilesDetector returns a fixed []DetectedHomeConfigFile.
type fakeHomeConfigFilesDetector struct {
	files []DetectedHomeConfigFile
	err   error
}

func (f *fakeHomeConfigFilesDetector) Detect() ([]DetectedHomeConfigFile, error) {
	return f.files, f.err
}

// fakeProjectUpdater records AddSecret and AddMount calls for the project config updater.
type fakeProjectUpdater struct {
	secrets []struct{ projectID, secretName string }
	mounts  []struct {
		projectID, host, target string
		ro                      bool
	}
	err error
}

func (f *fakeProjectUpdater) AddSecret(projectID, secretName string) error {
	f.secrets = append(f.secrets, struct{ projectID, secretName string }{projectID, secretName})
	return f.err
}

func (f *fakeProjectUpdater) AddMount(projectID, host, target string, ro bool) error {
	f.mounts = append(f.mounts, struct {
		projectID, host, target string
		ro                      bool
	}{projectID, host, target, ro})
	return f.err
}

// fakeProjectLoader returns a fixed *workspace.WorkspaceConfiguration per projectID.
type fakeProjectLoader struct {
	configs map[string]*workspace.WorkspaceConfiguration
}

func (f *fakeProjectLoader) Load(projectID string) (*workspace.WorkspaceConfiguration, error) {
	if cfg, ok := f.configs[projectID]; ok {
		return cfg, nil
	}
	// Mirror the real loader: when no project-specific entry exists but a global
	// one does, return the global config (it is the merged result).
	if projectID != "" {
		if global, ok := f.configs[""]; ok {
			return global, nil
		}
	}
	return &workspace.WorkspaceConfiguration{}, nil
}

// fakeHomeConfigWorkspaceConfig returns a fixed *workspace.WorkspaceConfiguration.
type fakeHomeConfigWorkspaceConfig struct {
	cfg *workspace.WorkspaceConfiguration
}

func (f *fakeHomeConfigWorkspaceConfig) Load() (*workspace.WorkspaceConfiguration, error) {
	if f.cfg != nil {
		return f.cfg, nil
	}
	return nil, config.ErrConfigNotFound
}

func detectedGitconfig() DetectedHomeConfigFile {
	return DetectedHomeConfigFile{
		Name:          "gitconfig",
		HostPath:      "$HOME/.gitconfig",
		ContainerPath: "$HOME/.gitconfig",
	}
}

// alwaysLocalHomeConfig is a selectTarget stub that always picks the local target.
func alwaysLocalHomeConfig(_ []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error) {
	return HomeConfigFilesConfigTargetLocal, nil
}

// alwaysProjectHomeConfig is a selectTarget stub that always picks the project target.
func alwaysProjectHomeConfig(_ []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error) {
	return HomeConfigFilesConfigTargetProject, nil
}

func confirmYes(_ string) (bool, error) { return true, nil }
func confirmNo(_ string) (bool, error)  { return false, nil }

func TestHomeConfigFilesAutoconf_NoFilesDetected(t *testing.T) {
	t.Parallel()

	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{},
		ProjectUpdater: updater,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no mounts, got %v", updater.mounts)
	}
}

func TestHomeConfigFilesAutoconf_YesGlobalTarget(t *testing.T) {
	t.Parallel()

	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		Yes:            true,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(updater.mounts))
	}
	m := updater.mounts[0]
	if m.projectID != "" {
		t.Errorf("projectID = %q, want empty (global)", m.projectID)
	}
	if m.host != "$HOME/.gitconfig" || m.target != "$HOME/.gitconfig" || !m.ro {
		t.Errorf("unexpected mount: %+v", m)
	}
	if !strings.Contains(buf.String(), "global project config") {
		t.Errorf("expected global message, got: %s", buf.String())
	}
}

func TestHomeConfigFilesAutoconf_ConfirmApplyLocalTarget(t *testing.T) {
	t.Parallel()

	projectUpdater := &fakeProjectUpdater{}
	workspaceUpdater := &fakeWorkspaceUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:         &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater:   projectUpdater,
		WorkspaceUpdater: workspaceUpdater,
		Yes:              false,
		Confirm:          confirmYes,
		SelectTarget:     alwaysLocalHomeConfig,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(workspaceUpdater.mounts) != 1 {
		t.Fatalf("expected 1 workspace mount, got %d", len(workspaceUpdater.mounts))
	}
	if len(projectUpdater.mounts) != 0 {
		t.Errorf("expected no project mounts, got %v", projectUpdater.mounts)
	}
	m := workspaceUpdater.mounts[0]
	if m.host != "$HOME/.gitconfig" || m.target != "$HOME/.gitconfig" || !m.ro {
		t.Errorf("unexpected mount: %+v", m)
	}
}

func TestHomeConfigFilesAutoconf_ConfirmApplyProjectTarget(t *testing.T) {
	t.Parallel()

	projectUpdater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:         &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater:   projectUpdater,
		WorkspaceUpdater: &fakeWorkspaceUpdater{},
		ProjectID:        "github.com/org/repo",
		Yes:              false,
		Confirm:          confirmYes,
		SelectTarget:     alwaysProjectHomeConfig,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(projectUpdater.mounts) != 1 {
		t.Fatalf("expected 1 project mount, got %d", len(projectUpdater.mounts))
	}
	m := projectUpdater.mounts[0]
	if m.projectID != "github.com/org/repo" {
		t.Errorf("projectID = %q, want %q", m.projectID, "github.com/org/repo")
	}
}

func TestHomeConfigFilesAutoconf_ConfirmDeclined(t *testing.T) {
	t.Parallel()

	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		Yes:            false,
		Confirm:        confirmNo,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no mounts after decline, got %v", updater.mounts)
	}
	if !strings.Contains(buf.String(), "Skipped") {
		t.Errorf("expected skip message, got: %s", buf.String())
	}
}

func TestHomeConfigFilesAutoconf_SelectTargetSkipped(t *testing.T) {
	t.Parallel()

	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		Yes:            false,
		Confirm:        confirmYes,
		SelectTarget: func(_ []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error) {
			return 0, ErrSkipped
		},
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no mounts after skip, got %v", updater.mounts)
	}
}

func TestHomeConfigFilesAutoconf_AlreadyMountedGlobal(t *testing.T) {
	t.Parallel()

	ro := true
	globalCfg := &workspace.WorkspaceConfiguration{
		Mounts: &[]workspace.Mount{
			{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig", Ro: &ro},
		},
	}
	loader := &fakeProjectLoader{
		configs: map[string]*workspace.WorkspaceConfiguration{
			"": globalCfg,
		},
	}
	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		ProjectLoader:  loader,
		Yes:            true,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no new mounts (already configured), got %v", updater.mounts)
	}
	if !strings.Contains(buf.String(), "already mounted") {
		t.Errorf("expected 'already mounted' message, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "global") {
		t.Errorf("expected 'global' in message, got: %s", buf.String())
	}
}

func TestHomeConfigFilesAutoconf_AlreadyMountedGlobalWithProjectID(t *testing.T) {
	t.Parallel()

	// Mount exists only in global. With a ProjectID set, Load(projectID) would
	// return the merged (global+project) config, which also contains the mount.
	// The runner must NOT report "project" in that case.
	ro := true
	globalCfg := &workspace.WorkspaceConfiguration{
		Mounts: &[]workspace.Mount{
			{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig", Ro: &ro},
		},
	}
	loader := &fakeProjectLoader{
		configs: map[string]*workspace.WorkspaceConfiguration{
			"": globalCfg,
			// "my-project" key intentionally absent — mount comes from global only.
		},
	}
	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		ProjectLoader:  loader,
		ProjectID:      "my-project",
		Yes:            true,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no new mounts (already configured globally), got %v", updater.mounts)
	}
	out := buf.String()
	if !strings.Contains(out, "global") {
		t.Errorf("expected 'global' in message, got: %s", out)
	}
	if strings.Contains(out, "project") {
		t.Errorf("must not report 'project' when mount is only in global, got: %s", out)
	}
}

func TestHomeConfigFilesAutoconf_AlreadyMountedProject(t *testing.T) {
	t.Parallel()

	ro := true
	projectCfg := &workspace.WorkspaceConfiguration{
		Mounts: &[]workspace.Mount{
			{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig", Ro: &ro},
		},
	}
	loader := &fakeProjectLoader{
		configs: map[string]*workspace.WorkspaceConfiguration{
			"my-project": projectCfg,
		},
	}
	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: updater,
		ProjectLoader:  loader,
		ProjectID:      "my-project",
		Yes:            true,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no new mounts (already configured), got %v", updater.mounts)
	}
	if !strings.Contains(buf.String(), "project") {
		t.Errorf("expected 'project' in message, got: %s", buf.String())
	}
}

func TestHomeConfigFilesAutoconf_AlreadyMountedLocal(t *testing.T) {
	t.Parallel()

	ro := true
	workspaceCfg := &fakeHomeConfigWorkspaceConfig{
		cfg: &workspace.WorkspaceConfiguration{
			Mounts: &[]workspace.Mount{
				{Host: "$HOME/.gitconfig", Target: "$HOME/.gitconfig", Ro: &ro},
			},
		},
	}
	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:        &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater:  updater,
		WorkspaceConfig: workspaceCfg,
		Yes:             true,
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(updater.mounts) != 0 {
		t.Errorf("expected no new mounts (already configured locally), got %v", updater.mounts)
	}
}

func TestHomeConfigFilesAutoconf_DetectorError(t *testing.T) {
	t.Parallel()

	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector: &fakeHomeConfigFilesDetector{err: errors.New("stat failed")},
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error from detector, got nil")
	}
}

func TestHomeConfigFilesAutoconf_LocalTargetNotOfferedWhenUpdaterNil(t *testing.T) {
	t.Parallel()

	var capturedOptions []HomeConfigFilesConfigTargetOption
	updater := &fakeProjectUpdater{}
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:         &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater:   updater,
		WorkspaceUpdater: nil, // no local updater
		ProjectID:        "proj",
		Yes:              false,
		Confirm:          confirmYes,
		SelectTarget: func(opts []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error) {
			capturedOptions = opts
			return HomeConfigFilesConfigTargetGlobal, nil
		},
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, opt := range capturedOptions {
		if opt.Target == HomeConfigFilesConfigTargetLocal {
			t.Error("local target should not be offered when WorkspaceUpdater is nil")
		}
	}
}

func TestHomeConfigFilesAutoconf_ProjectTargetNotOfferedWhenNoProjectID(t *testing.T) {
	t.Parallel()

	var capturedOptions []HomeConfigFilesConfigTargetOption
	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: &fakeProjectUpdater{},
		ProjectID:      "", // no project
		Yes:            false,
		Confirm:        confirmYes,
		SelectTarget: func(opts []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error) {
			capturedOptions = opts
			return HomeConfigFilesConfigTargetGlobal, nil
		},
	})

	var buf bytes.Buffer
	if err := runner.Run(&buf); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, opt := range capturedOptions {
		if opt.Target == HomeConfigFilesConfigTargetProject {
			t.Error("project target should not be offered when ProjectID is empty")
		}
	}
}

func TestHomeConfigFilesAutoconf_LocalTargetMissingUpdater(t *testing.T) {
	t.Parallel()

	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:         &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater:   &fakeProjectUpdater{},
		WorkspaceUpdater: nil,
		Yes:              false,
		Confirm:          confirmYes,
		SelectTarget:     alwaysLocalHomeConfig,
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error when local target selected but WorkspaceUpdater is nil")
	}
}

func TestHomeConfigFilesAutoconf_ProjectTargetMissingProjectID(t *testing.T) {
	t.Parallel()

	runner := NewHomeConfigFilesAutoconf(HomeConfigFilesAutoconfOptions{
		Detector:       &fakeHomeConfigFilesDetector{files: []DetectedHomeConfigFile{detectedGitconfig()}},
		ProjectUpdater: &fakeProjectUpdater{},
		ProjectID:      "",
		Yes:            false,
		Confirm:        confirmYes,
		SelectTarget:   alwaysProjectHomeConfig,
	})

	if err := runner.Run(&bytes.Buffer{}); err == nil {
		t.Error("expected error when project target selected but ProjectID is empty")
	}
}
