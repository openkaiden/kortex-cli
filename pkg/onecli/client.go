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

// Package onecli provides a client for the OneCLI API and utilities for
// mapping workspace secrets to OneCLI secret definitions.
package onecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the contract for interacting with the OneCLI API.
type Client interface {
	CreateSecret(ctx context.Context, input CreateSecretInput) (*Secret, error)
	UpdateSecret(ctx context.Context, id string, input UpdateSecretInput) error
	ListSecrets(ctx context.Context) ([]Secret, error)
	DeleteSecret(ctx context.Context, id string) error
	GetContainerConfig(ctx context.Context) (*ContainerConfig, error)
}

// UpdateSecretInput is the request body for updating a secret.
type UpdateSecretInput struct {
	Value           *string          `json:"value,omitempty"`
	HostPattern     *string          `json:"hostPattern,omitempty"`
	PathPattern     *string          `json:"pathPattern,omitempty"`
	InjectionConfig *InjectionConfig `json:"injectionConfig,omitempty"`
}

// ContainerConfig holds the environment variables and CA certificate returned
// by the OneCLI /api/container-config endpoint.
type ContainerConfig struct {
	Env                        map[string]string `json:"env"`
	CACertificate              string            `json:"caCertificate"`
	CACertificateContainerPath string            `json:"caCertificateContainerPath"`
}

// Secret represents a secret returned by the OneCLI API.
type Secret struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	HostPattern     string           `json:"hostPattern"`
	PathPattern     *string          `json:"pathPattern"`
	InjectionConfig *InjectionConfig `json:"injectionConfig"`
	CreatedAt       string           `json:"createdAt"`
}

// InjectionConfig describes how a secret is injected into HTTP requests.
type InjectionConfig struct {
	HeaderName  string `json:"headerName"`
	ValueFormat string `json:"valueFormat,omitempty"`
}

// CreateSecretInput is the request body for creating a secret.
type CreateSecretInput struct {
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Value           string           `json:"value"`
	HostPattern     string           `json:"hostPattern"`
	PathPattern     string           `json:"pathPattern,omitempty"`
	InjectionConfig *InjectionConfig `json:"injectionConfig,omitempty"`
}

// APIError represents an error response from the OneCLI API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

var _ Client = (*client)(nil)

// NewClient creates a new OneCLI API client.
func NewClient(baseURL, apiKey string) Client {
	return &client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateSecret creates a new secret in OneCLI.
func (c *client) CreateSecret(ctx context.Context, input CreateSecretInput) (*Secret, error) {
	var secret Secret
	if err := c.do(ctx, http.MethodPost, "/api/secrets", input, &secret); err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}
	return &secret, nil
}

// ListSecrets returns all secrets for the authenticated user.
func (c *client) ListSecrets(ctx context.Context) ([]Secret, error) {
	var secrets []Secret
	if err := c.do(ctx, http.MethodGet, "/api/secrets", nil, &secrets); err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	return secrets, nil
}

// UpdateSecret updates an existing secret by ID.
func (c *client) UpdateSecret(ctx context.Context, id string, input UpdateSecretInput) error {
	if err := c.do(ctx, http.MethodPatch, "/api/secrets/"+id, input, nil); err != nil {
		return fmt.Errorf("updating secret: %w", err)
	}
	return nil
}

// DeleteSecret deletes a secret by ID.
func (c *client) DeleteSecret(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, "/api/secrets/"+id, nil, nil); err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}
	return nil
}

// GetContainerConfig returns the proxy environment variables, CA certificate, and
// agent access token needed to configure a workspace container.
func (c *client) GetContainerConfig(ctx context.Context) (*ContainerConfig, error) {
	var cfg ContainerConfig
	if err := c.do(ctx, http.MethodGet, "/api/container-config", nil, &cfg); err != nil {
		return nil, fmt.Errorf("getting container config: %w", err)
	}
	return &cfg, nil
}

func (c *client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
		}
		return &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
