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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/openkaiden/kdn/pkg/onecli"
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
