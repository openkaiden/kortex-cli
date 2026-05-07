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
	"errors"
	"fmt"
	"io"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
)

// HomeConfigFilesConfigTarget identifies where a detected home config file mount is recorded.
type HomeConfigFilesConfigTarget int

const (
	// HomeConfigFilesConfigTargetGlobal records the mount in the global entry of
	// projects.json (applies to all projects and workspaces).
	HomeConfigFilesConfigTargetGlobal HomeConfigFilesConfigTarget = iota
	// HomeConfigFilesConfigTargetProject records the mount in the project-specific
	// entry of projects.json (keyed by the computed project ID).
	HomeConfigFilesConfigTargetProject
	// HomeConfigFilesConfigTargetLocal records the mount in the local
	// .kaiden/workspace.json.
	HomeConfigFilesConfigTargetLocal
)

// HomeConfigFilesConfigTargetOption pairs a HomeConfigFilesConfigTarget with a
// human-readable label for display in selection prompts.
type HomeConfigFilesConfigTargetOption struct {
	Target HomeConfigFilesConfigTarget
	Label  string
}

// HomeConfigFilesAutoconfOptions configures a HomeConfigFilesAutoconf runner.
type HomeConfigFilesAutoconfOptions struct {
	Detector HomeConfigFilesDetector

	// ProjectUpdater writes to ~/.kdn/config/projects.json for global/project targets.
	ProjectUpdater config.ProjectConfigUpdater
	// WorkspaceUpdater writes to .kaiden/workspace.json. When nil the local target
	// is not offered.
	WorkspaceUpdater config.WorkspaceConfigUpdater

	// ProjectLoader is used to check whether a file is already mounted in the
	// global or project-specific config. May be nil (skips those checks).
	ProjectLoader config.ProjectConfigLoader
	// WorkspaceConfig is used to check whether a file is already mounted in the
	// local workspace config. May be nil (skips that check).
	WorkspaceConfig config.Config

	// ProjectID is the project identifier for the current working directory,
	// used for the project target. When empty the project target is not offered.
	ProjectID string

	Yes bool

	// Confirm is called to ask whether to mount a detected file.
	// Returning false skips the file.
	Confirm func(prompt string) (bool, error)

	// SelectTarget is called to ask where to record the mount.
	// It may return ErrSkipped to skip without applying.
	SelectTarget func(options []HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error)
}

// HomeConfigFilesAutoconf orchestrates home config file detection and mount
// application. For each registered config file that exists in the host home
// directory and is not yet mounted, it confirms with the user and writes the
// mount to the chosen configuration target.
type HomeConfigFilesAutoconf interface {
	Run(out io.Writer) error
}

type homeConfigFilesAutoconfRunner struct {
	detector         HomeConfigFilesDetector
	projectUpdater   config.ProjectConfigUpdater
	workspaceUpdater config.WorkspaceConfigUpdater
	projectLoader    config.ProjectConfigLoader
	workspaceConfig  config.Config
	projectID        string
	yes              bool
	confirm          func(string) (bool, error)
	selectTarget     func([]HomeConfigFilesConfigTargetOption) (HomeConfigFilesConfigTarget, error)
}

var _ HomeConfigFilesAutoconf = (*homeConfigFilesAutoconfRunner)(nil)

// NewHomeConfigFilesAutoconf returns a HomeConfigFilesAutoconf configured by opts.
func NewHomeConfigFilesAutoconf(opts HomeConfigFilesAutoconfOptions) HomeConfigFilesAutoconf {
	return &homeConfigFilesAutoconfRunner{
		detector:         opts.Detector,
		projectUpdater:   opts.ProjectUpdater,
		workspaceUpdater: opts.WorkspaceUpdater,
		projectLoader:    opts.ProjectLoader,
		workspaceConfig:  opts.WorkspaceConfig,
		projectID:        opts.ProjectID,
		yes:              opts.Yes,
		confirm:          opts.Confirm,
		selectTarget:     opts.SelectTarget,
	}
}

func (r *homeConfigFilesAutoconfRunner) Run(out io.Writer) error {
	detected, err := r.detector.Detect()
	if err != nil {
		return err
	}

	for _, f := range detected {
		if err := r.processFile(out, f); err != nil {
			return err
		}
	}
	return nil
}

func (r *homeConfigFilesAutoconfRunner) processFile(out io.Writer, f DetectedHomeConfigFile) error {
	locations := r.findExistingLocations(f)
	if len(locations) > 0 {
		fmt.Fprintf(out, "%s %s already mounted (%s).\n", greenCheck, f.HostPath, formatHomeConfigLocations(locations))
		return nil
	}

	fmt.Fprintf(out, "Detected home config file %s\n", f.HostPath)

	if !r.yes {
		ok, err := r.confirm(fmt.Sprintf("Mount %s read-only?", f.HostPath))
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !ok {
			fmt.Fprintf(out, "%s Skipped %s.\n", greyDash, f.HostPath)
			return nil
		}
	}

	target := HomeConfigFilesConfigTargetGlobal // default for --yes
	if !r.yes {
		options := r.buildTargetOptions()
		var selErr error
		target, selErr = r.selectTarget(options)
		if errors.Is(selErr, ErrSkipped) {
			fmt.Fprintf(out, "%s Skipped %s.\n", greyDash, f.HostPath)
			return nil
		}
		if selErr != nil {
			return fmt.Errorf("target selection failed: %w", selErr)
		}
	}

	return r.applyTarget(out, f, target)
}

func (r *homeConfigFilesAutoconfRunner) buildTargetOptions() []HomeConfigFilesConfigTargetOption {
	opts := []HomeConfigFilesConfigTargetOption{
		{Target: HomeConfigFilesConfigTargetGlobal, Label: "Global (all projects)"},
	}
	if r.projectID != "" {
		opts = append(opts, HomeConfigFilesConfigTargetOption{
			Target: HomeConfigFilesConfigTargetProject,
			Label:  fmt.Sprintf("Project (%s)", r.projectID),
		})
	}
	if r.workspaceUpdater != nil {
		opts = append(opts, HomeConfigFilesConfigTargetOption{
			Target: HomeConfigFilesConfigTargetLocal,
			Label:  "Local (.kaiden/workspace.json)",
		})
	}
	return opts
}

func (r *homeConfigFilesAutoconfRunner) applyTarget(out io.Writer, f DetectedHomeConfigFile, target HomeConfigFilesConfigTarget) error {
	switch target {
	case HomeConfigFilesConfigTargetGlobal:
		if err := r.projectUpdater.AddMount("", f.HostPath, f.ContainerPath, true); err != nil {
			return fmt.Errorf("failed to add %s mount to global config: %w", f.HostPath, err)
		}
		fmt.Fprintf(out, "%s Added %s mount to global project config.\n", greenCheck, f.HostPath)

	case HomeConfigFilesConfigTargetProject:
		if r.projectID == "" {
			return fmt.Errorf("project config target selected but no project was detected")
		}
		if err := r.projectUpdater.AddMount(r.projectID, f.HostPath, f.ContainerPath, true); err != nil {
			return fmt.Errorf("failed to add %s mount to project config: %w", f.HostPath, err)
		}
		fmt.Fprintf(out, "%s Added %s mount to project config.\n", greenCheck, f.HostPath)

	case HomeConfigFilesConfigTargetLocal:
		if r.workspaceUpdater == nil {
			return fmt.Errorf("local config target selected but workspace updater is not configured")
		}
		if err := r.workspaceUpdater.AddMount(f.HostPath, f.ContainerPath, true); err != nil {
			return fmt.Errorf("failed to add %s mount to local workspace config: %w", f.HostPath, err)
		}
		fmt.Fprintf(out, "%s Added %s mount to local workspace config.\n", greenCheck, f.HostPath)

	default:
		return fmt.Errorf("unknown home config files target %d", target)
	}
	return nil
}

// findExistingLocations returns the config targets where the file's mount is
// already recorded.
func (r *homeConfigFilesAutoconfRunner) findExistingLocations(f DetectedHomeConfigFile) []HomeConfigFilesConfigTarget {
	var locations []HomeConfigFilesConfigTarget

	if r.projectLoader != nil {
		inGlobal := false
		if cfg, err := r.projectLoader.Load(""); err == nil && hasMountTarget(cfg, f.ContainerPath) {
			inGlobal = true
			locations = append(locations, HomeConfigFilesConfigTargetGlobal)
		}
		if r.projectID != "" && !inGlobal {
			// Load(projectID) merges global into the result, so only report the
			// project location when the mount is not already counted as global.
			if cfg, err := r.projectLoader.Load(r.projectID); err == nil && hasMountTarget(cfg, f.ContainerPath) {
				locations = append(locations, HomeConfigFilesConfigTargetProject)
			}
		}
	}

	if r.workspaceConfig != nil {
		if cfg, err := r.workspaceConfig.Load(); err == nil && hasMountTarget(cfg, f.ContainerPath) {
			locations = append(locations, HomeConfigFilesConfigTargetLocal)
		}
	}

	return locations
}

// hasMountTarget returns true if cfg contains a mount with the given target path.
func hasMountTarget(cfg *workspace.WorkspaceConfiguration, target string) bool {
	if cfg == nil || cfg.Mounts == nil {
		return false
	}
	for _, m := range *cfg.Mounts {
		if m.Target == target {
			return true
		}
	}
	return false
}

func formatHomeConfigLocations(locs []HomeConfigFilesConfigTarget) string {
	names := make([]string, 0, len(locs))
	for _, l := range locs {
		switch l {
		case HomeConfigFilesConfigTargetGlobal:
			names = append(names, "global")
		case HomeConfigFilesConfigTargetProject:
			names = append(names, "project")
		case HomeConfigFilesConfigTargetLocal:
			names = append(names, "local")
		}
	}
	return strings.Join(names, ", ")
}
