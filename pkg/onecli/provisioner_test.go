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
	"fmt"
	"net/http"
	"testing"
)

type fakeClient struct {
	created    []CreateSecretInput
	updated    []UpdateSecretInput
	updatedIDs []string
	existing   []Secret
	createErr  error
}

// Ensure fakeClient implements Client at compile time.
var _ Client = (*fakeClient)(nil)

func (f *fakeClient) CreateSecret(_ context.Context, input CreateSecretInput) (*Secret, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = append(f.created, input)
	return &Secret{ID: "fake-id", Name: input.Name}, nil
}

func (f *fakeClient) UpdateSecret(_ context.Context, id string, input UpdateSecretInput) error {
	f.updatedIDs = append(f.updatedIDs, id)
	f.updated = append(f.updated, input)
	return nil
}

func (f *fakeClient) ListSecrets(_ context.Context) ([]Secret, error) {
	return f.existing, nil
}

func (f *fakeClient) DeleteSecret(_ context.Context, _ string) error {
	return nil
}

func (f *fakeClient) GetContainerConfig(_ context.Context) (*ContainerConfig, error) {
	return &ContainerConfig{Env: map[string]string{}}, nil
}

func TestProvisioner_AllSecretsCreated(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{}
	prov := NewSecretProvisioner(fc)

	secrets := []CreateSecretInput{
		{Name: "secret-1", Type: "generic", Value: "val1", HostPattern: "a.com"},
		{Name: "secret-2", Type: "generic", Value: "val2", HostPattern: "b.com"},
	}

	if err := prov.ProvisionSecrets(context.Background(), secrets); err != nil {
		t.Fatalf("ProvisionSecrets() error: %v", err)
	}
	if len(fc.created) != 2 {
		t.Fatalf("created %d secrets, want 2", len(fc.created))
	}
}

func TestProvisioner_EmptySlice(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{}
	prov := NewSecretProvisioner(fc)

	if err := prov.ProvisionSecrets(context.Background(), nil); err != nil {
		t.Fatalf("ProvisionSecrets() error: %v", err)
	}
	if len(fc.created) != 0 {
		t.Errorf("created %d secrets for nil input, want 0", len(fc.created))
	}
}

func TestProvisioner_ConflictUpdatesExisting(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{
		createErr: &APIError{StatusCode: http.StatusConflict, Message: "already exists"},
		existing: []Secret{
			{ID: "sec-42", Name: "existing", Type: "generic", HostPattern: "old.com"},
		},
	}
	prov := NewSecretProvisioner(fc)

	secrets := []CreateSecretInput{
		{Name: "existing", Type: "generic", Value: "new-val", HostPattern: "new.com"},
	}

	if err := prov.ProvisionSecrets(context.Background(), secrets); err != nil {
		t.Fatalf("ProvisionSecrets() error: %v", err)
	}
	if len(fc.updatedIDs) != 1 {
		t.Fatalf("expected 1 update, got %d", len(fc.updatedIDs))
	}
	if fc.updatedIDs[0] != "sec-42" {
		t.Errorf("updated ID = %q, want %q", fc.updatedIDs[0], "sec-42")
	}
	if *fc.updated[0].Value != "new-val" {
		t.Errorf("updated value = %q, want %q", *fc.updated[0].Value, "new-val")
	}
	if *fc.updated[0].HostPattern != "new.com" {
		t.Errorf("updated host = %q, want %q", *fc.updated[0].HostPattern, "new.com")
	}
}

func TestProvisioner_ConflictSecretNotFound(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{
		createErr: &APIError{StatusCode: http.StatusConflict, Message: "already exists"},
		existing:  []Secret{},
	}
	prov := NewSecretProvisioner(fc)

	secrets := []CreateSecretInput{
		{Name: "ghost", Type: "generic", Value: "val", HostPattern: "a.com"},
	}

	err := prov.ProvisionSecrets(context.Background(), secrets)
	if err == nil {
		t.Fatal("expected error when existing secret not found after 409")
	}
}

func TestProvisioner_OtherErrorPropagates(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{
		createErr: &APIError{StatusCode: http.StatusUnauthorized, Message: "bad key"},
	}
	prov := NewSecretProvisioner(fc)

	secrets := []CreateSecretInput{
		{Name: "test", Type: "generic", Value: "val", HostPattern: "a.com"},
	}

	err := prov.ProvisionSecrets(context.Background(), secrets)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestProvisioner_NonAPIErrorPropagates(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{
		createErr: fmt.Errorf("network failure"),
	}
	prov := NewSecretProvisioner(fc)

	secrets := []CreateSecretInput{
		{Name: "test", Type: "generic", Value: "val", HostPattern: "a.com"},
	}

	err := prov.ProvisionSecrets(context.Background(), secrets)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}
