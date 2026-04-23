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
	"testing"

	"github.com/openkaiden/kdn/pkg/onecli"
)

func TestConfigureNetworking(t *testing.T) {
	t.Parallel()

	t.Run("creates allow and block rules", func(t *testing.T) {
		t.Parallel()

		var createdRules []onecli.CreateRuleInput

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				_ = json.NewEncoder(w).Encode([]onecli.Rule{})
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
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

		hosts := []string{"api.github.com", "registry.npmjs.org"}
		err := rt.configureNetworking(context.Background(), server.URL, hosts)
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		// Expect 2 allow rules + 1 block-all rule
		if len(createdRules) != 3 {
			t.Fatalf("got %d rules, want 3", len(createdRules))
		}

		for i, host := range hosts {
			if createdRules[i].HostPattern != host {
				t.Errorf("rule[%d].HostPattern = %q, want %q", i, createdRules[i].HostPattern, host)
			}
			if createdRules[i].Action != "rate_limit" {
				t.Errorf("rule[%d].Action = %q, want %q", i, createdRules[i].Action, "rate_limit")
			}
			if createdRules[i].RateLimit != 65535 {
				t.Errorf("rule[%d].RateLimit = %d, want 65535", i, createdRules[i].RateLimit)
			}
			if createdRules[i].RateLimitWindow != "minute" {
				t.Errorf("rule[%d].RateLimitWindow = %q, want %q", i, createdRules[i].RateLimitWindow, "minute")
			}
		}

		last := createdRules[2]
		if last.HostPattern != "*" {
			t.Errorf("block-all HostPattern = %q, want %q", last.HostPattern, "*")
		}
		if last.Action != "block" {
			t.Errorf("block-all Action = %q, want %q", last.Action, "block")
		}
	})

	t.Run("deletes existing rules before creating new ones", func(t *testing.T) {
		t.Parallel()

		deletedIDs := []string{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				existing := []onecli.Rule{
					{ID: "old-1", Name: "old-rule-1", HostPattern: "old.example.com", Action: "rate_limit"},
					{ID: "old-2", Name: "block-all", HostPattern: "*", Action: "block"},
				}
				_ = json.NewEncoder(w).Encode(existing)
			case r.Method == http.MethodDelete:
				id := r.URL.Path[len("/api/rules/"):]
				deletedIDs = append(deletedIDs, id)
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
				_ = json.NewEncoder(w).Encode(onecli.Rule{ID: "new-rule"})
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		err := rt.configureNetworking(context.Background(), server.URL, []string{"api.github.com"})
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		if len(deletedIDs) != 2 {
			t.Errorf("deleted %d rules, want 2", len(deletedIDs))
		}
	})

	t.Run("handles empty hosts with only block-all rule", func(t *testing.T) {
		t.Parallel()

		var createdRules []onecli.CreateRuleInput

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/user/api-key":
				_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_testkey"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/rules":
				_ = json.NewEncoder(w).Encode([]onecli.Rule{})
			case r.Method == http.MethodPost && r.URL.Path == "/api/rules":
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
		err := rt.configureNetworking(context.Background(), server.URL, []string{})
		if err != nil {
			t.Fatalf("configureNetworking() error: %v", err)
		}

		if len(createdRules) != 1 {
			t.Fatalf("got %d rules, want 1 (block-all)", len(createdRules))
		}
		if createdRules[0].HostPattern != "*" || createdRules[0].Action != "block" {
			t.Errorf("expected block-all rule, got %+v", createdRules[0])
		}
	})

	t.Run("returns error when API key retrieval fails", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		rt := &podmanRuntime{}
		err := rt.configureNetworking(context.Background(), server.URL, []string{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
