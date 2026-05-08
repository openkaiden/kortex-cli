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
	"github.com/devfile/alizer/pkg/apis/model"
	"github.com/devfile/alizer/pkg/apis/recognizer"
)

// AlizerResult holds the languages and ports detected by alizer in a source directory.
type AlizerResult struct {
	// Languages is the list of programming language names detected (e.g. "Go", "Python"),
	// ordered by weight (most prominent first).
	Languages []string

	// Ports is the deduplicated list of TCP port numbers detected across all components.
	Ports []int
}

// AlizerDetector detects programming languages and ports in a source directory.
type AlizerDetector interface {
	Detect() (AlizerResult, error)
}

type alizerDetector struct {
	path           string
	analyzeFunc    func(string) ([]model.Language, error)
	componentsFunc func(string) ([]model.Component, error)
}

var _ AlizerDetector = (*alizerDetector)(nil)

// NewAlizerDetector returns an AlizerDetector that analyzes the given directory.
func NewAlizerDetector(path string) AlizerDetector {
	return newAlizerDetectorWithInjection(path, recognizer.Analyze, recognizer.DetectComponents)
}

func newAlizerDetectorWithInjection(
	path string,
	analyzeFunc func(string) ([]model.Language, error),
	componentsFunc func(string) ([]model.Component, error),
) AlizerDetector {
	return &alizerDetector{
		path:           path,
		analyzeFunc:    analyzeFunc,
		componentsFunc: componentsFunc,
	}
}

// Detect runs alizer on the configured path and returns detected languages and ports.
func (d *alizerDetector) Detect() (AlizerResult, error) {
	languages, err := d.analyzeFunc(d.path)
	if err != nil {
		return AlizerResult{}, err
	}

	components, err := d.componentsFunc(d.path)
	if err != nil {
		return AlizerResult{}, err
	}

	var languageNames []string
	for _, lang := range languages {
		languageNames = append(languageNames, lang.Name)
	}

	seen := make(map[int]bool)
	var ports []int
	for _, comp := range components {
		for _, port := range comp.Ports {
			if !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}

	return AlizerResult{
		Languages: languageNames,
		Ports:     ports,
	}, nil
}
