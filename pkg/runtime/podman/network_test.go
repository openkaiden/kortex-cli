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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/runtime/podman/exec"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

func assertAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got, want := r.Header.Get("Authorization"), "Bearer oc_testkey"; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestConfigureNetworking(t *testing.T) {
	t.Parallel()

	t.Run("creates manual_approval rule and writes config", func(t *testing.T) {
		t.Parallel()

		var createdRules []onecli.CreateRuleInput

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				_ = json.NewEncoder(w).Encode([]onecli.Rule{})
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				var input onecli.CreateRuleInput
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Errorf("decoding rule: %v", err)
				}
				createdRules = append(createdRules, input)
				_ = json.NewEncoder(w).Encode(onecli.Rule{ID: "new-rule"})
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		approvalDir := t.TempDir()

		hosts := []string{"api.github.com", "registry.npmjs.org"}
		err := rt.configureNetworking(context.Background(), server.URL, hosts, approvalDir)
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		if len(createdRules) != 1 {
			t.Fatalf("got %d rules, want 1", len(createdRules))
		}

		rule := createdRules[0]
		if rule.HostPattern != "*" {
			t.Errorf("rule.HostPattern = %q, want %q", rule.HostPattern, "*")
		}
		if rule.Action != "manual_approval" {
			t.Errorf("rule.Action = %q, want %q", rule.Action, "manual_approval")
		}
		if rule.Name != "manual-approval-all" {
			t.Errorf("rule.Name = %q, want %q", rule.Name, "manual-approval-all")
		}

		// Verify config.json was written with correct content
		data, err := os.ReadFile(filepath.Join(approvalDir, "config.json"))
		if err != nil {
			t.Fatalf("reading config.json: %v", err)
		}
		var cfg approvalHandlerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshaling config.json: %v", err)
		}
		if cfg.OnecliURL != "http://localhost:10254" {
			t.Errorf("config.onecliUrl = %q, want %q", cfg.OnecliURL, "http://localhost:10254")
		}
		if cfg.GatewayURL != "http://localhost:10255" {
			t.Errorf("config.gatewayUrl = %q, want %q", cfg.GatewayURL, "http://localhost:10255")
		}
		if cfg.APIKey != "oc_testkey" {
			t.Errorf("config.apiKey = %q, want %q", cfg.APIKey, "oc_testkey")
		}
		if len(cfg.Hosts) != 2 || cfg.Hosts[0] != "api.github.com" || cfg.Hosts[1] != "registry.npmjs.org" {
			t.Errorf("config.hosts = %v, want [api.github.com registry.npmjs.org]", cfg.Hosts)
		}
	})

	t.Run("deletes existing rules before creating new ones", func(t *testing.T) {
		t.Parallel()

		deletedIDs := []string{}
		operations := []string{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				existing := []onecli.Rule{
					{ID: "old-1", Name: "old-rule-1", HostPattern: "old.example.com", Action: "rate_limit"},
					{ID: "old-2", Name: "block-all", HostPattern: "*", Action: "block"},
				}
				_ = json.NewEncoder(w).Encode(existing)
			case r.Method == http.MethodDelete:
				assertAuth(t, r)
				id := r.URL.Path[len("/api/rules/"):]
				deletedIDs = append(deletedIDs, id)
				operations = append(operations, "delete:"+id)
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				operations = append(operations, "create")
				_ = json.NewEncoder(w).Encode(onecli.Rule{ID: "new-rule"})
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		approvalDir := t.TempDir()
		err := rt.configureNetworking(context.Background(), server.URL, []string{"api.github.com"}, approvalDir)
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		// Assert exact deleted IDs
		wantDeleted := []string{"old-1", "old-2"}
		if len(deletedIDs) != len(wantDeleted) {
			t.Fatalf("deletedIDs = %v, want %v", deletedIDs, wantDeleted)
		}
		for i, want := range wantDeleted {
			if deletedIDs[i] != want {
				t.Fatalf("deletedIDs = %v, want %v", deletedIDs, wantDeleted)
			}
		}

		// Assert all deletes happened before creates
		wantOps := []string{"delete:old-1", "delete:old-2", "create"}
		if len(operations) != len(wantOps) {
			t.Fatalf("operations = %v, want %v", operations, wantOps)
		}
		for i, want := range wantOps {
			if operations[i] != want {
				t.Fatalf("operations = %v, want deletes before creates", operations)
			}
		}
	})

	t.Run("handles empty hosts with manual_approval rule", func(t *testing.T) {
		t.Parallel()

		var createdRules []onecli.CreateRuleInput

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				_ = json.NewEncoder(w).Encode([]onecli.Rule{})
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				var input onecli.CreateRuleInput
				_ = json.NewDecoder(r.Body).Decode(&input)
				createdRules = append(createdRules, input)
				_ = json.NewEncoder(w).Encode(onecli.Rule{ID: "new-rule"})
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		approvalDir := t.TempDir()
		err := rt.configureNetworking(context.Background(), server.URL, []string{}, approvalDir)
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		if len(createdRules) != 1 {
			t.Fatalf("got %d rules, want 1 (manual_approval)", len(createdRules))
		}
		if createdRules[0].Action != "manual_approval" || createdRules[0].HostPattern != "*" {
			t.Errorf("expected manual_approval rule, got %+v", createdRules[0])
		}

		// Verify config.json has empty hosts
		data, err := os.ReadFile(filepath.Join(approvalDir, "config.json"))
		if err != nil {
			t.Fatalf("reading config.json: %v", err)
		}
		var cfg approvalHandlerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshaling config.json: %v", err)
		}
		if len(cfg.Hosts) != 0 {
			t.Errorf("config.hosts = %v, want empty", cfg.Hosts)
		}
	})

	t.Run("returns error when API key retrieval fails", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		approvalDir := t.TempDir()
		err := rt.configureNetworking(context.Background(), server.URL, []string{}, approvalDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestClearNetworkingRules(t *testing.T) {
	t.Parallel()

	t.Run("deletes all existing rules", func(t *testing.T) {
		t.Parallel()

		deletedIDs := []string{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				assertAuth(t, r)
				existing := []onecli.Rule{
					{ID: "rule-1", Name: "manual-approval-all", HostPattern: "*", Action: "manual_approval"},
					{ID: "rule-2", Name: "old-rule", HostPattern: "example.com", Action: "block"},
				}
				_ = json.NewEncoder(w).Encode(existing)
			case r.Method == http.MethodDelete:
				assertAuth(t, r)
				deletedIDs = append(deletedIDs, r.URL.Path[len("/api/rules/"):])
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		err := rt.clearNetworkingRules(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("clearNetworkingRules() error: %v", err)
		}

		if len(deletedIDs) != 2 {
			t.Fatalf("expected 2 deletions, got %d: %v", len(deletedIDs), deletedIDs)
		}
		if deletedIDs[0] != "rule-1" || deletedIDs[1] != "rule-2" {
			t.Errorf("deleted IDs = %v, want [rule-1 rule-2]", deletedIDs)
		}
	})

	t.Run("succeeds when no rules exist", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				_ = json.NewEncoder(w).Encode([]onecli.Rule{})
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		if err := rt.clearNetworkingRules(context.Background(), server.URL); err != nil {
			t.Fatalf("clearNetworkingRules() error: %v", err)
		}
	})

	t.Run("returns error when API key retrieval fails", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		if err := rt.clearNetworkingRules(context.Background(), server.URL); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBuildNftScript(t *testing.T) {
	t.Parallel()

	t.Run("generates IPv4 and IPv6 rules blocking agent UID", func(t *testing.T) {
		t.Parallel()

		script := buildNftScript(1000, "")

		if !strings.Contains(script, "command -v nft") {
			t.Error("expected nftables install guard")
		}
		if !strings.Contains(script, "nft add table ip filter") {
			t.Error("expected IPv4 table creation")
		}
		if !strings.Contains(script, "nft add table ip6 filter") {
			t.Error("expected IPv6 table creation")
		}
		if !strings.Contains(script, "policy accept") {
			t.Error("expected ACCEPT default policy")
		}
		if !strings.Contains(script, "meta skuid 1000 drop") {
			t.Error("expected agent UID drop rule")
		}
		if !strings.Contains(script, "oif lo accept") {
			t.Error("expected loopback rule")
		}
		if strings.Contains(script, "ip daddr") {
			t.Error("expected no host-gateway rule when hostGW is empty")
		}
	})

	t.Run("includes host-gateway rule when IP provided", func(t *testing.T) {
		t.Parallel()

		script := buildNftScript(1000, "10.0.2.2")

		if !strings.Contains(script, "nft add rule ip filter output ip daddr 10.0.2.2 accept") {
			t.Error("expected host-gateway rule for 10.0.2.2")
		}
	})

	t.Run("host-gateway rule comes before agent drop rule", func(t *testing.T) {
		t.Parallel()

		script := buildNftScript(1000, "10.0.2.2")

		hostGWIdx := strings.Index(script, "ip daddr 10.0.2.2 accept")
		dropIdx := strings.Index(script, "meta skuid 1000 drop")
		if hostGWIdx >= dropIdx {
			t.Error("host-gateway accept should come before agent drop rule")
		}
	})

	t.Run("idempotent delete before create", func(t *testing.T) {
		t.Parallel()

		script := buildNftScript(1000, "")

		ipv4DeleteIdx := strings.Index(script, "nft delete table ip filter")
		ipv4CreateIdx := strings.Index(script, "nft add table ip filter")
		if ipv4DeleteIdx >= ipv4CreateIdx {
			t.Error("IPv4 delete should come before create")
		}

		ipv6DeleteIdx := strings.Index(script, "nft delete table ip6 filter")
		ipv6CreateIdx := strings.Index(script, "nft add table ip6 filter")
		if ipv6DeleteIdx >= ipv6CreateIdx {
			t.Error("IPv6 delete should come before create")
		}
	})
}

func TestSetupFirewallRules(t *testing.T) {
	t.Parallel()

	t.Run("execs nft commands into network-guard container", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 5 && args[0] == "exec" && args[2] == "sh" && args[3] == "-c" {
				return []byte("10.0.2.2\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.setupFirewallRules(context.Background(), "my-pod", 1000)
		if err != nil {
			t.Fatalf("setupFirewallRules() error: %v", err)
		}

		found := false
		for _, call := range fakeExec.RunCalls {
			if len(call) >= 4 && call[0] == "exec" && call[1] == "my-pod-network-guard" && call[2] == "sh" && call[3] == "-c" {
				script := call[4]
				if strings.Contains(script, "meta skuid 1000 drop") && strings.Contains(script, "ip daddr 10.0.2.2") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected exec call with nft script including agent UID drop and host-gateway rules")
		}
	})

	t.Run("skips host-gateway rule when resolution fails on non-WSL", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("getent failed")
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.setupFirewallRules(context.Background(), "my-pod", 1000)
		if err != nil {
			t.Fatalf("setupFirewallRules() error: %v", err)
		}

		for _, call := range fakeExec.RunCalls {
			if len(call) >= 5 && call[0] == "exec" && call[2] == "sh" {
				if strings.Contains(call[4], "ip daddr") {
					t.Error("expected no host-gateway rule when resolution fails on non-WSL")
				}
			}
		}
	})

	t.Run("uses podman machine default gateway fallback on WSL2 when getent fails", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 5 && strings.Contains(args[4], "getent hosts") {
				return nil, fmt.Errorf("getent failed")
			}
			if len(args) >= 4 && args[0] == "machine" && args[1] == "info" {
				return []byte("wsl\n"), nil
			}
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte("default via 192.168.1.1 dev eth0\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.setupFirewallRules(context.Background(), "my-pod", 1000)
		if err != nil {
			t.Fatalf("setupFirewallRules() error: %v", err)
		}

		found := false
		for _, call := range fakeExec.RunCalls {
			if len(call) >= 5 && call[0] == "exec" && call[2] == "sh" && call[3] == "-c" {
				if strings.Contains(call[4], "ip daddr 192.168.1.1 accept") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected host-gateway rule with nameserver IP on WSL2")
		}
	})

	t.Run("uses podman machine default gateway fallback on WSL2 when getent returns 127.0.0.1", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 5 && strings.Contains(args[4], "getent hosts") {
				return []byte("127.0.0.1\n"), nil
			}
			if len(args) >= 4 && args[0] == "machine" && args[1] == "info" {
				return []byte("wsl\n"), nil
			}
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte("default via 172.20.0.1 dev eth0\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.setupFirewallRules(context.Background(), "my-pod", 1000)
		if err != nil {
			t.Fatalf("setupFirewallRules() error: %v", err)
		}

		found := false
		for _, call := range fakeExec.RunCalls {
			if len(call) >= 5 && call[0] == "exec" && call[2] == "sh" && call[3] == "-c" {
				if strings.Contains(call[4], "ip daddr 172.20.0.1 accept") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected host-gateway rule with nameserver IP when getent returns 127.0.0.1")
		}
	})

	t.Run("returns error when exec fails", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
			return fmt.Errorf("exec failed")
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.setupFirewallRules(context.Background(), "my-pod", 1000)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestClearFirewallRules(t *testing.T) {
	t.Parallel()

	t.Run("execs delete commands into network-guard container", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		rt := &podmanRuntime{executor: fakeExec}

		err := rt.clearFirewallRules(context.Background(), "my-pod")
		if err != nil {
			t.Fatalf("clearFirewallRules() error: %v", err)
		}

		found := false
		for _, call := range fakeExec.RunCalls {
			if len(call) >= 4 && call[0] == "exec" && call[1] == "my-pod-network-guard" && call[2] == "sh" && call[3] == "-c" {
				script := call[4]
				if strings.Contains(script, "nft delete table ip filter") && strings.Contains(script, "nft delete table ip6 filter") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected exec call with delete table commands")
		}
	})

	t.Run("returns error when exec fails", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.RunFunc = func(ctx context.Context, args ...string) error {
			return fmt.Errorf("exec failed")
		}

		rt := &podmanRuntime{executor: fakeExec}

		err := rt.clearFirewallRules(context.Background(), "my-pod")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// fakeSecretStore implements secret.Store using a pre-populated list.
type fakeSecretStore struct {
	items []secret.ListItem
	err   error
}

var _ secret.Store = (*fakeSecretStore)(nil)

func (f *fakeSecretStore) List() ([]secret.ListItem, error) { return f.items, f.err }
func (f *fakeSecretStore) Get(string) (secret.ListItem, string, error) {
	return secret.ListItem{}, "", nil
}
func (f *fakeSecretStore) Create(secret.CreateParams) error { return nil }
func (f *fakeSecretStore) Remove(string) error              { return nil }

func makeSecretService(name string, patterns []string) secretservice.SecretService {
	return secretservice.NewSecretService(name, patterns, "", nil, "", "", "")
}

func makeRegistry(t *testing.T, services ...secretservice.SecretService) secretservice.Registry {
	t.Helper()
	reg := secretservice.NewRegistry()
	for _, svc := range services {
		if err := reg.Register(svc); err != nil {
			t.Fatalf("makeRegistry: failed to register %q: %v", svc.Name(), err)
		}
	}
	return reg
}

func denyConfig(secrets []string) *workspace.WorkspaceConfiguration {
	mode := workspace.Deny
	cfg := &workspace.WorkspaceConfiguration{
		Network: &workspace.NetworkConfiguration{Mode: &mode},
		Secrets: &secrets,
	}
	return cfg
}

func TestCollectSecretHosts(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns nil", func(t *testing.T) {
		t.Parallel()
		got, err := collectSecretHosts(nil, &fakeSecretStore{}, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil secrets field returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := &workspace.WorkspaceConfiguration{}
		got, err := collectSecretHosts(cfg, &fakeSecretStore{}, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("empty secrets list returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := denyConfig([]string{})
		got, err := collectSecretHosts(cfg, &fakeSecretStore{}, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil store returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := denyConfig([]string{"mysecret"})
		got, err := collectSecretHosts(cfg, nil, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil registry returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := denyConfig([]string{"mysecret"})
		got, err := collectSecretHosts(cfg, &fakeSecretStore{}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("known type secret returns service host patterns", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{items: []secret.ListItem{{Name: "mygithub", Type: "github"}}}
		reg := makeRegistry(t, makeSecretService("github", []string{"api.github.com"}))
		got, err := collectSecretHosts(denyConfig([]string{"mygithub"}), store, reg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "api.github.com" {
			t.Errorf("got %v, want [api.github.com]", got)
		}
	})

	t.Run("other type secret returns item hosts", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{
			items: []secret.ListItem{
				{Name: "mykey", Type: secret.TypeOther, Hosts: []string{"api.example.com", "api2.example.com"}},
			},
		}
		got, err := collectSecretHosts(denyConfig([]string{"mykey"}), store, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "api.example.com" || got[1] != "api2.example.com" {
			t.Errorf("got %v, want [api.example.com api2.example.com]", got)
		}
	})

	t.Run("deduplicates hosts across multiple secrets of the same type", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{
			items: []secret.ListItem{
				{Name: "gh1", Type: "github"},
				{Name: "gh2", Type: "github"},
			},
		}
		reg := makeRegistry(t, makeSecretService("github", []string{"api.github.com"}))
		got, err := collectSecretHosts(denyConfig([]string{"gh1", "gh2"}), store, reg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "api.github.com" {
			t.Errorf("got %v, want [api.github.com] (deduplicated)", got)
		}
	})

	t.Run("mixes known and other type secrets", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{
			items: []secret.ListItem{
				{Name: "mygithub", Type: "github"},
				{Name: "myother", Type: secret.TypeOther, Hosts: []string{"custom.example.com"}},
			},
		}
		reg := makeRegistry(t, makeSecretService("github", []string{"api.github.com"}))
		got, err := collectSecretHosts(denyConfig([]string{"mygithub", "myother"}), store, reg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"api.github.com", "custom.example.com"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i, h := range want {
			if got[i] != h {
				t.Errorf("got[%d] = %q, want %q", i, got[i], h)
			}
		}
	})

	t.Run("skips secrets not found in store", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{items: []secret.ListItem{}}
		got, err := collectSecretHosts(denyConfig([]string{"missing"}), store, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("skips secrets with type not in registry", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{items: []secret.ListItem{{Name: "mykey", Type: "unknown-type"}}}
		got, err := collectSecretHosts(denyConfig([]string{"mykey"}), store, makeRegistry(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("store list error returns error", func(t *testing.T) {
		t.Parallel()
		store := &fakeSecretStore{err: errors.New("disk error")}
		_, err := collectSecretHosts(denyConfig([]string{"mykey"}), store, makeRegistry(t))
		if err == nil {
			t.Error("expected error from store.List(), got nil")
		}
	})
}

func TestIsPodmanWSL(t *testing.T) {
	t.Parallel()

	t.Run("returns true when VMType is wsl", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "info" {
				return []byte("wsl\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		if !rt.isPodmanWSL(context.Background()) {
			t.Error("expected isPodmanWSL() to return true for wsl VMType")
		}
	})

	t.Run("returns false when VMType is not wsl", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "info" {
				return []byte("qemu\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		if rt.isPodmanWSL(context.Background()) {
			t.Error("expected isPodmanWSL() to return false for qemu VMType")
		}
	})

	t.Run("returns false when machine info fails", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("no machine")
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		if rt.isPodmanWSL(context.Background()) {
			t.Error("expected isPodmanWSL() to return false on error")
		}
	})
}

func TestInjectWSLHostEntry(t *testing.T) {
	t.Parallel()

	t.Run("adds /etc/hosts entry with nameserver IP from podman machine", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte("default via 10.255.0.1 dev eth0\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.injectWSLHostEntry(context.Background(), "test-container")
		if err != nil {
			t.Fatalf("injectWSLHostEntry() error: %v", err)
		}

		found := false
		for _, call := range fakeExec.RunCalls {
			if len(call) >= 7 && call[0] == "exec" && call[1] == "--user" && call[2] == "root" && call[3] == "test-container" && call[4] == "sh" && call[5] == "-c" {
				script := call[6]
				if strings.Contains(script, "grep -v") && strings.Contains(script, "10.255.0.1 native-host.internal") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected exec call writing native-host.internal to /etc/hosts")
		}
	})

	t.Run("returns error when host IP cannot be resolved", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return nil, fmt.Errorf("ssh failed")
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.injectWSLHostEntry(context.Background(), "test-container")
		if err == nil {
			t.Fatal("expected error when host IP resolution fails, got nil")
		}
	})

	t.Run("returns error when default gateway cannot be determined", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte(""), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}

		err := rt.injectWSLHostEntry(context.Background(), "test-container")
		if err == nil {
			t.Fatal("expected error when default gateway cannot be determined, got nil")
		}
	})
}

func TestResolveWSLHostIP(t *testing.T) {
	t.Parallel()

	t.Run("extracts default gateway via podman machine ssh", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte("default via 172.30.0.1 dev eth0\n"), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		ip := rt.resolveWSLHostIP(context.Background())
		if ip != "172.30.0.1" {
			t.Errorf("got %q, want %q", ip, "172.30.0.1")
		}
	})

	t.Run("returns empty when ssh fails", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("ssh failed")
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		if ip := rt.resolveWSLHostIP(context.Background()); ip != "" {
			t.Errorf("got %q, want empty", ip)
		}
	})

	t.Run("returns empty when no default route", func(t *testing.T) {
		t.Parallel()

		fakeExec := exec.NewFake()
		fakeExec.OutputFunc = func(ctx context.Context, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "machine" && args[1] == "ssh" {
				return []byte(""), nil
			}
			return []byte{}, nil
		}

		rt := &podmanRuntime{executor: fakeExec, system: &fakeSystem{}}
		if ip := rt.resolveWSLHostIP(context.Background()); ip != "" {
			t.Errorf("got %q, want empty", ip)
		}
	})
}

func TestMergeHosts(t *testing.T) {
	t.Parallel()

	t.Run("returns a when b is empty", func(t *testing.T) {
		t.Parallel()
		a := []string{"a.com", "b.com"}
		got := mergeHosts(a, nil)
		if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
			t.Errorf("got %v, want %v", got, a)
		}
	})

	t.Run("deduplicates overlapping entries", func(t *testing.T) {
		t.Parallel()
		got := mergeHosts([]string{"a.com", "b.com"}, []string{"b.com", "c.com"})
		want := []string{"a.com", "b.com", "c.com"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i, h := range want {
			if got[i] != h {
				t.Errorf("got[%d] = %q, want %q", i, got[i], h)
			}
		}
	})

	t.Run("preserves order a first then new entries from b", func(t *testing.T) {
		t.Parallel()
		got := mergeHosts([]string{"x.com"}, []string{"y.com"})
		if len(got) != 2 || got[0] != "x.com" || got[1] != "y.com" {
			t.Errorf("got %v, want [x.com y.com]", got)
		}
	})
}
