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

package openshell

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/secret"
)

const providerNamePrefix = "kdn-"

var providerTypeMap = map[string]string{
	"github":    "github",
	"anthropic": "anthropic",
}

var providerNameRegex = regexp.MustCompile("[^a-z0-9]+")

// providerName converts a kdn secret name into a deterministic OpenShell provider name.
func providerName(secretName string) string {
	s := strings.ToLower(secretName)
	s = providerNameRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return providerNamePrefix + s
}

// providerType maps a kdn secret type to an OpenShell provider type.
func providerType(kdnType string) string {
	if pt, ok := providerTypeMap[kdnType]; ok {
		return pt
	}
	return "generic"
}

// ensureProviders checks that OpenShell providers exist for all secrets
// configured in the workspace. Missing providers are created automatically.
// Returns the list of provider names that were created or already existed,
// so the caller can pass them to sandbox create via --provider flags.
func (r *openshellRuntime) ensureProviders(ctx context.Context, wsCfg *workspace.WorkspaceConfiguration) ([]string, error) {
	if wsCfg == nil || wsCfg.Secrets == nil || len(*wsCfg.Secrets) == 0 {
		return nil, nil
	}
	if r.secretStore == nil || r.secretServiceRegistry == nil {
		return nil, nil
	}

	existing, err := r.listExistingProviders(ctx)
	if err != nil {
		return nil, err
	}

	items, err := r.secretStore.List()
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}

	byName := make(map[string]secret.ListItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	l := logger.FromContext(ctx)

	var providerNames []string
	for _, name := range *wsCfg.Secrets {
		item, ok := byName[name]
		if !ok {
			continue
		}

		pName := providerName(name)

		if existing[pName] {
			providerNames = append(providerNames, pName)
			continue
		}

		_, value, err := r.secretStore.Get(name)
		if err != nil {
			return nil, fmt.Errorf("reading secret %q: %w", name, err)
		}

		var envVars []string
		if item.Type == secret.TypeOther {
			envVars = item.Envs
		} else {
			svc, svcErr := r.secretServiceRegistry.Get(item.Type)
			if svcErr != nil {
				continue
			}
			envVars = svc.EnvVars()
		}

		if len(envVars) == 0 {
			continue
		}

		pType := providerType(item.Type)

		args := []string{"provider", "create", "--name", pName, "--type", pType}
		for _, envVar := range envVars {
			args = append(args, "--credential", fmt.Sprintf("%s=%s", envVar, value))
		}

		fmt.Fprintf(l.Stderr(), "Creating provider %s (type=%s) for secret %q\n", pName, pType, name)
		if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(), args...); err != nil {
			return nil, fmt.Errorf("creating provider %s: %w", pName, err)
		}

		providerNames = append(providerNames, pName)
	}

	return providerNames, nil
}

// listExistingProviders returns the set of provider names currently registered
// in the OpenShell gateway.
func (r *openshellRuntime) listExistingProviders(ctx context.Context) (map[string]bool, error) {
	l := logger.FromContext(ctx)

	output, err := r.executor.Output(ctx, l.Stderr(), "provider", "list", "--names")
	if err != nil {
		return nil, fmt.Errorf("listing providers: %w", err)
	}

	providers := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			providers[name] = true
		}
	}
	return providers, nil
}
