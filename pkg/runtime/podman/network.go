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

package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/onecli"
)

// loadNetworkConfig reads the merged workspace configuration for a project by
// combining the workspace-level config (.kaiden/workspace.json) with the
// project-level config from projects.json. It mirrors the merge logic used
// at workspace creation time so that edits to projects.json are picked up on
// the next Start() without recreating the workspace.
func loadNetworkConfig(sourcePath, storageDir, projectID string) (*workspace.WorkspaceConfiguration, error) {
	merger := config.NewMerger()

	var merged *workspace.WorkspaceConfiguration

	wsCfgLoader, err := config.NewConfig(filepath.Join(sourcePath, ".kaiden"))
	if err != nil {
		return nil, fmt.Errorf("initializing workspace config loader: %w", err)
	}
	if wc, loadErr := wsCfgLoader.Load(); loadErr == nil {
		merged = wc
	}

	projectLoader, err := config.NewProjectConfigLoader(storageDir)
	if err != nil {
		return nil, fmt.Errorf("initializing project config loader: %w", err)
	}
	if pc, loadErr := projectLoader.Load(projectID); loadErr == nil {
		merged = merger.Merge(merged, pc)
	}

	return merged, nil
}

// approvalHandlerConfig is serialized to config.json in the approval-handler
// directory so the Node.js sidecar can connect to OneCLI and make decisions.
type approvalHandlerConfig struct {
	OnecliURL  string   `json:"onecliUrl"`
	GatewayURL string   `json:"gatewayUrl"`
	APIKey     string   `json:"apiKey"`
	Hosts      []string `json:"hosts"`
}

// clearNetworkingRules removes all existing networking rules from OneCLI.
// Called when switching to allow mode so that no leftover manual_approval or
// block rules from a previous deny-mode start remain active.
func (p *podmanRuntime) clearNetworkingRules(ctx context.Context, onecliBaseURL string) error {
	creds := onecli.NewCredentialProvider(onecliBaseURL)
	apiKey, err := creds.APIKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OneCLI API key: %w", err)
	}

	client := onecli.NewClient(onecliBaseURL, apiKey)

	rules, err := client.ListRules(ctx)
	if err != nil {
		return fmt.Errorf("listing existing rules: %w", err)
	}
	for _, r := range rules {
		if delErr := client.DeleteRule(ctx, r.ID); delErr != nil {
			return fmt.Errorf("deleting rule %s: %w", r.ID, delErr)
		}
	}
	return nil
}

// configureNetworking applies deny-mode networking via the OneCLI manual
// approval mechanism. It deletes any existing rules, creates a single
// manual_approval rule for all hosts, and writes config.json so the
// approval-handler sidecar knows which hosts to approve.
func (p *podmanRuntime) configureNetworking(ctx context.Context, onecliBaseURL string, hosts []string, approvalHandlerDir string) error {
	creds := onecli.NewCredentialProvider(onecliBaseURL)
	apiKey, err := creds.APIKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OneCLI API key: %w", err)
	}

	client := onecli.NewClient(onecliBaseURL, apiKey)

	rules, err := client.ListRules(ctx)
	if err != nil {
		return fmt.Errorf("listing existing rules: %w", err)
	}
	for _, r := range rules {
		if delErr := client.DeleteRule(ctx, r.ID); delErr != nil {
			return fmt.Errorf("deleting rule %s: %w", r.ID, delErr)
		}
	}

	if _, err := client.CreateRule(ctx, onecli.CreateRuleInput{
		Name:        "manual-approval-all",
		HostPattern: "*",
		Action:      "manual_approval",
		Enabled:     true,
	}); err != nil {
		return fmt.Errorf("creating manual_approval rule: %w", err)
	}

	// The sidecar runs inside the pod and shares the network namespace with
	// OneCLI, so it must use the internal container ports, not the host-mapped
	// ports that the Go CLI uses from outside the pod.
	// API (10254) is used for rule management; gateway (10255) is used for
	// manual approval long-polling.
	cfg := approvalHandlerConfig{
		OnecliURL:  "http://localhost:10254",
		GatewayURL: "http://localhost:10255",
		APIKey:     apiKey,
		Hosts:      hosts,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling approval handler config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(approvalHandlerDir, "config.json"), data, 0644); err != nil {
		return fmt.Errorf("writing approval handler config: %w", err)
	}

	return nil
}
