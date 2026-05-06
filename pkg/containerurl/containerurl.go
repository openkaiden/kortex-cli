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
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// URLRewriter rewrites a raw URL for container reachability.
type URLRewriter interface {
	Rewrite(raw string) string
}

// ReachabilityTarget pairs a hostname alias with its concrete IP for probing.
type ReachabilityTarget struct {
	Alias string
	IP    string
}

const probeTimeout = 300 * time.Millisecond

// defaultRewriter always rewrites localhost URLs to host.containers.internal.
type defaultRewriter struct{}

func (d *defaultRewriter) Rewrite(raw string) string { return RewriteURL(raw) }

// DefaultRewriter returns a URLRewriter that always rewrites to host.containers.internal.
func DefaultRewriter() URLRewriter { return &defaultRewriter{} }

// probingRewriter probes each target IP for the requested port and rewrites
// to the first reachable alias. Results are cached by original host:port.
type probingRewriter struct {
	targets []ReachabilityTarget
	cache   map[string]string // "host:port" -> resolved alias
}

// NewResolver returns a URLRewriter that probes each target's IP for the
// requested port and rewrites to the first reachable alias. If no target is
// reachable the URL is returned unchanged so the failure is observable.
func NewResolver(targets []ReachabilityTarget) URLRewriter {
	return &probingRewriter{
		targets: targets,
		cache:   make(map[string]string),
	}
}

func (r *probingRewriter) Rewrite(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	hostname := parsed.Hostname()
	if !isLocalhost(hostname) {
		return raw
	}

	port := parsed.Port()
	cacheKey := hostname + ":" + port

	if alias, ok := r.cache[cacheKey]; ok {
		return setHost(parsed, alias, port)
	}

	for _, t := range r.targets {
		if port != "" && isReachable(t.IP, port) {
			r.cache[cacheKey] = t.Alias
			return setHost(parsed, t.Alias, port)
		}
	}

	return raw
}

func setHost(u *url.URL, alias, port string) string {
	clone := *u
	if port != "" {
		clone.Host = alias + ":" + port
	} else {
		clone.Host = alias
	}
	return clone.String()
}

func isReachable(ip, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), probeTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ContainerHost is the hostname used to reach the podman VM from inside a container.
const ContainerHost = "host.containers.internal"

// NativeHost is the hostname injected into /etc/hosts to reach the native
// host machine (e.g. the Windows host on WSL2). On non-WSL2 platforms this
// is not injected — URLs fall back to ContainerHost.
const NativeHost = "native-host.internal"

// localhostAliases lists hostnames and IPs that refer to the local machine.
var localhostAliases = []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "[::1]"}

// isLocalhost reports whether host is a localhost-equivalent name or IP.
func isLocalhost(host string) bool {
	for _, alias := range localhostAliases {
		if strings.EqualFold(host, alias) {
			return true
		}
	}
	return false
}

// RewriteURL replaces localhost aliases in a URL with host.containers.internal
// so the URL is reachable from inside a container. If the input is not a valid
// URL or does not reference localhost, it is returned unchanged.
func RewriteURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if !isLocalhost(parsed.Hostname()) {
		return rawURL
	}

	return setHost(parsed, ContainerHost, parsed.Port())
}

// RewriteMCPCommandArgs rewrites localhost URLs in MCP command args so they
// are reachable from inside a container. Only command-based MCP servers are
// affected — these are spawned inside the container and may reference
// localhost to reach host services. URL-based MCP servers (remote endpoints)
// are not modified.
func RewriteMCPCommandArgs(mcp *workspace.McpConfiguration, rewriter URLRewriter) {
	if mcp == nil || mcp.Commands == nil {
		return
	}

	for i := range *mcp.Commands {
		cmd := &(*mcp.Commands)[i]
		if cmd.Args == nil {
			continue
		}
		for j, arg := range *cmd.Args {
			(*cmd.Args)[j] = rewriter.Rewrite(arg)
		}
	}
}

// localhostURLPattern matches http(s)://localhost-alias:port patterns in text.
var localhostURLPattern = regexp.MustCompile(
	`https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[?::1\]?)(?::\d+)?(?:/[^\s"',}\]]*)?`,
)

// RewriteSettings rewrites all localhost URLs found in agent settings files.
// This is a runtime concern — agents produce settings with localhost URLs and
// the runtime rewrites them based on network topology.
func RewriteSettings(settings map[string][]byte, rewriter URLRewriter) map[string][]byte {
	if settings == nil {
		return nil
	}
	result := make(map[string][]byte, len(settings))
	for path, data := range settings {
		result[path] = localhostURLPattern.ReplaceAllFunc(data, func(match []byte) []byte {
			return []byte(rewriter.Rewrite(string(match)))
		})
	}
	return result
}
