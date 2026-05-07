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
	"os"
	"path/filepath"
	"testing"
)

func TestHomeConfigFilesDetector_FileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte("[user]\n\tname = Test\n"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	specs := []homeConfigFileSpec{
		{name: "gitconfig", hostRelPath: ".gitconfig", containerRelPath: ".gitconfig"},
	}
	d := newHomeConfigFilesDetectorWithInjection(dir, func(p string) error {
		_, err := os.Stat(p)
		return err
	}, specs)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected) != 1 {
		t.Fatalf("expected 1 detected file, got %d", len(detected))
	}
	f := detected[0]
	if f.Name != "gitconfig" {
		t.Errorf("Name = %q, want %q", f.Name, "gitconfig")
	}
	if f.HostPath != "$HOME/.gitconfig" {
		t.Errorf("HostPath = %q, want %q", f.HostPath, "$HOME/.gitconfig")
	}
	if f.ContainerPath != "$HOME/.gitconfig" {
		t.Errorf("ContainerPath = %q, want %q", f.ContainerPath, "$HOME/.gitconfig")
	}
}

func TestHomeConfigFilesDetector_FileAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specs := []homeConfigFileSpec{
		{name: "gitconfig", hostRelPath: ".gitconfig", containerRelPath: ".gitconfig"},
	}
	d := newHomeConfigFilesDetectorWithInjection(dir, func(p string) error {
		_, err := os.Stat(p)
		return err
	}, specs)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected) != 0 {
		t.Errorf("expected 0 detected files, got %d: %v", len(detected), detected)
	}
}

func TestHomeConfigFilesDetector_MultipleSpecs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Only create .gitconfig, not .npmrc
	if err := os.WriteFile(filepath.Join(dir, ".gitconfig"), []byte(""), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	specs := []homeConfigFileSpec{
		{name: "gitconfig", hostRelPath: ".gitconfig", containerRelPath: ".gitconfig"},
		{name: "npmrc", hostRelPath: ".npmrc", containerRelPath: ".npmrc"},
	}
	d := newHomeConfigFilesDetectorWithInjection(dir, func(p string) error {
		_, err := os.Stat(p)
		return err
	}, specs)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected) != 1 {
		t.Fatalf("expected 1 detected file, got %d", len(detected))
	}
	if detected[0].Name != "gitconfig" {
		t.Errorf("Name = %q, want %q", detected[0].Name, "gitconfig")
	}
}

func TestHomeConfigFilesDetector_StatError(t *testing.T) {
	t.Parallel()

	statErr := errors.New("permission denied")
	specs := []homeConfigFileSpec{
		{name: "gitconfig", hostRelPath: ".gitconfig", containerRelPath: ".gitconfig"},
	}
	d := newHomeConfigFilesDetectorWithInjection("/home/user", func(string) error {
		return statErr
	}, specs)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	// stat errors (including permission denied) are treated as "not found" — skip silently
	if len(detected) != 0 {
		t.Errorf("expected 0 detected files on stat error, got %d", len(detected))
	}
}

func TestHomeConfigFilesDetector_EmptySpecs(t *testing.T) {
	t.Parallel()

	d := newHomeConfigFilesDetectorWithInjection("/home/user", func(string) error {
		return nil
	}, nil)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected) != 0 {
		t.Errorf("expected 0 detected files for empty specs, got %d", len(detected))
	}
}

func TestHomeConfigFilesDetector_DivergentHostAndContainerPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Simulate a Windows-style host path that differs from the container path.
	hostRelPath := "AppData/Roaming/tool/config"
	containerRelPath := ".config/tool/config"
	if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(hostRelPath[:len(hostRelPath)-len("/config")])), 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(hostRelPath)), []byte(""), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	specs := []homeConfigFileSpec{
		{name: "tool", hostRelPath: hostRelPath, containerRelPath: containerRelPath},
	}
	d := newHomeConfigFilesDetectorWithInjection(dir, func(p string) error {
		_, err := os.Stat(p)
		return err
	}, specs)

	detected, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected) != 1 {
		t.Fatalf("expected 1 detected file, got %d", len(detected))
	}
	f := detected[0]
	if f.HostPath != "$HOME/AppData/Roaming/tool/config" {
		t.Errorf("HostPath = %q, want %q", f.HostPath, "$HOME/AppData/Roaming/tool/config")
	}
	if f.ContainerPath != "$HOME/.config/tool/config" {
		t.Errorf("ContainerPath = %q, want %q", f.ContainerPath, "$HOME/.config/tool/config")
	}
}

func TestRegisteredHomeConfigFiles_ContainsGitconfig(t *testing.T) {
	t.Parallel()

	found := false
	for _, spec := range registeredHomeConfigFiles {
		if spec.name == "gitconfig" && spec.hostRelPath == ".gitconfig" && spec.containerRelPath == ".gitconfig" {
			found = true
			break
		}
	}
	if !found {
		t.Error("registeredHomeConfigFiles does not contain gitconfig spec")
	}
}
