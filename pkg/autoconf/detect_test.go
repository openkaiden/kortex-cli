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

package autoconf

import (
	"testing"

	"github.com/openkaiden/kdn/pkg/secretservice"
)

func makeService(name string, envVars []string) secretservice.SecretService {
	return secretservice.NewSecretService(name, nil, "", envVars, "", "", "")
}

func envLookup(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestDetect_NoEnvVars(t *testing.T) {
	t.Parallel()
	services := []secretservice.SecretService{
		makeService("github", []string{"GH_TOKEN", "GITHUB_TOKEN"}),
	}
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{}), services)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 {
		t.Errorf("expected no detections, got %v", got.NeedsAction)
	}
}

func TestDetect_OneService(t *testing.T) {
	t.Parallel()
	services := []secretservice.SecretService{
		makeService("anthropic", []string{"ANTHROPIC_API_KEY"}),
	}
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-abc",
	}), services)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got.NeedsAction))
	}
	if got.NeedsAction[0].ServiceName != "anthropic" || got.NeedsAction[0].EnvVarName != "ANTHROPIC_API_KEY" || got.NeedsAction[0].Value != "sk-ant-abc" {
		t.Errorf("unexpected detection: %+v", got.NeedsAction[0])
	}
}

func TestDetect_MultipleServices(t *testing.T) {
	t.Parallel()
	services := []secretservice.SecretService{
		makeService("anthropic", []string{"ANTHROPIC_API_KEY"}),
		makeService("github", []string{"GH_TOKEN", "GITHUB_TOKEN"}),
	}
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-abc",
		"GH_TOKEN":          "ghp_xyz",
	}), services)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got.NeedsAction))
	}
}

func TestDetect_FirstEnvVarWins(t *testing.T) {
	t.Parallel()
	services := []secretservice.SecretService{
		makeService("github", []string{"GH_TOKEN", "GITHUB_TOKEN"}),
	}
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{
		"GH_TOKEN":     "ghp_first",
		"GITHUB_TOKEN": "ghp_second",
	}), services)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got.NeedsAction))
	}
	if got.NeedsAction[0].EnvVarName != "GH_TOKEN" || got.NeedsAction[0].Value != "ghp_first" {
		t.Errorf("expected first env var to win, got %+v", got.NeedsAction[0])
	}
}

func TestDetect_EmptyEnvVarSkipped(t *testing.T) {
	t.Parallel()
	services := []secretservice.SecretService{
		makeService("github", []string{"GH_TOKEN", "GITHUB_TOKEN"}),
	}
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{
		"GH_TOKEN":     "",
		"GITHUB_TOKEN": "ghp_fallback",
	}), services)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got.NeedsAction))
	}
	if got.NeedsAction[0].EnvVarName != "GITHUB_TOKEN" || got.NeedsAction[0].Value != "ghp_fallback" {
		t.Errorf("expected fallback to GITHUB_TOKEN, got %+v", got.NeedsAction[0])
	}
}

func TestDetect_NoServices(t *testing.T) {
	t.Parallel()
	detector := newSecretDetectorWithLookup(envLookup(map[string]string{
		"GH_TOKEN": "ghp_xyz",
	}), nil)
	got, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NeedsAction) != 0 {
		t.Errorf("expected no detections for empty services, got %v", got.NeedsAction)
	}
}
