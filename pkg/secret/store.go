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
	"fmt"
	"os"
	"path/filepath"

	gokeyring "github.com/zalando/go-keyring"
)

// ErrSecretAlreadyExists is returned when a secret with the same name already exists.
var ErrSecretAlreadyExists = errors.New("secret already exists")

const (
	keyringService  = "kdn"
	secretsFileName = "secrets.json"
)

// keyring is an internal interface so tests can inject a fake implementation.
type keyring interface {
	Set(service, user, password string) error
}

// realKeyring delegates to the go-keyring library.
type realKeyring struct{}

var _ keyring = (*realKeyring)(nil)

func (r *realKeyring) Set(service, user, password string) error {
	return gokeyring.Set(service, user, password)
}

// store is the unexported implementation of Store.
type store struct {
	storageDir string
	kr         keyring
}

var _ Store = (*store)(nil)

// NewStore creates a Store backed by the system keychain and the given storage directory.
func NewStore(storageDir string) Store {
	return &store{storageDir: storageDir, kr: &realKeyring{}}
}

// newStoreWithKeyring creates a Store with an injectable keyring, used in tests.
func newStoreWithKeyring(storageDir string, kr keyring) Store {
	return &store{storageDir: storageDir, kr: kr}
}

// secretRecord is the JSON-serialisable metadata for a single secret.
type secretRecord struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Description    string   `json:"description,omitempty"`
	Hosts          []string `json:"hosts,omitempty"`
	Path           string   `json:"path,omitempty"`
	Header         string   `json:"header,omitempty"`
	HeaderTemplate string   `json:"headerTemplate,omitempty"`
}

type secretsFile struct {
	Secrets []secretRecord `json:"secrets"`
}

// Create stores the secret value in the system keychain then saves metadata.
// The duplicate check is performed before writing to the keychain so that an
// existing keychain entry is never overwritten when the name is already taken.
func (s *store) Create(params CreateParams) error {
	sf, err := s.loadSecretsFile()
	if err != nil {
		return err
	}

	for _, existing := range sf.Secrets {
		if existing.Name == params.Name {
			return fmt.Errorf("secret %q: %w", params.Name, ErrSecretAlreadyExists)
		}
	}

	if err := s.kr.Set(keyringService, params.Name, params.Value); err != nil {
		return fmt.Errorf("failed to store secret in keychain: %w", err)
	}

	return s.appendAndSave(sf, params)
}

// loadSecretsFile reads and parses secrets.json, returning an empty struct when
// the file does not yet exist.
func (s *store) loadSecretsFile() (secretsFile, error) {
	var sf secretsFile
	data, err := os.ReadFile(filepath.Join(s.storageDir, secretsFileName))
	if os.IsNotExist(err) {
		return sf, nil
	}
	if err != nil {
		return sf, fmt.Errorf("failed to read secrets file: %w", err)
	}
	if err := json.Unmarshal(data, &sf); err != nil {
		return sf, fmt.Errorf("failed to parse secrets file: %w", err)
	}
	return sf, nil
}

// appendAndSave appends the new record to sf and persists it to disk.
func (s *store) appendAndSave(sf secretsFile, params CreateParams) error {
	sf.Secrets = append(sf.Secrets, secretRecord{
		Name:           params.Name,
		Type:           params.Type,
		Description:    params.Description,
		Hosts:          params.Hosts,
		Path:           params.Path,
		Header:         params.Header,
		HeaderTemplate: params.HeaderTemplate,
	})

	jsonData, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	if err := os.MkdirAll(s.storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(s.storageDir, secretsFileName), jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}
