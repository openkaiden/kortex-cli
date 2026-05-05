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

// Package containerurl provides helpers for rewriting localhost URLs to
// host.containers.internal so they are reachable from inside a container.
package containerurl

import (
	"net/url"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// ContainerHost is the hostname used to reach the host machine from inside a container.
const ContainerHost = "host.containers.internal"

// localhostAliases lists hostnames and IPs that refer to the local machine.
var localhostAliases = []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "[::1]"}

// RewriteURL replaces localhost aliases in a URL with host.containers.internal
// so the URL is reachable from inside a container. If the input is not a valid
// URL or does not reference localhost, it is returned unchanged.
func RewriteURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	hostname := parsed.Hostname()
	for _, alias := range localhostAliases {
		if strings.EqualFold(hostname, alias) {
			port := parsed.Port()
			if port != "" {
				parsed.Host = ContainerHost + ":" + port
			} else {
				parsed.Host = ContainerHost
			}
			return parsed.String()
		}
	}

	return rawURL
}

// RewriteMCPCommandArgs rewrites localhost URLs in MCP command args so they
// are reachable from inside a container. Only command-based MCP servers are
// affected — these are spawned inside the container and may reference
// localhost to reach host services. URL-based MCP servers (remote endpoints)
// are not modified.
func RewriteMCPCommandArgs(mcp *workspace.McpConfiguration) {
	if mcp == nil || mcp.Commands == nil {
		return
	}

	for i := range *mcp.Commands {
		cmd := &(*mcp.Commands)[i]
		if cmd.Args == nil {
			continue
		}
		for j, arg := range *cmd.Args {
			(*cmd.Args)[j] = RewriteURL(arg)
		}
	}
}
