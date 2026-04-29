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

	"github.com/fatih/color"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/secret"
)

// ConfigTarget identifies where a detected secret's reference is recorded.
type ConfigTarget int

const (
	// ConfigTargetGlobal records the secret in the global entry of projects.json
	// (applies to all projects).
	ConfigTargetGlobal ConfigTarget = iota
	// ConfigTargetProject records the secret in the project-specific entry of
	// projects.json (keyed by the computed project ID for the current directory).
	ConfigTargetProject
	// ConfigTargetLocal records the secret in the local .kaiden/workspace.json.
	ConfigTargetLocal
)

// ConfigTargetOption pairs a ConfigTarget with a human-readable label for display
// in selection prompts.
type ConfigTargetOption struct {
	Target ConfigTarget
	Label  string
}

// ErrSkipped may be returned by Options.SelectTarget to signal that the user
// chose not to add the secret to any configuration target.
var ErrSkipped = errors.New("skipped")

// Options configures an Autoconf runner.
type Options struct {
	Detector SecretDetector
	Store    secret.Store

	// ProjectUpdater writes to ~/.kdn/config/projects.json.
	ProjectUpdater config.ProjectConfigUpdater
	// WorkspaceUpdater writes to .kaiden/workspace.json in the current directory.
	// When nil the local target is not offered to the user. When non-nil the file
	// is created automatically if the user selects the local target.
	WorkspaceUpdater config.WorkspaceConfigUpdater
	// ProjectID is the project identifier for the current working directory,
	// used when the user selects ConfigTargetProject.
	ProjectID string

	Yes bool

	// Confirm is called (once per secret) to ask whether to create it.
	// Returning false skips the secret entirely.
	Confirm func(prompt string) (bool, error)

	// SelectTarget is called after a secret is created to ask where to record
	// the reference. It may return ErrSkipped to skip adding to any config.
	// Not called when Yes is true (defaults to ConfigTargetGlobal).
	SelectTarget func(secretName string, options []ConfigTargetOption) (ConfigTarget, error)
}

// Autoconf orchestrates secret detection and application: it scans the
// environment, prompts for confirmation, creates secrets, and records their
// references in the chosen configuration target.
type Autoconf interface {
	Run(out io.Writer) error
}

type autoconfRunner struct {
	detector         SecretDetector
	store            secret.Store
	projectUpdater   config.ProjectConfigUpdater
	workspaceUpdater config.WorkspaceConfigUpdater
	projectID        string
	yes              bool
	confirm          func(string) (bool, error)
	selectTarget     func(string, []ConfigTargetOption) (ConfigTarget, error)
}

var _ Autoconf = (*autoconfRunner)(nil)

var greenCheck = color.New(color.FgGreen).Sprint("✓")
var greyDash = color.New(color.FgHiBlack).Sprint("–")

// New returns an Autoconf configured by opts.
func New(opts Options) Autoconf {
	return &autoconfRunner{
		detector:         opts.Detector,
		store:            opts.Store,
		projectUpdater:   opts.ProjectUpdater,
		workspaceUpdater: opts.WorkspaceUpdater,
		projectID:        opts.ProjectID,
		yes:              opts.Yes,
		confirm:          opts.Confirm,
		selectTarget:     opts.SelectTarget,
	}
}

func (a *autoconfRunner) Run(out io.Writer) error {
	result, err := a.detector.Detect()
	if err != nil {
		return err
	}

	for _, c := range result.Configured {
		fmt.Fprintf(out, "%s Secret %q already configured (%s).\n", greenCheck, c.ServiceName, formatLocations(c.Locations))
	}

	if len(result.NeedsAction) == 0 {
		if len(result.Configured) == 0 {
			fmt.Fprintln(out, "No secrets detected in environment.")
		}
		return nil
	}

	for _, d := range result.NeedsAction {
		if err := a.processSecret(out, d); err != nil {
			return err
		}
	}
	return nil
}

func formatLocations(locs []ConfigTarget) string {
	names := make([]string, 0, len(locs))
	for _, l := range locs {
		switch l {
		case ConfigTargetGlobal:
			names = append(names, "global")
		case ConfigTargetProject:
			names = append(names, "project")
		case ConfigTargetLocal:
			names = append(names, "local")
		}
	}
	return strings.Join(names, ", ")
}

func (a *autoconfRunner) processSecret(out io.Writer, d DetectedSecret) error {
	fmt.Fprintf(out, "Detected secret %q from %s\n", d.ServiceName, d.EnvVarName)

	// Step 1: confirm creation.
	if !a.yes {
		ok, err := a.confirm(fmt.Sprintf("Create secret %q?", d.ServiceName))
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !ok {
			fmt.Fprintf(out, "%s Skipped %q.\n", greyDash, d.ServiceName)
			return nil
		}
	}

	if err := a.createSecret(out, d); err != nil {
		return err
	}

	// Step 2: select config target.
	return a.addToConfig(out, d.ServiceName)
}

func (a *autoconfRunner) createSecret(out io.Writer, d DetectedSecret) error {
	_, _, err := a.store.Get(d.ServiceName)
	if err == nil {
		fmt.Fprintf(out, "Secret %q already exists, skipping creation.\n", d.ServiceName)
		return nil
	}
	if !errors.Is(err, secret.ErrSecretNotFound) {
		return fmt.Errorf("failed to check secret %q: %w", d.ServiceName, err)
	}

	if err := a.store.Create(secret.CreateParams{
		Name:  d.ServiceName,
		Type:  d.ServiceName,
		Value: d.Value,
	}); err != nil {
		return fmt.Errorf("failed to create secret %q: %w", d.ServiceName, err)
	}
	fmt.Fprintf(out, "%s Created secret %q.\n", greenCheck, d.ServiceName)
	return nil
}

func (a *autoconfRunner) addToConfig(out io.Writer, secretName string) error {
	var target ConfigTarget

	if a.yes {
		target = ConfigTargetGlobal
	} else {
		var err error
		target, err = a.selectTarget(secretName, a.buildTargetOptions())
		if errors.Is(err, ErrSkipped) {
			fmt.Fprintf(out, "%s Skipped adding %q to configuration.\n", greyDash, secretName)
			return nil
		}
		if err != nil {
			return fmt.Errorf("target selection failed: %w", err)
		}
	}

	return a.applyTarget(out, secretName, target)
}

func (a *autoconfRunner) buildTargetOptions() []ConfigTargetOption {
	opts := []ConfigTargetOption{
		{Target: ConfigTargetGlobal, Label: "Global (all projects)"},
		{Target: ConfigTargetProject, Label: fmt.Sprintf("Project (%s)", a.projectID)},
	}
	if a.workspaceUpdater != nil {
		opts = append(opts, ConfigTargetOption{
			Target: ConfigTargetLocal,
			Label:  "Local (.kaiden/workspace.json)",
		})
	}
	return opts
}

func (a *autoconfRunner) applyTarget(out io.Writer, secretName string, target ConfigTarget) error {
	switch target {
	case ConfigTargetGlobal:
		if err := a.projectUpdater.AddSecret("", secretName); err != nil {
			return fmt.Errorf("failed to update global config for secret %q: %w", secretName, err)
		}
		fmt.Fprintf(out, "%s Added secret %q to global project config.\n", greenCheck, secretName)

	case ConfigTargetProject:
		if err := a.projectUpdater.AddSecret(a.projectID, secretName); err != nil {
			return fmt.Errorf("failed to update project config for secret %q: %w", secretName, err)
		}
		fmt.Fprintf(out, "%s Added secret %q to project config.\n", greenCheck, secretName)

	case ConfigTargetLocal:
		if err := a.workspaceUpdater.AddSecret(secretName); err != nil {
			return fmt.Errorf("failed to update workspace config for secret %q: %w", secretName, err)
		}
		fmt.Fprintf(out, "%s Added secret %q to local workspace config.\n", greenCheck, secretName)
	}
	return nil
}
