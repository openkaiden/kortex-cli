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

// Package credential provides a pluggable abstraction for file-based credentials
// that kdn intercepts when declared as workspace mounts. A placeholder file is
// substituted so the real secret never lands inside the container; actual auth
// flows through the OneCLI proxy.
package credential

import (
	"context"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
)

// Credential describes how a particular file-based credential is intercepted
// when declared as a workspace mount.
type Credential interface {
	// Name returns the unique identifier for this credential type (e.g. "gcloud", "openshift").
	Name() string

	// ContainerFilePath returns the absolute path inside the container at which
	// the placeholder file must be mounted.
	ContainerFilePath() string

	// Detect scans workspace mounts and returns the resolved host-side path to
	// the real credential file and the mount entry to intercept.
	// Returns ("", nil) when this credential is not declared or not applicable.
	Detect(mounts []workspace.Mount, homeDir string) (hostFilePath string, intercepted *workspace.Mount)

	// FakeFile returns the bytes to write as the placeholder credential file
	// that will be mounted into the container instead of the real one.
	// hostFilePath is the path to the real credential on the host.
	FakeFile(hostFilePath string) ([]byte, error)

	// Configure performs any OneCLI setup needed when this credential is active
	// (e.g. calling ConnectApp or creating secrets with the real credential).
	// hostFilePath is the path to the real credential on the host.
	Configure(ctx context.Context, client onecli.Client, hostFilePath string) error

	// HostPatterns returns host globs to add to the allow list in deny-mode
	// networking when this credential is active. hostFilePath lets dynamic
	// implementations extract the server URL from the real credential file.
	HostPatterns(hostFilePath string) []string
}

// Registry manages Credential implementations.
type Registry interface {
	// Register adds a credential implementation to the registry.
	// Returns an error if a credential with the same name is already registered.
	Register(c Credential) error

	// List returns all registered credentials.
	List() []Credential
}
