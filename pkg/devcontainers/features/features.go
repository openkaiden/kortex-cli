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

// Package features implements backend-agnostic modeling, downloading, and ordering
// of Dev Container Features as described in https://containers.dev/implementors/features/.
package features

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// FeatureOptions provides access to a feature's option specifications.
type FeatureOptions interface {
	// Merge merges user-supplied options with the spec defaults, validates
	// user values (type, enum membership), and normalizes keys per spec:
	// uppercased, non-alphanumeric chars replaced with '_'.
	// (e.g. "install-tools" → "INSTALL_TOOLS", "go.version" → "GO_VERSION")
	// Returns a map of normalized key → string value, or an error if any
	// user-supplied value is invalid (wrong type, not in enum, etc.).
	Merge(userOptions map[string]interface{}) (map[string]string, error)
}

// FeatureMetadata holds data parsed from devcontainer-feature.json.
type FeatureMetadata interface {
	// ContainerEnv returns persistent environment variables to set in the image
	// after the feature is installed. Values may use ${VAR} expansion.
	ContainerEnv() map[string]string
	// Options returns the feature's option specifications with merge/validation.
	Options() FeatureOptions
	// InstallsAfter returns the IDs of features that must be installed before this one.
	InstallsAfter() []string
}

// Feature models a devcontainer feature that can be resolved to a local directory.
type Feature interface {
	// ID returns the feature identifier as declared in the workspace config.
	ID() string
	// Download resolves the feature into destDir (fetching from a registry or
	// copying from the local file tree) and returns the parsed metadata.
	Download(ctx context.Context, destDir string) (FeatureMetadata, error)
}

// FromMap converts the workspace Features map (feature ID → user options) to:
//   - a slice of Feature instances sorted by ID (deterministic, pre-ordering)
//   - the user-supplied options map, keyed by feature ID
//
// workspaceConfigDir is the directory containing workspace.json; it is used to
// resolve relative paths for local features.
// Returns an error if any ID is an unsupported Tgz URI.
func FromMap(m map[string]map[string]interface{}, workspaceConfigDir string) ([]Feature, map[string]map[string]interface{}, error) {
	feats := make([]Feature, 0, len(m))
	userOpts := make(map[string]map[string]interface{}, len(m))

	for id, opts := range m {
		switch {
		case strings.HasPrefix(id, "./") || strings.HasPrefix(id, "../"):
			feats = append(feats, &localFeature{id: id, dir: workspaceConfigDir})
		case strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://"):
			return nil, nil, fmt.Errorf("direct HTTP(S) feature sources are not supported: %s", id)
		default:
			feats = append(feats, &ociFeature{id: id})
		}
		userOpts[id] = opts
	}

	sort.Slice(feats, func(i, j int) bool {
		return feats[i].ID() < feats[j].ID()
	})

	return feats, userOpts, nil
}

// featureBaseID returns the feature ID with its version tag or digest stripped.
// Per the Dev Container spec, installsAfter values are always versionless IDs,
// so this is used to match them against registered feature IDs that may carry a tag.
func featureBaseID(id string) string {
	if i := strings.Index(id, "@"); i >= 0 {
		return id[:i]
	}
	firstSlash := strings.Index(id, "/")
	if i := strings.LastIndex(id, ":"); i > firstSlash {
		return id[:i]
	}
	return id
}

// Order returns features in the correct installation order, respecting the
// installsAfter fields read from each feature's devcontainer-feature.json.
// metadata must contain an entry for every feature in feats; an error is
// returned if any entry is missing.
// Per the Dev Container spec, installsAfter is a hint: references to IDs not
// present in feats are silently ignored.
// Returns an error if a cycle is detected.
func Order(feats []Feature, metadata map[string]FeatureMetadata) ([]Feature, error) {
	for _, f := range feats {
		if _, ok := metadata[f.ID()]; !ok {
			return nil, fmt.Errorf("missing metadata for feature %q", f.ID())
		}
	}

	featMap := make(map[string]Feature, len(feats))
	// featByBase allows installsAfter values (always versionless per spec) to
	// resolve a feature registered with a version tag, e.g.
	// "…/common-utils" matches "…/common-utils:2".
	featByBase := make(map[string]Feature, len(feats))
	for _, f := range feats {
		featMap[f.ID()] = f
		featByBase[featureBaseID(f.ID())] = f
	}

	// Kahn's topological sort.
	// A.installsAfter = [B] means B must be installed before A: edge B → A.
	inDegree := make(map[string]int, len(feats))
	dependents := make(map[string][]string, len(feats))
	for _, f := range feats {
		id := f.ID()
		if _, exists := inDegree[id]; !exists {
			inDegree[id] = 0
		}
		for _, dep := range metadata[id].InstallsAfter() {
			resolved, ok := featByBase[dep]
			if !ok {
				// dep is not in our feature set; ignore it
				continue
			}
			dependents[resolved.ID()] = append(dependents[resolved.ID()], id)
			inDegree[id]++
		}
	}

	queue := make([]string, 0, len(feats))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	ordered := make([]Feature, 0, len(feats))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, featMap[id])

		deps := append([]string(nil), dependents[id]...)
		sort.Strings(deps)
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}

	if len(ordered) != len(feats) {
		return nil, fmt.Errorf("cycle detected in feature installsAfter dependencies")
	}

	return ordered, nil
}
