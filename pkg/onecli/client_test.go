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

package onecli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSecret(t *testing.T) {
	t.Parallel()

	want := Secret{
		ID:          "sec-123",
		Name:        "GitHub Token",
		Type:        "generic",
		HostPattern: "api.github.com",
	}

	var gotInput CreateSecretInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/secrets" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotInput); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	input := CreateSecretInput{
		Name:        "GitHub Token",
		Type:        "generic",
		Value:       "ghp_xxx",
		HostPattern: "api.github.com",
	}
	got, err := c.CreateSecret(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateSecret() error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("got ID %q, want %q", got.ID, want.ID)
	}
	if gotInput.Name != input.Name {
		t.Errorf("server got name %q, want %q", gotInput.Name, input.Name)
	}
}

func TestCreateSecret_Conflict(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "secret already exists"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	_, err := c.CreateSecret(context.Background(), CreateSecretInput{Name: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("got status %d, want %d", apiErr.StatusCode, http.StatusConflict)
	}
}

func TestCreateSecret_AuthFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "bad-key")
	_, err := c.CreateSecret(context.Background(), CreateSecretInput{Name: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestListSecrets(t *testing.T) {
	t.Parallel()

	want := []Secret{
		{ID: "sec-1", Name: "one"},
		{ID: "sec-2", Name: "two"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/secrets" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	got, err := c.ListSecrets(context.Background())
	if err != nil {
		t.Fatalf("ListSecrets() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d secrets, want 2", len(got))
	}
	if got[0].ID != "sec-1" || got[1].ID != "sec-2" {
		t.Errorf("unexpected secrets: %+v", got)
	}
}

func TestDeleteSecret(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/secrets/sec-123" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	if err := c.DeleteSecret(context.Background(), "sec-123"); err != nil {
		t.Fatalf("DeleteSecret() error: %v", err)
	}
}

func TestGetContainerConfig(t *testing.T) {
	t.Parallel()

	want := ContainerConfig{
		Env: map[string]string{
			"HTTPS_PROXY":         "http://x:aoc_token@localhost:10255",
			"HTTP_PROXY":          "http://x:aoc_token@localhost:10255",
			"NODE_EXTRA_CA_CERTS": "/tmp/onecli-gateway-ca.pem",
			"ANTHROPIC_API_KEY":   "placeholder",
		},
		CACertificate:              "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----",
		CACertificateContainerPath: "/tmp/onecli-gateway-ca.pem",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/container-config" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	got, err := c.GetContainerConfig(context.Background())
	if err != nil {
		t.Fatalf("GetContainerConfig() error: %v", err)
	}
	if got.CACertificateContainerPath != want.CACertificateContainerPath {
		t.Errorf("CACertificateContainerPath = %q, want %q", got.CACertificateContainerPath, want.CACertificateContainerPath)
	}
	if got.Env["HTTPS_PROXY"] != want.Env["HTTPS_PROXY"] {
		t.Errorf("HTTPS_PROXY = %q, want %q", got.Env["HTTPS_PROXY"], want.Env["HTTPS_PROXY"])
	}
	if got.CACertificate == "" {
		t.Error("CACertificate should not be empty")
	}
}

func TestCreateRule(t *testing.T) {
	t.Parallel()

	want := Rule{
		ID:          "rule-1",
		Name:        "allow-api.github.com",
		HostPattern: "api.github.com",
		Action:      "rate_limit",
		Enabled:     true,
	}

	var gotInput CreateRuleInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/rules" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotInput); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	input := CreateRuleInput{
		Name:            "allow-api.github.com",
		HostPattern:     "api.github.com",
		Action:          "rate_limit",
		Enabled:         true,
		RateLimit:       65535,
		RateLimitWindow: "minute",
	}

	c := NewClient(server.URL, "test-key")
	got, err := c.CreateRule(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateRule() error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("got ID %q, want %q", got.ID, want.ID)
	}
	if gotInput.HostPattern != input.HostPattern {
		t.Errorf("got HostPattern %q, want %q", gotInput.HostPattern, input.HostPattern)
	}
	if gotInput.RateLimit != input.RateLimit {
		t.Errorf("got RateLimit %d, want %d", gotInput.RateLimit, input.RateLimit)
	}
}

func TestListRules(t *testing.T) {
	t.Parallel()

	want := []Rule{
		{ID: "rule-1", Name: "allow-github", HostPattern: "api.github.com", Action: "rate_limit", Enabled: true},
		{ID: "rule-2", Name: "block-all", HostPattern: "*", Action: "block", Enabled: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/rules" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	got, err := c.ListRules(context.Background())
	if err != nil {
		t.Fatalf("ListRules() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rules, want 2", len(got))
	}
	if got[0].ID != "rule-1" || got[1].ID != "rule-2" {
		t.Errorf("unexpected rules: %+v", got)
	}
}

func TestDeleteRule(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/rules/rule-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key")
	if err := c.DeleteRule(context.Background(), "rule-1"); err != nil {
		t.Fatalf("DeleteRule() error: %v", err)
	}
}

func TestCreateRule_AuthFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "bad-key")
	_, err := c.CreateRule(context.Background(), CreateRuleInput{Name: "x", HostPattern: "x", Action: "block", Enabled: true})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}
