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
	"fmt"
	"io"
	"path/filepath"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
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
	if err == nil {
		if wc, loadErr := wsCfgLoader.Load(); loadErr == nil {
			merged = wc
		}
	}

	projectLoader, err := config.NewProjectConfigLoader(storageDir)
	if err != nil {
		return merged, nil
	}
	if pc, loadErr := projectLoader.Load(projectID); loadErr == nil {
		merged = merger.Merge(merged, pc)
	}

	return merged, nil
}

// getAPIKeyFromPostgres queries the postgres container for the OneCLI API key
// by running a psql command inside the pod's postgres container.
func getAPIKeyFromPostgres(ctx context.Context, podName string, executor exec.Executor) (string, error) {
	output, err := executor.Output(ctx, io.Discard,
		"exec", podName+"-postgres",
		"psql", "-U", "onecli", "-d", "onecli", "-t", "-A",
		"-c", "select key from api_keys limit 1",
	)
	if err != nil {
		return "", fmt.Errorf("querying postgres for API key: %w", err)
	}
	key := strings.TrimSpace(string(output))
	if key == "" {
		return "", fmt.Errorf("no API key found in database")
	}
	return key, nil
}

// configureNetworking applies deny-mode network rules to the OneCLI gateway.
// It first deletes any existing rules (ensuring idempotency across restarts),
// then creates a rate_limit rule for each allowed host and a catch-all block rule.
func (p *podmanRuntime) configureNetworking(ctx context.Context, podName, onecliBaseURL string, hosts []string) error {
	apiKey, err := getAPIKeyFromPostgres(ctx, podName, p.executor)
	if err != nil {
		return err
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

	for _, host := range hosts {
		if _, createErr := client.CreateRule(ctx, onecli.CreateRuleInput{
			Name:            "allow-" + host,
			HostPattern:     host,
			Action:          "rate_limit",
			Enabled:         true,
			RateLimit:       65535,
			RateLimitWindow: "minute",
		}); createErr != nil {
			return fmt.Errorf("creating rule for %s: %w", host, createErr)
		}
	}

	if _, err := client.CreateRule(ctx, onecli.CreateRuleInput{
		Name:        "block-all",
		HostPattern: "=*",
		Action:      "block",
		Enabled:     true,
	}); err != nil {
		return fmt.Errorf("creating block-all rule: %w", err)
	}

	return nil
}
