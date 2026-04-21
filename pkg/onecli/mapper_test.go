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
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

func registryWithGitHub(t *testing.T) secretservice.Registry {
	t.Helper()
	reg := secretservice.NewRegistry()
	svc := secretservice.NewSecretService(
		"github",
		"api.github.com",
		"",
		[]string{"GH_TOKEN", "GITHUB_TOKEN"},
		"Authorization",
		"Bearer ${value}",
	)
	if err := reg.Register(svc); err != nil {
		t.Fatal(err)
	}
	return reg
}

func strPtr(s string) *string { return &s }

func TestMapper_KnownType_GitHub(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(registryWithGitHub(t))
	secret := workspace.Secret{
		Type:  "github",
		Value: "ghp_abc123",
	}

	got, err := mapper.Map(secret)
	if err != nil {
		t.Fatalf("Map() error: %v", err)
	}

	if got.Name != "github" {
		t.Errorf("Name = %q, want %q", got.Name, "github")
	}
	if got.Type != "generic" {
		t.Errorf("Type = %q, want %q", got.Type, "generic")
	}
	if got.Value != "ghp_abc123" {
		t.Errorf("Value = %q, want %q", got.Value, "ghp_abc123")
	}
	if got.HostPattern != "api.github.com" {
		t.Errorf("HostPattern = %q, want %q", got.HostPattern, "api.github.com")
	}
	if got.InjectionConfig == nil {
		t.Fatal("InjectionConfig is nil")
	}
	if got.InjectionConfig.HeaderName != "Authorization" {
		t.Errorf("HeaderName = %q, want %q", got.InjectionConfig.HeaderName, "Authorization")
	}
	if got.InjectionConfig.ValueFormat != "Bearer {value}" {
		t.Errorf("ValueFormat = %q, want %q", got.InjectionConfig.ValueFormat, "Bearer {value}")
	}
}

func TestMapper_KnownType_WithName(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(registryWithGitHub(t))
	secret := workspace.Secret{
		Type:  "github",
		Value: "ghp_abc123",
		Name:  strPtr("my-gh-token"),
	}

	got, err := mapper.Map(secret)
	if err != nil {
		t.Fatalf("Map() error: %v", err)
	}
	if got.Name != "my-gh-token" {
		t.Errorf("Name = %q, want %q", got.Name, "my-gh-token")
	}
}

func TestMapper_UnknownType(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(secretservice.NewRegistry())
	secret := workspace.Secret{
		Type:  "unknown-service",
		Value: "token",
	}

	_, err := mapper.Map(secret)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestMapper_OtherType_AllFields(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(secretservice.NewRegistry())
	hosts := []string{"api.example.com"}
	secret := workspace.Secret{
		Type:           "other",
		Value:          "my-key-123",
		Name:           strPtr("custom-api"),
		Header:         strPtr("X-Api-Key"),
		HeaderTemplate: strPtr("Token ${value}"),
		Hosts:          &hosts,
		Path:           strPtr("/v2"),
	}

	got, err := mapper.Map(secret)
	if err != nil {
		t.Fatalf("Map() error: %v", err)
	}

	if got.Name != "custom-api" {
		t.Errorf("Name = %q, want %q", got.Name, "custom-api")
	}
	if got.Type != "generic" {
		t.Errorf("Type = %q, want %q", got.Type, "generic")
	}
	if got.Value != "my-key-123" {
		t.Errorf("Value = %q, want %q", got.Value, "my-key-123")
	}
	if got.HostPattern != "api.example.com" {
		t.Errorf("HostPattern = %q, want %q", got.HostPattern, "api.example.com")
	}
	if got.PathPattern != "/v2" {
		t.Errorf("PathPattern = %q, want %q", got.PathPattern, "/v2")
	}
	if got.InjectionConfig == nil {
		t.Fatal("InjectionConfig is nil")
	}
	if got.InjectionConfig.HeaderName != "X-Api-Key" {
		t.Errorf("HeaderName = %q, want %q", got.InjectionConfig.HeaderName, "X-Api-Key")
	}
	if got.InjectionConfig.ValueFormat != "Token {value}" {
		t.Errorf("ValueFormat = %q, want %q", got.InjectionConfig.ValueFormat, "Token {value}")
	}
}

func TestMapper_OtherType_MultipleHosts_Error(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(secretservice.NewRegistry())
	hosts := []string{"api.example.com", "api2.example.com"}
	secret := workspace.Secret{
		Type:  "other",
		Value: "my-key-123",
		Hosts: &hosts,
	}

	_, err := mapper.Map(secret)
	if err == nil {
		t.Fatal("expected error for multiple hosts, got nil")
	}
	if !strings.Contains(err.Error(), "one host per secret") {
		t.Errorf("error should mention 'one host per secret', got: %v", err)
	}
}

func TestMapper_OtherType_MinimalFields(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(secretservice.NewRegistry())
	secret := workspace.Secret{
		Type:  "other",
		Value: "secret-val",
	}

	got, err := mapper.Map(secret)
	if err != nil {
		t.Fatalf("Map() error: %v", err)
	}

	if got.Name != "other" {
		t.Errorf("Name = %q, want %q", got.Name, "other")
	}
	if got.HostPattern != "*" {
		t.Errorf("HostPattern = %q, want %q", got.HostPattern, "*")
	}
	if got.PathPattern != "" {
		t.Errorf("PathPattern = %q, want empty", got.PathPattern)
	}
	if got.InjectionConfig != nil {
		t.Errorf("InjectionConfig should be nil for other type without header, got %+v", got.InjectionConfig)
	}
}

func TestMapper_OtherType_EmptyHosts(t *testing.T) {
	t.Parallel()

	mapper := NewSecretMapper(secretservice.NewRegistry())
	emptyHosts := []string{}
	secret := workspace.Secret{
		Type:  "other",
		Value: "val",
		Hosts: &emptyHosts,
	}

	got, err := mapper.Map(secret)
	if err != nil {
		t.Fatalf("Map() error: %v", err)
	}
	if got.HostPattern != "*" {
		t.Errorf("HostPattern = %q, want %q for empty hosts", got.HostPattern, "*")
	}
}

func TestConvertTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Bearer ${value}", "Bearer {value}"},
		{"${value}", "{value}"},
		{"no-placeholder", "no-placeholder"},
		{"", ""},
		{"${value} and ${value}", "{value} and {value}"},
	}

	for _, tt := range tests {
		if got := convertTemplate(tt.input); got != tt.want {
			t.Errorf("convertTemplate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
