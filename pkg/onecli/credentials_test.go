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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCredentialProvider_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/api-key" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "oc_test_key_123"})
	}))
	defer server.Close()

	provider := NewCredentialProvider(server.URL)
	key, err := provider.APIKey(context.Background())
	if err != nil {
		t.Fatalf("APIKey() error: %v", err)
	}
	if key != "oc_test_key_123" {
		t.Errorf("got key %q, want %q", key, "oc_test_key_123")
	}
}

func TestCredentialProvider_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider := NewCredentialProvider(server.URL)
	_, err := provider.APIKey(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestCredentialProvider_InvalidKeyFormat(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "bad_prefix"})
	}))
	defer server.Close()

	provider := NewCredentialProvider(server.URL)
	_, err := provider.APIKey(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid key format")
	}
}

func TestCredentialProvider_Unreachable(t *testing.T) {
	t.Parallel()

	provider := NewCredentialProvider("http://localhost:1")
	_, err := provider.APIKey(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
