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

// Package credentialsetup provides centralized registration of all available
// credential implementations.
package credentialsetup

import (
	"fmt"

	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/credential/gcloud"
	"github.com/openkaiden/kdn/pkg/credential/kubeconfig"
)

// CredentialRegistrar is an interface for types that can register credentials.
// This is implemented by instances.Manager.
type CredentialRegistrar interface {
	RegisterCredential(c credential.Credential) error
}

// credentialFactory is a function that creates a new credential instance.
type credentialFactory func() credential.Credential

// availableCredentials lists all credential implementations to register.
var availableCredentials = []credentialFactory{
	gcloud.New,
	kubeconfig.New,
}

// RegisterAll registers all available credential implementations to the given registrar.
// Returns an error if any credential fails to register.
func RegisterAll(registrar CredentialRegistrar) error {
	return registerAllWithFactories(registrar, availableCredentials)
}

// registerAllWithFactories registers the given credentials to the registrar.
// This function is internal and used for testing with custom credential lists.
func registerAllWithFactories(registrar CredentialRegistrar, factories []credentialFactory) error {
	for _, factory := range factories {
		c := factory()
		if c == nil {
			return fmt.Errorf("credential factory returned nil")
		}
		if err := registrar.RegisterCredential(c); err != nil {
			return fmt.Errorf("failed to register credential %q: %w", c.Name(), err)
		}
	}
	return nil
}
