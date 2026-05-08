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
	"errors"
	"testing"

	"github.com/devfile/alizer/pkg/apis/model"
)

func TestAlizerDetector_ReturnsLanguageNames(t *testing.T) {
	t.Parallel()

	det := newAlizerDetectorWithInjection(
		"/some/path",
		func(string) ([]model.Language, error) {
			return []model.Language{
				{Name: "Go", Weight: 90},
				{Name: "Python", Weight: 10},
			}, nil
		},
		func(string) ([]model.Component, error) {
			return nil, nil
		},
	)

	result, err := det.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(result.Languages))
	}
	if result.Languages[0] != "Go" || result.Languages[1] != "Python" {
		t.Errorf("unexpected languages: %v", result.Languages)
	}
}

func TestAlizerDetector_DeduplicatesPorts(t *testing.T) {
	t.Parallel()

	det := newAlizerDetectorWithInjection(
		"/some/path",
		func(string) ([]model.Language, error) {
			return nil, nil
		},
		func(string) ([]model.Component, error) {
			return []model.Component{
				{Name: "api", Ports: []int{8080, 3000}},
				{Name: "worker", Ports: []int{8080, 5000}},
			}, nil
		},
	)

	result, err := det.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 8080 appears in both components — must appear only once.
	portSet := make(map[int]int)
	for _, p := range result.Ports {
		portSet[p]++
	}
	for _, p := range []int{8080, 3000, 5000} {
		if portSet[p] != 1 {
			t.Errorf("port %d appears %d times, want 1", p, portSet[p])
		}
	}
	if len(result.Ports) != 3 {
		t.Errorf("expected 3 unique ports, got %d: %v", len(result.Ports), result.Ports)
	}
}

func TestAlizerDetector_EmptyResults(t *testing.T) {
	t.Parallel()

	det := newAlizerDetectorWithInjection(
		"/some/path",
		func(string) ([]model.Language, error) { return nil, nil },
		func(string) ([]model.Component, error) { return nil, nil },
	)

	result, err := det.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Languages) != 0 {
		t.Errorf("expected no languages, got %v", result.Languages)
	}
	if len(result.Ports) != 0 {
		t.Errorf("expected no ports, got %v", result.Ports)
	}
}

func TestAlizerDetector_AnalyzeError(t *testing.T) {
	t.Parallel()

	want := errors.New("analyze failed")
	det := newAlizerDetectorWithInjection(
		"/some/path",
		func(string) ([]model.Language, error) { return nil, want },
		func(string) ([]model.Component, error) { return nil, nil },
	)

	_, err := det.Detect()
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}

func TestAlizerDetector_ComponentsError(t *testing.T) {
	t.Parallel()

	want := errors.New("components failed")
	det := newAlizerDetectorWithInjection(
		"/some/path",
		func(string) ([]model.Language, error) { return nil, nil },
		func(string) ([]model.Component, error) { return nil, want },
	)

	_, err := det.Detect()
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}
