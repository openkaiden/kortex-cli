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
	"fmt"
	"strings"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

const secretTypeOther = "other"

// SecretMapper converts workspace secrets to OneCLI CreateSecretInput values.
type SecretMapper interface {
	Map(secret workspace.Secret) (CreateSecretInput, error)
}

type secretMapper struct {
	registry secretservice.Registry
}

var _ SecretMapper = (*secretMapper)(nil)

// NewSecretMapper creates a SecretMapper that uses the given registry to look up
// secret service metadata for known secret types.
func NewSecretMapper(registry secretservice.Registry) SecretMapper {
	return &secretMapper{registry: registry}
}

// Map converts a workspace secret to a CreateSecretInput.
// For type "other", the secret's own fields are used directly.
// For all other types, the SecretService registry provides host pattern, header, and template.
func (m *secretMapper) Map(secret workspace.Secret) (CreateSecretInput, error) {
	if secret.Type == secretTypeOther {
		return m.mapOtherSecret(secret)
	}
	return m.mapKnownSecret(secret)
}

func (m *secretMapper) mapKnownSecret(secret workspace.Secret) (CreateSecretInput, error) {
	svc, err := m.registry.Get(secret.Type)
	if err != nil {
		return CreateSecretInput{}, fmt.Errorf("unknown secret type %q: %w", secret.Type, err)
	}

	input := CreateSecretInput{
		Name:        secretName(secret.Name, secret.Type),
		Type:        "generic",
		Value:       secret.Value,
		HostPattern: svc.HostPattern(),
		PathPattern: svc.Path(),
	}

	if headerName := svc.HeaderName(); headerName != "" {
		input.InjectionConfig = &InjectionConfig{
			HeaderName:  headerName,
			ValueFormat: convertTemplate(svc.HeaderTemplate()),
		}
	}

	return input, nil
}

func (m *secretMapper) mapOtherSecret(secret workspace.Secret) (CreateSecretInput, error) {
	if secret.Hosts != nil && len(*secret.Hosts) > 1 {
		return CreateSecretInput{}, fmt.Errorf("secret type %q supports only one host per secret; declare one secret per host (got %d hosts)", secretTypeOther, len(*secret.Hosts))
	}

	input := CreateSecretInput{
		Name:        secretName(secret.Name, secretTypeOther),
		Type:        "generic",
		Value:       secret.Value,
		HostPattern: otherHostPattern(secret.Hosts),
		PathPattern: derefString(secret.Path),
	}

	if header := derefString(secret.Header); header != "" {
		input.InjectionConfig = &InjectionConfig{
			HeaderName:  header,
			ValueFormat: convertTemplate(derefString(secret.HeaderTemplate)),
		}
	}

	return input, nil
}

func secretName(name *string, fallback string) string {
	if name != nil && *name != "" {
		return *name
	}
	return fallback
}

func otherHostPattern(hosts *[]string) string {
	if hosts != nil && len(*hosts) > 0 {
		return (*hosts)[0]
	}
	return "*"
}

func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// convertTemplate converts kdn's ${value} placeholder to OneCLI's {value} format.
func convertTemplate(tmpl string) string {
	return strings.ReplaceAll(tmpl, "${value}", "{value}")
}
