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

package secret

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeKeyring records Set calls without touching the real system keychain.
type fakeKeyring struct {
	calls []fakeKeyringCall
	err   error
}

type fakeKeyringCall struct {
	service  string
	user     string
	password string
}

func (f *fakeKeyring) Set(service, user, password string) error {
	f.calls = append(f.calls, fakeKeyringCall{service, user, password})
	return f.err
}

func TestStore_Create_StoresValueInKeychain(t *testing.T) {
	t.Parallel()

	kr := &fakeKeyring{}
	st := newStoreWithKeyring(t.TempDir(), kr)

	err := st.Create(CreateParams{
		Name:  "my-token",
		Type:  "github",
		Value: "ghp_secret",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if len(kr.calls) != 1 {
		t.Fatalf("expected 1 keychain call, got %d", len(kr.calls))
	}
	call := kr.calls[0]
	if call.service != keyringService {
		t.Errorf("expected service %q, got %q", keyringService, call.service)
	}
	if call.user != "my-token" {
		t.Errorf("expected user %q, got %q", "my-token", call.user)
	}
	if call.password != "ghp_secret" {
		t.Errorf("expected password %q, got %q", "ghp_secret", call.password)
	}
}

func TestStore_Create_SavesMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st := newStoreWithKeyring(dir, &fakeKeyring{})

	err := st.Create(CreateParams{
		Name:           "my-api-key",
		Type:           TypeOther,
		Value:          "secret123",
		Description:    "API key for example service",
		Hosts:          []string{"api.example.com"},
		Path:           "/api/v1",
		Header:         "Authorization",
		HeaderTemplate: "Bearer ${value}",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, secretsFileName))
	if err != nil {
		t.Fatalf("failed to read secrets file: %v", err)
	}

	var sf secretsFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("failed to parse secrets file: %v", err)
	}

	if len(sf.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(sf.Secrets))
	}

	rec := sf.Secrets[0]
	if rec.Name != "my-api-key" {
		t.Errorf("Name: want %q, got %q", "my-api-key", rec.Name)
	}
	if rec.Type != TypeOther {
		t.Errorf("Type: want %q, got %q", TypeOther, rec.Type)
	}
	if rec.Description != "API key for example service" {
		t.Errorf("Description: want %q, got %q", "API key for example service", rec.Description)
	}
	if len(rec.Hosts) != 1 || rec.Hosts[0] != "api.example.com" {
		t.Errorf("Hosts: want [api.example.com], got %v", rec.Hosts)
	}
	if rec.Path != "/api/v1" {
		t.Errorf("Path: want %q, got %q", "/api/v1", rec.Path)
	}
	if rec.Header != "Authorization" {
		t.Errorf("Header: want %q, got %q", "Authorization", rec.Header)
	}
	if rec.HeaderTemplate != "Bearer ${value}" {
		t.Errorf("HeaderTemplate: want %q, got %q", "Bearer ${value}", rec.HeaderTemplate)
	}
}

func TestStore_Create_ErrorsOnDuplicate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	kr := &fakeKeyring{}
	st := newStoreWithKeyring(dir, kr)

	params := CreateParams{
		Name:           "my-token",
		Type:           TypeOther,
		Value:          "v1",
		Hosts:          []string{"example.com"},
		Path:           "/",
		Header:         "Authorization",
		HeaderTemplate: "Bearer ${value}",
	}
	if err := st.Create(params); err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	callsBefore := len(kr.calls)
	params.Value = "v2"
	err := st.Create(params)
	if err == nil {
		t.Fatal("expected error when creating duplicate secret")
	}
	if !errors.Is(err, ErrSecretAlreadyExists) {
		t.Errorf("expected ErrSecretAlreadyExists, got: %v", err)
	}
	// Keychain must not be touched when the duplicate is detected
	if len(kr.calls) != callsBefore {
		t.Errorf("keychain was written despite duplicate: got %d total calls, want %d", len(kr.calls), callsBefore)
	}
}

func TestStore_Create_KeychainError(t *testing.T) {
	t.Parallel()

	kr := &fakeKeyring{err: os.ErrPermission}
	st := newStoreWithKeyring(t.TempDir(), kr)

	err := st.Create(CreateParams{Name: "x", Type: "github", Value: "v"})
	if err == nil {
		t.Fatal("expected error when keychain fails")
	}
}
