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
	"errors"
	"fmt"
	"net/http"
)

// SecretProvisioner creates or updates OneCLI secrets from a list of pre-mapped CreateSecretInput values.
type SecretProvisioner interface {
	ProvisionSecrets(ctx context.Context, secrets []CreateSecretInput) error
}

type secretProvisioner struct {
	client Client
}

var _ SecretProvisioner = (*secretProvisioner)(nil)

// NewSecretProvisioner creates a SecretProvisioner that uses the given client to create secrets.
func NewSecretProvisioner(client Client) SecretProvisioner {
	return &secretProvisioner{client: client}
}

// ProvisionSecrets creates each secret via the OneCLI API.
// If a secret already exists (409 conflict), it is updated with the new values.
func (p *secretProvisioner) ProvisionSecrets(ctx context.Context, secrets []CreateSecretInput) error {
	for i, input := range secrets {
		if _, err := p.client.CreateSecret(ctx, input); err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
				if updateErr := p.updateExisting(ctx, input); updateErr != nil {
					return fmt.Errorf("failed to update existing secret %d (%q): %w", i, input.Name, updateErr)
				}
				continue
			}
			return fmt.Errorf("failed to create secret %d (%q): %w", i, input.Name, err)
		}
	}
	return nil
}

// updateExisting finds the existing secret by name and updates it.
func (p *secretProvisioner) updateExisting(ctx context.Context, input CreateSecretInput) error {
	existing, err := p.client.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("listing secrets to find existing: %w", err)
	}

	for _, s := range existing {
		if s.Name == input.Name {
			update := UpdateSecretInput{
				Value:       &input.Value,
				HostPattern: &input.HostPattern,
			}
			if input.PathPattern != "" {
				update.PathPattern = &input.PathPattern
			}
			if input.InjectionConfig != nil {
				update.InjectionConfig = input.InjectionConfig
			}
			return p.client.UpdateSecret(ctx, s.ID, update)
		}
	}

	return fmt.Errorf("secret %q not found after 409 conflict", input.Name)
}
