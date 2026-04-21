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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiKeyPrefix = "oc_"

// CredentialProvider retrieves the OneCLI API key.
type CredentialProvider interface {
	// APIKey returns the OneCLI API key.
	APIKey(ctx context.Context) (string, error)
}

type credentialProvider struct {
	baseURL string
}

var _ CredentialProvider = (*credentialProvider)(nil)

// NewCredentialProvider creates a CredentialProvider that retrieves the API key
// from the OneCLI web service at the given base URL.
// In local mode, the first call bootstraps the local user and generates the key.
func NewCredentialProvider(baseURL string) CredentialProvider {
	return &credentialProvider{
		baseURL: baseURL,
	}
}

// APIKey retrieves the API key by calling GET /api/user/api-key.
// In local mode (no auth required), this bootstraps the local user on first call.
func (p *credentialProvider) APIKey(ctx context.Context) (string, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/user/api-key", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting API key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get API key (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding API key response: %w", err)
	}

	if !strings.HasPrefix(result.APIKey, apiKeyPrefix) {
		return "", fmt.Errorf("API key has unexpected format")
	}

	return result.APIKey, nil
}
