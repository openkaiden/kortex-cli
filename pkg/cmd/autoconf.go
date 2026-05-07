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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/openkaiden/kdn/pkg/autoconf"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/git"
	"github.com/openkaiden/kdn/pkg/project"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservicesetup"
	"github.com/spf13/cobra"
)

type autoconfCmd struct {
	yes              bool
	store            secret.Store
	projectUpdater   config.ProjectConfigUpdater
	workspaceUpdater config.WorkspaceConfigUpdater
	projectID        string
	projectDetector  project.Detector
	detector         autoconf.SecretDetector
	confirm          func(prompt string) (bool, error)
	selectTarget     func(secretName string, options []autoconf.ConfigTargetOption) (autoconf.ConfigTarget, error)

	vertexDetector     autoconf.VertexDetector
	agentUpdater       config.AgentConfigUpdater
	agentLoader        config.AgentConfigLoader
	workspaceCfg       config.Config
	vertexSelectTarget func(options []autoconf.ClaudeVertexConfigTargetOption) (autoconf.ClaudeVertexConfigTarget, error)

	homeConfigFilesDetector     autoconf.HomeConfigFilesDetector
	projectLoader               config.ProjectConfigLoader
	homeConfigFilesSelectTarget func(options []autoconf.HomeConfigFilesConfigTargetOption) (autoconf.HomeConfigFilesConfigTarget, error)
}

func (a *autoconfCmd) preRun(cmd *cobra.Command, args []string) error {
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return fmt.Errorf("failed to read --storage flag: %w", err)
	}

	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return fmt.Errorf("failed to resolve storage directory: %w", err)
	}

	a.store = secret.NewStore(absStorageDir)
	services := secretservicesetup.ListServices()

	projectUpdater, err := config.NewProjectConfigUpdater(absStorageDir)
	if err != nil {
		return fmt.Errorf("failed to create project config updater: %w", err)
	}
	a.projectUpdater = projectUpdater

	loader, err := config.NewProjectConfigLoader(absStorageDir)
	if err != nil {
		return fmt.Errorf("failed to create project config loader: %w", err)
	}
	a.projectLoader = loader

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if a.projectDetector == nil {
		a.projectDetector = project.NewDetector(git.NewDetector())
	}
	a.projectID = a.projectDetector.DetectProject(cmd.Context(), cwd)

	kaidenDir := filepath.Join(cwd, ".kaiden")

	// workspaceConfig is always wired (Load returns ErrConfigNotFound when the
	// file is absent, which the filter handles gracefully).
	workspaceCfg, wcErr := config.NewConfig(kaidenDir)
	if wcErr != nil {
		return fmt.Errorf("failed to create workspace config: %w", wcErr)
	}
	a.workspaceCfg = workspaceCfg

	if a.detector == nil {
		a.detector = autoconf.NewFilteredSecretDetector(services, a.store, loader, a.projectID, workspaceCfg)
	}

	if a.vertexDetector == nil {
		vd, vdErr := autoconf.NewVertexDetector()
		if vdErr != nil {
			return fmt.Errorf("failed to create vertex detector: %w", vdErr)
		}
		a.vertexDetector = vd
	}

	agentUpdater, auErr := config.NewAgentConfigUpdater(absStorageDir)
	if auErr != nil {
		return fmt.Errorf("failed to create agent config updater: %w", auErr)
	}
	a.agentUpdater = agentUpdater

	agentLoader, alErr := config.NewAgentConfigLoader(absStorageDir)
	if alErr != nil {
		return fmt.Errorf("failed to create agent config loader: %w", alErr)
	}
	a.agentLoader = agentLoader

	wu, wuErr := config.NewWorkspaceConfigUpdater(kaidenDir)
	if wuErr != nil {
		return fmt.Errorf("failed to create workspace config updater: %w", wuErr)
	}
	a.workspaceUpdater = wu

	if a.homeConfigFilesDetector == nil {
		hd, hdErr := autoconf.NewHomeConfigFilesDetector()
		if hdErr != nil {
			return fmt.Errorf("failed to create home config files detector: %w", hdErr)
		}
		a.homeConfigFilesDetector = hd
	}

	if a.confirm == nil {
		a.confirm = huhConfirm
	}
	if a.selectTarget == nil {
		a.selectTarget = huhSelectTarget
	}
	if a.vertexSelectTarget == nil {
		a.vertexSelectTarget = huhSelectVertexTarget
	}
	if a.homeConfigFilesSelectTarget == nil {
		a.homeConfigFilesSelectTarget = huhSelectHomeConfigFilesTarget
	}

	return nil
}

func huhConfirm(prompt string) (bool, error) {
	var ok bool
	err := huh.NewConfirm().
		Title(prompt).
		Affirmative("Yes").
		Negative("No").
		Value(&ok).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return false, nil
	}
	return ok, err
}

func huhSelectVertexTarget(options []autoconf.ClaudeVertexConfigTargetOption) (autoconf.ClaudeVertexConfigTarget, error) {
	huhOptions := make([]huh.Option[autoconf.ClaudeVertexConfigTarget], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt.Label, opt.Target)
	}

	var selected autoconf.ClaudeVertexConfigTarget
	err := huh.NewSelect[autoconf.ClaudeVertexConfigTarget]().
		Title("Add Vertex AI configuration to:").
		Options(huhOptions...).
		Value(&selected).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return 0, autoconf.ErrSkipped
	}
	return selected, err
}

func huhSelectHomeConfigFilesTarget(options []autoconf.HomeConfigFilesConfigTargetOption) (autoconf.HomeConfigFilesConfigTarget, error) {
	huhOptions := make([]huh.Option[autoconf.HomeConfigFilesConfigTarget], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt.Label, opt.Target)
	}

	var selected autoconf.HomeConfigFilesConfigTarget
	err := huh.NewSelect[autoconf.HomeConfigFilesConfigTarget]().
		Title("Add home config file mount to:").
		Options(huhOptions...).
		Value(&selected).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return 0, autoconf.ErrSkipped
	}
	return selected, err
}

func huhSelectTarget(secretName string, options []autoconf.ConfigTargetOption) (autoconf.ConfigTarget, error) {
	huhOptions := make([]huh.Option[autoconf.ConfigTarget], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt.Label, opt.Target)
	}

	var selected autoconf.ConfigTarget
	err := huh.NewSelect[autoconf.ConfigTarget]().
		Title(fmt.Sprintf("Add secret %q to:", secretName)).
		Options(huhOptions...).
		Value(&selected).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return 0, autoconf.ErrSkipped
	}
	return selected, err
}

func (a *autoconfCmd) run(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	runner := autoconf.New(autoconf.Options{
		Detector:         a.detector,
		Store:            a.store,
		ProjectUpdater:   a.projectUpdater,
		WorkspaceUpdater: a.workspaceUpdater,
		ProjectID:        a.projectID,
		Yes:              a.yes,
		Confirm:          a.confirm,
		SelectTarget:     a.selectTarget,
	})
	if err := runner.Run(out); err != nil {
		return err
	}

	if a.vertexDetector != nil {
		vertexRunner := autoconf.NewClaudeVertexAutoconf(autoconf.ClaudeVertexAutoconfOptions{
			Detector:         a.vertexDetector,
			AgentUpdater:     a.agentUpdater,
			WorkspaceUpdater: a.workspaceUpdater,
			AgentLoader:      a.agentLoader,
			WorkspaceConfig:  a.workspaceCfg,
			Yes:              a.yes,
			Confirm:          a.confirm,
			SelectTarget:     a.vertexSelectTarget,
		})
		if err := vertexRunner.Run(out); err != nil {
			return err
		}
	}

	if a.homeConfigFilesDetector == nil {
		return nil
	}
	homeConfigRunner := autoconf.NewHomeConfigFilesAutoconf(autoconf.HomeConfigFilesAutoconfOptions{
		Detector:         a.homeConfigFilesDetector,
		ProjectUpdater:   a.projectUpdater,
		WorkspaceUpdater: a.workspaceUpdater,
		ProjectLoader:    a.projectLoader,
		WorkspaceConfig:  a.workspaceCfg,
		ProjectID:        a.projectID,
		Yes:              a.yes,
		Confirm:          a.confirm,
		SelectTarget:     a.homeConfigFilesSelectTarget,
	})
	return homeConfigRunner.Run(out)
}

// NewAutoconfCmd returns the autoconf command.
func NewAutoconfCmd() *cobra.Command {
	c := &autoconfCmd{}

	cmd := &cobra.Command{
		Use:   "autoconf",
		Short: "Automatically configure workspace settings from the environment",
		Long: `Detect environment variables and files to auto-configure workspace settings.

Scans registered secret services and creates secrets for any service whose
environment variables are set. Secrets are stored in the local secret store
and added to the chosen configuration target (global, project-specific, or local).`,
		Example: `# Detect and apply secrets from the environment
kdn autoconf

# Apply without confirmation prompt
kdn autoconf --yes

# Use a custom storage directory
kdn autoconf --storage /custom/path

# Pass secrets inline and apply immediately
GH_TOKEN="$(gh auth token)" kdn autoconf --yes`,
		Args:    cobra.NoArgs,
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	cmd.Flags().BoolVarP(&c.yes, "yes", "y", false, "Apply changes without confirmation prompt")

	return cmd
}
