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

package agent

import (
	"encoding/json"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

func TestClaw_Name(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	if got := agent.Name(); got != "openclaw" {
		t.Errorf("Name() = %q, want %q", got, "openclaw")
	}
}

func TestClaw_SkipOnboarding_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	settings := make(map[string]SettingsFile)

	result, err := agent.SkipOnboarding(settings, "/workspace/sources", nil)
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	sf, exists := result[ClawConfigPath]
	if !exists {
		t.Fatalf("Expected %s to be created", ClawConfigPath)
	}
	clawJSON := sf.Content

	var config map[string]interface{}
	if err := json.Unmarshal(clawJSON, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	gateway, ok := config["gateway"].(map[string]interface{})
	if !ok {
		t.Fatalf("gateway is not a map: %v", config["gateway"])
	}

	auth, ok := gateway["auth"].(map[string]interface{})
	if !ok {
		t.Fatalf("gateway.auth is not a map: %v", gateway["auth"])
	}
	if auth["mode"] != "token" {
		t.Errorf("gateway.auth.mode = %v, want %q", auth["mode"], "token")
	}
	if auth["token"] != "openclaw123" {
		t.Errorf("gateway.auth.token = %v, want %q", auth["token"], "openclaw123")
	}

	controlUi, ok := gateway["controlUi"].(map[string]interface{})
	if !ok {
		t.Fatalf("gateway.controlUi is not a map: %v", gateway["controlUi"])
	}
	if enabled, ok := controlUi["enabled"].(bool); !ok || !enabled {
		t.Errorf("gateway.controlUi.enabled = %v, want true", controlUi["enabled"])
	}

	if gateway["bind"] != "lan" {
		t.Errorf("gateway.bind = %v, want %q", gateway["bind"], "lan")
	}
}

func TestClaw_SkipOnboarding_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	result, err := agent.SkipOnboarding(nil, "/workspace/sources", nil)
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[ClawConfigPath]; !exists {
		t.Errorf("Expected %s to be created", ClawConfigPath)
	}
}

func TestClaw_SkipOnboarding_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	existingSettings := map[string]interface{}{
		"customField": "custom value",
		"gateway": map[string]interface{}{
			"port": 9999,
			"auth": map[string]interface{}{
				"mode":        "token",
				"otherOption": "preserved",
			},
			"controlUi": map[string]interface{}{
				"enabled":  false,
				"basePath": "/custom",
			},
		},
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string]SettingsFile{
		ClawConfigPath: {Content: existingJSON},
	}

	result, err := agent.SkipOnboarding(settings, "/workspace/sources", nil)
	if err != nil {
		t.Fatalf("SkipOnboarding() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if config["customField"] != "custom value" {
		t.Errorf("customField = %v, want %q", config["customField"], "custom value")
	}

	gateway := config["gateway"].(map[string]interface{})
	if gateway["port"] != float64(9999) {
		t.Errorf("gateway.port = %v, want 9999", gateway["port"])
	}

	auth := gateway["auth"].(map[string]interface{})
	if auth["mode"] != "token" {
		t.Errorf("gateway.auth.mode = %v, want %q", auth["mode"], "token")
	}
	if auth["token"] != "openclaw123" {
		t.Errorf("gateway.auth.token = %v, want %q", auth["token"], "openclaw123")
	}
	if _, hasOther := auth["otherOption"]; hasOther {
		t.Error("expected otherOption to be removed when auth is replaced")
	}

	controlUi := gateway["controlUi"].(map[string]interface{})
	if enabled, ok := controlUi["enabled"].(bool); !ok || !enabled {
		t.Errorf("gateway.controlUi.enabled = %v, want true", controlUi["enabled"])
	}
	if controlUi["basePath"] != "/custom" {
		t.Errorf("gateway.controlUi.basePath = %v, want %q", controlUi["basePath"], "/custom")
	}
}

func TestClaw_SkipOnboarding_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	settings := map[string]SettingsFile{
		ClawConfigPath: {Content: []byte("invalid json {{{")},
	}

	_, err := agent.SkipOnboarding(settings, "/workspace/sources", nil)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestClaw_SetModel_NoExistingSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	settings := make(map[string]SettingsFile)

	result, err := agent.SetModel(settings, "gpt-4o")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	sf, exists := result[ClawConfigPath]
	if !exists {
		t.Fatalf("Expected %s to be created", ClawConfigPath)
	}
	clawJSON := sf.Content

	var config map[string]interface{}
	if err := json.Unmarshal(clawJSON, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	agents, ok := config["agents"].(map[string]interface{})
	if !ok {
		t.Fatalf("agents is not a map: %v", config["agents"])
	}
	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		t.Fatalf("agents.defaults is not a map: %v", agents["defaults"])
	}
	if model, ok := defaults["model"].(string); !ok || model != "gpt-4o" {
		t.Errorf("agents.defaults.model = %v, want %q", defaults["model"], "gpt-4o")
	}
}

func TestClaw_SetModel_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	result, err := agent.SetModel(nil, "gpt-4o")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result map")
	}

	if _, exists := result[ClawConfigPath]; !exists {
		t.Errorf("Expected %s to be created", ClawConfigPath)
	}
}

func TestClaw_SetModel_ProviderModelFormat(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	settings := make(map[string]SettingsFile)

	result, err := agent.SetModel(settings, "anthropic::claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	agents := config["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	if model, ok := defaults["model"].(string); !ok || model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("agents.defaults.model = %v, want %q", defaults["model"], "anthropic/claude-sonnet-4-6")
	}
}

func TestClaw_SetModel_ProviderModelURLFormat(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	settings := make(map[string]SettingsFile)

	result, err := agent.SetModel(settings, "openai::gpt-4o::http://localhost:8080/v1")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	agents := config["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	if model, ok := defaults["model"].(string); !ok || model != "openai/gpt-4o" {
		t.Errorf("agents.defaults.model = %v, want %q", defaults["model"], "openai/gpt-4o")
	}

	models := config["models"].(map[string]interface{})
	providers := models["providers"].(map[string]interface{})
	p := providers["openai"].(map[string]interface{})
	if p["baseUrl"] != "http://host.containers.internal:8080/v1" {
		t.Errorf("baseUrl = %v, want %q", p["baseUrl"], "http://host.containers.internal:8080/v1")
	}
	pModels := p["models"].([]interface{})
	if len(pModels) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(pModels))
	}
	modelEntry := pModels[0].(map[string]interface{})
	if modelEntry["id"] != "gpt-4o" {
		t.Errorf("model id = %v, want %q", modelEntry["id"], "gpt-4o")
	}
	if modelEntry["name"] != "gpt-4o" {
		t.Errorf("model name = %v, want %q", modelEntry["name"], "gpt-4o")
	}
}

func TestClaw_SetModel_NoProviderWithURL(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	result, err := agent.SetModel(nil, "::tester::http://localhost:8080/v1")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	agents := config["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	if defaults["model"] != "local/tester" {
		t.Errorf("agents.defaults.model = %v, want %q", defaults["model"], "local/tester")
	}

	models := config["models"].(map[string]interface{})
	providers := models["providers"].(map[string]interface{})
	p := providers["local"].(map[string]interface{})
	if p["baseUrl"] != "http://host.containers.internal:8080/v1" {
		t.Errorf("baseUrl = %v, want %q", p["baseUrl"], "http://host.containers.internal:8080/v1")
	}
	if p["api"] != "openai-completions" {
		t.Errorf("api = %v, want %q", p["api"], "openai-completions")
	}
}

func TestClaw_SetModel_CloudProviderNoProviderBlock(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	result, err := agent.SetModel(nil, "anthropic::claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if _, hasModels := config["models"]; hasModels {
		t.Error("Expected no models.providers block for cloud provider")
	}
}

func TestClaw_SetModel_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	existingSettings := map[string]interface{}{
		"customField": "custom value",
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model":       "old-model",
				"otherOption": "preserved",
			},
			"otherAgent": map[string]interface{}{
				"name": "test",
			},
		},
	}

	existingJSON, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal existing settings: %v", err)
	}

	settings := map[string]SettingsFile{
		ClawConfigPath: {Content: existingJSON},
	}

	result, err := agent.SetModel(settings, "new-model")
	if err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if config["customField"] != "custom value" {
		t.Errorf("customField = %v, want %q", config["customField"], "custom value")
	}

	agents := config["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	if defaults["model"] != "new-model" {
		t.Errorf("agents.defaults.model = %v, want %q", defaults["model"], "new-model")
	}
	if defaults["otherOption"] != "preserved" {
		t.Errorf("agents.defaults.otherOption = %v, want %q", defaults["otherOption"], "preserved")
	}
	if _, ok := agents["otherAgent"]; !ok {
		t.Error("agents.otherAgent was not preserved")
	}
}

func TestClaw_SetModel_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaw()

	settings := map[string]SettingsFile{
		ClawConfigPath: {Content: []byte("invalid json {{{")},
	}

	_, err := agent.SetModel(settings, "gpt-4o")
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestClaw_SkillsDir(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	if got := agent.SkillsDir(); got != "$HOME/.openclaw/skills" {
		t.Errorf("SkillsDir() = %q, want %q", got, "$HOME/.openclaw/skills")
	}
}

func TestClaw_SetMCPServers_NilMCP(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	settings := map[string]SettingsFile{
		ClawConfigPath: {Content: []byte(`{"gateway":{"auth":false}}`)},
	}

	result, err := agent.SetMCPServers(settings, nil)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	if string(result[ClawConfigPath].Content) != `{"gateway":{"auth":false}}` {
		t.Errorf("SetMCPServers() with nil MCP modified settings unexpectedly: %s", result[ClawConfigPath].Content)
	}
}

func TestClaw_SetMCPServers_NilSettings(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	args := []string{"-y", "@modelcontextprotocol/server-filesystem"}
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "filesystem", Command: "npx", Args: &args},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result map")
	}
	if _, exists := result[ClawConfigPath]; !exists {
		t.Errorf("Expected %s to be created", ClawConfigPath)
	}
}

func TestClaw_SetMCPServers_CommandBased(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	args := []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"}
	env := map[string]string{"NODE_ENV": "production"}
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "filesystem", Command: "npx", Args: &args, Env: &env},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers := mcpMap["servers"].(map[string]interface{})
	fsServer := servers["filesystem"].(map[string]interface{})

	if _, hasTransport := fsServer["transport"]; hasTransport {
		t.Errorf("expected no transport field for command-based server, got %v", fsServer["transport"])
	}
	if fsServer["command"] != "npx" {
		t.Errorf("command = %v, want %q", fsServer["command"], "npx")
	}

	gotArgs, ok := fsServer["args"].([]interface{})
	if !ok {
		t.Fatalf("args is not a slice: %v", fsServer["args"])
	}
	if len(gotArgs) != 3 || gotArgs[0] != "-y" {
		t.Errorf("args = %v, want %v", gotArgs, args)
	}

	gotEnv, ok := fsServer["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("env is not a map: %v", fsServer["env"])
	}
	if gotEnv["NODE_ENV"] != "production" {
		t.Errorf("env.NODE_ENV = %v, want %q", gotEnv["NODE_ENV"], "production")
	}
}

func TestClaw_SetMCPServers_URLBased(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	headers := map[string]string{"Authorization": "Bearer token123"}
	mcp := &workspace.McpConfiguration{
		Servers: &[]workspace.McpServer{
			{Name: "remote", Url: "https://example.com/mcp", Headers: &headers},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers := mcpMap["servers"].(map[string]interface{})
	remoteServer := servers["remote"].(map[string]interface{})

	if remoteServer["transport"] != "streamable-http" {
		t.Errorf("transport = %v, want %q", remoteServer["transport"], "streamable-http")
	}
	if remoteServer["url"] != "https://example.com/mcp" {
		t.Errorf("url = %v, want %q", remoteServer["url"], "https://example.com/mcp")
	}

	gotHeaders, ok := remoteServer["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers is not a map: %v", remoteServer["headers"])
	}
	if gotHeaders["Authorization"] != "Bearer token123" {
		t.Errorf("headers.Authorization = %v, want %q", gotHeaders["Authorization"], "Bearer token123")
	}
}

func TestClaw_SetMCPServers_URLBased_NoHeaders(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	mcp := &workspace.McpConfiguration{
		Servers: &[]workspace.McpServer{
			{Name: "simple", Url: "https://example.com/mcp"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers := mcpMap["servers"].(map[string]interface{})
	server := servers["simple"].(map[string]interface{})

	if _, hasHeaders := server["headers"]; hasHeaders {
		t.Errorf("Expected no headers field when Headers is nil, got: %v", server["headers"])
	}
}

func TestClaw_SetMCPServers_Mixed(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "local-tool", Command: "python3", Args: &[]string{"/scripts/mcp.py"}},
		},
		Servers: &[]workspace.McpServer{
			{Name: "remote-api", Url: "https://api.example.com/mcp"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers, ok := mcpMap["servers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcp.servers is not a map")
	}
	if _, ok := servers["local-tool"]; !ok {
		t.Error("Expected local-tool server to be present")
	}
	if _, ok := servers["remote-api"]; !ok {
		t.Error("Expected remote-api server to be present")
	}
}

func TestClaw_SetMCPServers_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	existing := map[string]interface{}{
		"gateway": map[string]interface{}{"auth": false},
		"mcp": map[string]interface{}{
			"servers": map[string]interface{}{
				"existing-server": map[string]interface{}{
					"transport": "stdio",
					"command":   "existing-cmd",
				},
			},
		},
	}
	existingJSON, _ := json.Marshal(existing)

	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "new-tool", Command: "new-cmd"},
		},
	}

	result, err := agent.SetMCPServers(map[string]SettingsFile{ClawConfigPath: {Content: existingJSON}}, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	gateway := config["gateway"].(map[string]interface{})
	if gateway["auth"] != false {
		t.Error("gateway.auth was not preserved")
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers := mcpMap["servers"].(map[string]interface{})
	if _, ok := servers["existing-server"]; !ok {
		t.Error("existing-server was not preserved")
	}
	if _, ok := servers["new-tool"]; !ok {
		t.Error("new-tool was not added")
	}
}

func TestClaw_SetMCPServers_CommandNoArgs(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "tool", Command: "mytool"},
		},
	}

	result, err := agent.SetMCPServers(nil, mcp)
	if err != nil {
		t.Fatalf("SetMCPServers() error = %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(result[ClawConfigPath].Content, &config); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	mcpMap := config["mcp"].(map[string]interface{})
	servers := mcpMap["servers"].(map[string]interface{})
	server := servers["tool"].(map[string]interface{})

	args, ok := server["args"].([]interface{})
	if !ok || len(args) != 0 {
		t.Errorf("args = %v, want empty slice", server["args"])
	}
	envMap, ok := server["env"].(map[string]interface{})
	if !ok || len(envMap) != 0 {
		t.Errorf("env = %v, want empty map", server["env"])
	}
}

func TestClaw_SetMCPServers_InvalidJSON(t *testing.T) {
	t.Parallel()

	agent := NewClaw()
	mcp := &workspace.McpConfiguration{
		Commands: &[]workspace.McpCommand{
			{Name: "tool", Command: "mytool"},
		},
	}

	_, err := agent.SetMCPServers(map[string]SettingsFile{ClawConfigPath: {Content: []byte("invalid json {{{}")}}, mcp)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
