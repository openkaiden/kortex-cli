// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package system provides utilities for system-level operations.
package system

import (
	"os/exec"
)

// System provides an interface for system-level operations.
type System interface {
	// CommandExists checks if a command is available in the system PATH.
	CommandExists(name string) bool
	// Getuid returns the numeric user ID of the caller.
	Getuid() int
	// Getgid returns the numeric group ID of the caller.
	Getgid() int
}

// systemImpl is the default implementation of System.
type systemImpl struct{}

// Ensure systemImpl implements System at compile time.
var _ System = (*systemImpl)(nil)

// New creates a new System instance.
func New() System {
	return &systemImpl{}
}

// CommandExists checks if a command is available in the system PATH.
func (s *systemImpl) CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
