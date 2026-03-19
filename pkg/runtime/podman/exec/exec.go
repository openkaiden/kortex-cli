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

// Package exec provides an abstraction for executing podman commands.
package exec

import (
	"context"
	"os/exec"
)

// Executor provides an interface for executing podman commands.
type Executor interface {
	// Run executes a podman command and waits for it to complete.
	// Returns an error if the command fails.
	Run(ctx context.Context, args ...string) error

	// Output executes a podman command and returns its standard output.
	// Returns an error if the command fails.
	Output(ctx context.Context, args ...string) ([]byte, error)
}

// executor is the default implementation of Executor.
type executor struct{}

// Ensure executor implements Executor at compile time.
var _ Executor = (*executor)(nil)

// New creates a new Executor instance.
func New() Executor {
	return &executor{}
}

// Run executes a podman command and waits for it to complete.
func (e *executor) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "podman", args...)
	return cmd.Run()
}

// Output executes a podman command and returns its standard output.
func (e *executor) Output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "podman", args...)
	return cmd.Output()
}
