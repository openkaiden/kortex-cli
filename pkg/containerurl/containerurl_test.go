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

package containerurl

import (
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestRewriteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"localhost", "http://localhost:11434/v1", "http://host.containers.internal:11434/v1"},
		{"127.0.0.1", "http://127.0.0.1:8080/v1", "http://host.containers.internal:8080/v1"},
		{"0.0.0.0", "http://0.0.0.0:11434/v1", "http://host.containers.internal:11434/v1"},
		{"::1", "http://[::1]:11434/v1", "http://host.containers.internal:11434/v1"},
		{"remote host unchanged", "http://192.168.1.50:11434/v1", "http://192.168.1.50:11434/v1"},
		{"hostname unchanged", "http://my-server:11434/v1", "http://my-server:11434/v1"},
		{"https preserved", "https://localhost:11434/v1", "https://host.containers.internal:11434/v1"},
		{"no port", "http://localhost/v1", "http://host.containers.internal/v1"},
		{"invalid URL returned as-is", "not a url ://", "not a url ://"},
		{"non-URL arg unchanged", "mcp-server-milvus==0.1.1.dev8", "mcp-server-milvus==0.1.1.dev8"},
		{"flag name unchanged", "--milvus-uri", "--milvus-uri"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RewriteURL(tt.input)
			if got != tt.expected {
				t.Errorf("RewriteURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRewriteMCPCommandArgs(t *testing.T) {
	t.Parallel()

	t.Run("nil mcp", func(t *testing.T) {
		t.Parallel()
		RewriteMCPCommandArgs(nil)
	})

	t.Run("nil commands", func(t *testing.T) {
		t.Parallel()
		mcp := &workspace.McpConfiguration{}
		RewriteMCPCommandArgs(mcp)
	})

	t.Run("nil args", func(t *testing.T) {
		t.Parallel()
		cmds := []workspace.McpCommand{{Name: "test", Command: "uvx"}}
		mcp := &workspace.McpConfiguration{Commands: &cmds}
		RewriteMCPCommandArgs(mcp)
		if (*mcp.Commands)[0].Args != nil {
			t.Error("expected args to remain nil")
		}
	})

	t.Run("rewrites localhost in args", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"mcp-server-milvus==0.1.1.dev8",
			"--milvus-uri",
			"http://localhost:51017",
		}
		cmds := []workspace.McpCommand{{
			Name:    "milvus",
			Command: "uvx",
			Args:    &args,
		}}
		mcp := &workspace.McpConfiguration{Commands: &cmds}

		RewriteMCPCommandArgs(mcp)

		got := (*mcp.Commands)[0].Args
		expected := []string{
			"mcp-server-milvus==0.1.1.dev8",
			"--milvus-uri",
			"http://host.containers.internal:51017",
		}
		for i, v := range *got {
			if v != expected[i] {
				t.Errorf("arg[%d] = %q, want %q", i, v, expected[i])
			}
		}
	})

	t.Run("does not modify server URLs", func(t *testing.T) {
		t.Parallel()
		servers := []workspace.McpServer{{
			Name: "github",
			Url:  "https://api.githubcopilot.com/mcp",
		}}
		mcp := &workspace.McpConfiguration{Servers: &servers}

		RewriteMCPCommandArgs(mcp)

		if (*mcp.Servers)[0].Url != "https://api.githubcopilot.com/mcp" {
			t.Errorf("server URL was modified: %q", (*mcp.Servers)[0].Url)
		}
	})

	t.Run("multiple commands", func(t *testing.T) {
		t.Parallel()
		args1 := []string{"--url", "http://127.0.0.1:8080"}
		args2 := []string{"--host", "http://localhost:3000/api"}
		cmds := []workspace.McpCommand{
			{Name: "svc1", Command: "npx", Args: &args1},
			{Name: "svc2", Command: "node", Args: &args2},
		}
		mcp := &workspace.McpConfiguration{Commands: &cmds}

		RewriteMCPCommandArgs(mcp)

		if (*(*mcp.Commands)[0].Args)[1] != "http://host.containers.internal:8080" {
			t.Errorf("cmd1 arg not rewritten: %q", (*(*mcp.Commands)[0].Args)[1])
		}
		if (*(*mcp.Commands)[1].Args)[1] != "http://host.containers.internal:3000/api" {
			t.Errorf("cmd2 arg not rewritten: %q", (*(*mcp.Commands)[1].Args)[1])
		}
	})
}
