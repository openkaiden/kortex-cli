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
	"os"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
)

// podmanContainerHome is the home directory used by credential implementations.
const podmanContainerHome = "/home/agent"

// hostExpanders holds functions that transform host patterns before they are
// added to the network allow list. Credential-specific files (e.g.
// credentials_gcloud.go) register their expanders via init().
var hostExpanders []func([]string) []string

// registerHostExpander adds a host-pattern expander. Called from init() in
// credential-specific files.
func registerHostExpander(fn func([]string) []string) {
	hostExpanders = append(hostExpanders, fn)
}

// expandHostPatterns runs all registered host expanders on the given patterns.
func expandHostPatterns(hosts []string) []string {
	for _, fn := range hostExpanders {
		hosts = fn(hosts)
	}
	return hosts
}

// credentialUpload captures a detected credential's upload parameters.
type credentialUpload struct {
	hostPath      string
	containerPath string
}

// interceptCredentials detects file-based credential mounts in the workspace
// config and returns a function that uploads the real credential files into the
// sandbox after it is created. OpenShell sandboxes are isolated so real files
// can be uploaded directly.
//
// Intercepted mounts are removed from the workspace config and each
// credential's host patterns are added to the network allow list.
func (r *openshellRuntime) interceptCredentials(ctx context.Context, params runtime.CreateParams) (func(context.Context, string) error, error) {
	wsCfg := params.WorkspaceConfig
	if r.credentialRegistry == nil || wsCfg == nil || wsCfg.Mounts == nil || len(*wsCfg.Mounts) == 0 {
		return nil, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}

	l := logger.FromContext(ctx)

	var uploads []credentialUpload

	for _, cred := range r.credentialRegistry.List() {
		hostPath, intercepted := cred.Detect(*wsCfg.Mounts, homeDir)
		if intercepted == nil {
			continue
		}

		if _, err := os.Stat(hostPath); os.IsNotExist(err) {
			fmt.Fprintf(l.Stderr(), "%s credential file not found at %s, skipping\n", cred.Name(), hostPath)
			continue
		}

		removeMountFromConfig(wsCfg, intercepted)
		addHostsToNetworkConfig(wsCfg, expandHostPatterns(cred.HostPatterns(hostPath)))

		uploads = append(uploads, credentialUpload{
			hostPath:      hostPath,
			containerPath: remapContainerPath(cred.ContainerFilePath()),
		})
	}

	if len(uploads) == 0 {
		return nil, nil
	}

	uploadFn := func(ctx context.Context, sandboxName string) error {
		l := logger.FromContext(ctx)
		for _, u := range uploads {
			if err := r.executor.Run(ctx, l.Stdout(), l.Stderr(),
				"sandbox", "upload", sandboxName, u.hostPath, u.containerPath,
			); err != nil {
				return err
			}
		}
		return nil
	}

	return uploadFn, nil
}

// remapContainerPath translates a container file path from the Podman home
// directory (/home/agent) to the OpenShell sandbox home (/sandbox).
func remapContainerPath(containerFilePath string) string {
	if strings.HasPrefix(containerFilePath, podmanContainerHome) {
		return containerHome + containerFilePath[len(podmanContainerHome):]
	}
	return containerFilePath
}

// removeMountFromConfig removes the intercepted mount from the workspace config.
func removeMountFromConfig(wsCfg *workspace.WorkspaceConfiguration, intercepted *workspace.Mount) {
	if wsCfg.Mounts == nil {
		return
	}
	filtered := make([]workspace.Mount, 0, len(*wsCfg.Mounts))
	for i := range *wsCfg.Mounts {
		if &(*wsCfg.Mounts)[i] != intercepted {
			filtered = append(filtered, (*wsCfg.Mounts)[i])
		}
	}
	*wsCfg.Mounts = filtered
}

// addHostsToNetworkConfig appends hosts to the workspace network allow list.
func addHostsToNetworkConfig(wsCfg *workspace.WorkspaceConfiguration, hosts []string) {
	if len(hosts) == 0 {
		return
	}
	if wsCfg.Network == nil {
		wsCfg.Network = &workspace.NetworkConfiguration{}
	}
	if wsCfg.Network.Hosts == nil {
		hostsList := make([]string, 0, len(hosts))
		wsCfg.Network.Hosts = &hostsList
	}

	seen := make(map[string]bool, len(*wsCfg.Network.Hosts))
	for _, h := range *wsCfg.Network.Hosts {
		seen[strings.ToLower(h)] = true
	}
	for _, h := range hosts {
		if !seen[strings.ToLower(h)] {
			*wsCfg.Network.Hosts = append(*wsCfg.Network.Hosts, h)
		}
	}
}
