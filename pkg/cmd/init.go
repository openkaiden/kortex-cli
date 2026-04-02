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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	workspace "github.com/kortex-hub/kortex-cli-api/workspace-configuration/go"
	"github.com/kortex-hub/kortex-cli/pkg/config"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/kortex-hub/kortex-cli/pkg/logger"
	"github.com/kortex-hub/kortex-cli/pkg/runtimesetup"
	"github.com/kortex-hub/kortex-cli/pkg/steplogger"
	"github.com/spf13/cobra"
)

// initCmd contains the configuration for the init command
type initCmd struct {
	sourcesDir         string
	workspaceConfigDir string
	name               string
	runtime            string
	project            string
	agent              string
	absSourcesDir      string
	absConfigDir       string
	manager            instances.Manager
	workspaceConfig    *workspace.WorkspaceConfiguration
	verbose            bool
	output             string
	showLogs           bool
}

// preRun validates the parameters and flags
func (i *initCmd) preRun(cmd *cobra.Command, args []string) error {
	// Validate output format if specified
	if i.output != "" && i.output != "json" {
		return fmt.Errorf("unsupported output format: %s (supported: json)", i.output)
	}

	if i.showLogs && i.output == "json" {
		return fmt.Errorf("--show-logs cannot be used with --output json")
	}

	// Silence Cobra's default error output to stderr when JSON mode is enabled,
	// because we write the error in the JSON response to stdout instead
	if i.output == "json" {
		cmd.SilenceErrors = true
	}

	// Get storage directory from global flag
	storageDir, err := cmd.Flags().GetString("storage")
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to read --storage flag: %w", err))
	}

	// Convert to absolute path
	absStorageDir, err := filepath.Abs(storageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve storage directory path: %w", err))
	}

	// Create manager
	manager, err := instances.NewManager(absStorageDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to create manager: %w", err))
	}

	// Register all available runtimes
	if err := runtimesetup.RegisterAll(manager); err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to register runtimes: %w", err))
	}

	i.manager = manager

	// Determine runtime: flag takes precedence over environment variable
	if i.runtime == "" {
		// Check environment variable
		if envRuntime := os.Getenv("KORTEX_CLI_DEFAULT_RUNTIME"); envRuntime != "" {
			i.runtime = envRuntime
		} else {
			// Neither flag nor environment variable is set
			return outputErrorIfJSON(cmd, i.output, fmt.Errorf("runtime is required: use --runtime flag or set KORTEX_CLI_DEFAULT_RUNTIME environment variable"))
		}
	}

	// Determine agent: flag takes precedence over environment variable
	if i.agent == "" {
		// Check environment variable
		if envAgent := os.Getenv("KORTEX_CLI_DEFAULT_AGENT"); envAgent != "" {
			i.agent = envAgent
		} else {
			// Neither flag nor environment variable is set
			return outputErrorIfJSON(cmd, i.output, fmt.Errorf("agent is required: use --agent flag or set KORTEX_CLI_DEFAULT_AGENT environment variable"))
		}
	}

	// Get sources directory (default to current directory)
	i.sourcesDir = "."
	if len(args) > 0 {
		i.sourcesDir = args[0]
	}

	// Convert to absolute path for clarity
	absSourcesDir, err := filepath.Abs(i.sourcesDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve sources directory path: %w", err))
	}
	i.absSourcesDir = absSourcesDir

	// Verify that the sources directory exists and is a directory
	fileInfo, err := os.Stat(i.absSourcesDir)
	if os.IsNotExist(err) {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("sources directory does not exist: %s", i.absSourcesDir))
	} else if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to check sources directory: %w", err))
	}
	if !fileInfo.IsDir() {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("sources path is not a directory: %s", i.absSourcesDir))
	}

	// If workspace-configuration flag was not explicitly set, default to .kortex/ inside sources directory
	if !cmd.Flags().Changed("workspace-configuration") {
		i.workspaceConfigDir = filepath.Join(i.sourcesDir, ".kortex")
	}

	// Convert workspace config to absolute path
	absConfigDir, err := filepath.Abs(i.workspaceConfigDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to resolve workspace configuration directory path: %w", err))
	}
	i.absConfigDir = absConfigDir

	// Load and validate workspace configuration if it exists
	cfg, err := config.NewConfig(i.absConfigDir)
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to create config manager: %w", err))
	}

	workspaceConfig, err := cfg.Load()
	if err != nil {
		// ErrConfigNotFound is acceptable - configuration is optional
		if !errors.Is(err, config.ErrConfigNotFound) {
			return outputErrorIfJSON(cmd, i.output, fmt.Errorf("workspace configuration validation failed: %w", err))
		}
	}
	i.workspaceConfig = workspaceConfig

	return nil
}

// run executes the init command logic
func (i *initCmd) run(cmd *cobra.Command, args []string) error {
	// Create appropriate step logger based on output mode
	var stepLogger steplogger.StepLogger
	if i.output == "json" {
		// No step logging in JSON mode
		stepLogger = steplogger.NewNoOpLogger()
	} else {
		stepLogger = steplogger.NewTextLogger(cmd.ErrOrStderr())
	}
	defer stepLogger.Complete()

	ctx := steplogger.WithLogger(cmd.Context(), stepLogger)

	// Create appropriate logger based on --show-logs flag
	var l logger.Logger
	if i.showLogs {
		l = logger.NewTextLogger(cmd.OutOrStdout(), cmd.ErrOrStderr())
	} else {
		l = logger.NewNoOpLogger()
	}
	ctx = logger.WithLogger(ctx, l)

	// Create a new instance
	instance, err := instances.NewInstance(instances.NewInstanceParams{
		SourceDir: i.absSourcesDir,
		ConfigDir: i.absConfigDir,
		Name:      i.name,
	})
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, err)
	}

	// Add the instance to the manager
	// Manager handles config loading and merging internally
	addedInstance, err := i.manager.Add(ctx, instances.AddOptions{
		Instance:        instance,
		RuntimeType:     i.runtime,
		WorkspaceConfig: i.workspaceConfig,
		Project:         i.project,
		Agent:           i.agent,
	})
	if err != nil {
		return outputErrorIfJSON(cmd, i.output, err)
	}

	// Handle JSON output
	if i.output == "json" {
		return i.outputJSON(cmd, addedInstance)
	}

	// Handle text output
	out := cmd.OutOrStdout()
	if i.verbose {
		fmt.Fprintf(out, "Registered workspace:\n")
		fmt.Fprintf(out, "  ID: %s\n", addedInstance.GetID())
		fmt.Fprintf(out, "  Name: %s\n", addedInstance.GetName())
		fmt.Fprintf(out, "  Project: %s\n", addedInstance.GetProject())
		fmt.Fprintf(out, "  Agent: %s\n", addedInstance.GetAgent())
		fmt.Fprintf(out, "  Sources directory: %s\n", addedInstance.GetSourceDir())
		fmt.Fprintf(out, "  Configuration directory: %s\n", addedInstance.GetConfigDir())
		fmt.Fprintf(out, "  State: %s\n", addedInstance.GetRuntimeData().State)
	} else {
		fmt.Fprintln(out, addedInstance.GetID())
	}

	return nil
}

// outputJSON outputs the instance as JSON based on verbose flag
func (i *initCmd) outputJSON(cmd *cobra.Command, instance instances.Instance) error {
	var jsonData []byte
	var err error

	if i.verbose {
		// Verbose mode: return full Workspace
		workspace := instanceToWorkspace(instance)
		jsonData, err = json.MarshalIndent(workspace, "", "  ")
	} else {
		// Default mode: return only WorkspaceId
		workspaceId := instanceToWorkspaceId(instance)
		jsonData, err = json.MarshalIndent(workspaceId, "", "  ")
	}

	if err != nil {
		return outputErrorIfJSON(cmd, i.output, fmt.Errorf("failed to marshal to JSON: %w", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
	return nil
}

func NewInitCmd() *cobra.Command {
	c := &initCmd{}

	cmd := &cobra.Command{
		Use:   "init [sources-directory]",
		Short: "Register a new workspace",
		Long: `Register a new workspace with the specified sources directory and configuration location.

The sources directory defaults to the current directory (.) if not specified.
The workspace configuration directory defaults to .kortex/ inside the sources directory if not specified.`,
		Example: `# Register current directory as workspace
kortex-cli init --runtime fake --agent claude

# Register specific directory as workspace
kortex-cli init --runtime fake --agent claude /path/to/project

# Register with custom workspace name
kortex-cli init --runtime fake --agent claude --name my-project

# Register with custom project identifier
kortex-cli init --runtime fake --agent goose --project my-custom-project

# Show detailed output
kortex-cli init --runtime fake --agent claude --verbose

# Show runtime command output
kortex-cli init --runtime fake --agent claude --show-logs`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: c.preRun,
		RunE:    c.run,
	}

	// Add workspace-configuration flag
	cmd.Flags().StringVar(&c.workspaceConfigDir, "workspace-configuration", "", "Directory for workspace configuration (default: <sources-directory>/.kortex)")

	// Add name flag
	cmd.Flags().StringVarP(&c.name, "name", "n", "", "Name for the workspace (default: generated from sources directory)")

	// Add runtime flag
	cmd.Flags().StringVarP(&c.runtime, "runtime", "r", "", "Runtime to use for the workspace (required if KORTEX_CLI_DEFAULT_RUNTIME is not set)")
	cmd.RegisterFlagCompletionFunc("runtime", completeRuntimeFlag)

	// Add project flag
	cmd.Flags().StringVarP(&c.project, "project", "p", "", "Custom project identifier (default: auto-detected from git repository or source directory)")

	// Add agent flag
	cmd.Flags().StringVarP(&c.agent, "agent", "a", "", "Agent name for loading agent-specific configuration (required if KORTEX_CLI_DEFAULT_AGENT is not set)")

	// Add verbose flag
	cmd.Flags().BoolVarP(&c.verbose, "verbose", "v", false, "Show detailed output")

	// Add output flag
	cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
	cmd.Flags().BoolVar(&c.showLogs, "show-logs", false, "Show stdout and stderr from runtime commands")

	cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))

	return cmd
}
