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
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// homeConfigFileSpec defines a host home-directory config file to detect and auto-mount.
type homeConfigFileSpec struct {
	name string
	// hostRelPath is the path relative to $HOME on the host where the file lives.
	// It may differ from containerRelPath on Windows (e.g. "AppData/Roaming/tool/config"
	// vs ".config/tool/config" in the container). Always use forward slashes.
	hostRelPath string
	// containerRelPath is the path relative to $HOME inside the workspace container.
	// Containers are always Linux, so this is always a Unix-style forward-slash path.
	containerRelPath string
}

// registeredHomeConfigFiles lists the home config files to auto-detect.
// Add new entries here to extend the detection without any other code changes.
var registeredHomeConfigFiles = []homeConfigFileSpec{
	{name: "gitconfig", hostRelPath: ".gitconfig", containerRelPath: ".gitconfig"},
}

// DetectedHomeConfigFile holds information about a detected home config file.
type DetectedHomeConfigFile struct {
	// Name is the identifier for this config file (e.g. "gitconfig").
	Name string
	// HostPath is the mount source path expressed with the $HOME variable
	// (e.g. "$HOME/.gitconfig"). Portable across machines.
	HostPath string
	// ContainerPath is the mount target path inside the workspace container,
	// also expressed with $HOME (e.g. "$HOME/.gitconfig").
	ContainerPath string
}

// HomeConfigFilesDetector detects home directory config files that exist on the host.
type HomeConfigFilesDetector interface {
	// Detect returns the list of registered config files that exist in the host
	// home directory. Files that are absent are omitted from the result.
	Detect() ([]DetectedHomeConfigFile, error)
}

type envHomeConfigFilesDetector struct {
	homeDir  string
	statFile func(string) error
	specs    []homeConfigFileSpec
}

var _ HomeConfigFilesDetector = (*envHomeConfigFilesDetector)(nil)

// NewHomeConfigFilesDetector returns a HomeConfigFilesDetector that reads from
// the process environment. Returns an error only if the home directory cannot
// be determined.
func NewHomeConfigFilesDetector() (HomeConfigFilesDetector, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	return newHomeConfigFilesDetectorWithInjection(homeDir, func(p string) error {
		_, err := os.Stat(p)
		return err
	}, registeredHomeConfigFiles), nil
}

// newHomeConfigFilesDetectorWithInjection creates a detector with injectable
// dependencies for testing.
func newHomeConfigFilesDetectorWithInjection(homeDir string, statFile func(string) error, specs []homeConfigFileSpec) HomeConfigFilesDetector {
	return &envHomeConfigFilesDetector{
		homeDir:  homeDir,
		statFile: statFile,
		specs:    specs,
	}
}

// Detect returns the registered home config files that exist on the host.
func (d *envHomeConfigFilesDetector) Detect() ([]DetectedHomeConfigFile, error) {
	var detected []DetectedHomeConfigFile
	for _, spec := range d.specs {
		hostAbsPath := filepath.Join(d.homeDir, filepath.FromSlash(spec.hostRelPath))
		if err := d.statFile(hostAbsPath); err != nil {
			continue
		}
		detected = append(detected, DetectedHomeConfigFile{
			Name:          spec.name,
			HostPath:      path.Join("$HOME", spec.hostRelPath),
			ContainerPath: path.Join("$HOME", spec.containerRelPath),
		})
	}
	return detected, nil
}
