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

// Package secret provides interfaces and implementations for managing user secrets.
// Secret values are stored in the system keychain; non-sensitive metadata is persisted
// in a JSON file under the kdn storage directory.
package secret

// TypeOther is the secret type for custom secrets that require explicit host,
// path, header, and headerTemplate descriptors.
const TypeOther = "other"

// CreateParams holds all parameters needed to create a secret.
type CreateParams struct {
	Name           string
	Type           string
	Value          string
	Description    string
	Hosts          []string
	Path           string
	Header         string
	HeaderTemplate string
}

// Store manages persistent storage of secrets.
type Store interface {
	// Create stores the secret value in the system keychain and persists
	// the remaining metadata to the storage directory.
	Create(params CreateParams) error
}
