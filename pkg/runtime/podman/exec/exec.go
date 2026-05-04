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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Executor provides an interface for executing podman commands.
type Executor interface {
	// Run executes a podman command, writing stdout and stderr to the provided writers.
	// Returns an error if the command fails.
	Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error

	// Output executes a podman command and returns its standard output.
	// Stderr is written to the provided writer.
	// Returns an error if the command fails.
	Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error)

	// RunInteractive executes a podman command with stdin/stdout/stderr connected to the terminal.
	// This is used for interactive sessions where user input is required.
	// Returns an error if the command fails.
	RunInteractive(ctx context.Context, args ...string) error
}

// executor is the default implementation of Executor.
type executor struct {
	podmanPath string
}

// Ensure executor implements Executor at compile time.
var _ Executor = (*executor)(nil)

// New creates a new Executor instance using the given podman executable path.
// The path can be a bare name ("podman") or an absolute path ("/usr/bin/podman").
func New(podmanPath string) Executor {
	return &executor{podmanPath: podmanPath}
}

// Run executes a podman command, writing stdout and stderr to the provided writers.
// On failure, the error includes podman's stderr output for diagnostics,
// even when the caller passes io.Discard as the stderr writer.
func (e *executor) Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	var stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, e.podmanPath, args...)
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return fmt.Errorf("%w\nPodman stderr:\n%s", err, msg)
		}
		return err
	}
	return nil
}

// Output executes a podman command and returns its standard output.
// Stderr is written to the provided writer.
// On failure, the error includes podman's stderr output for diagnostics.
func (e *executor) Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error) {
	var stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, e.podmanPath, args...)
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return out, fmt.Errorf("%w\nPodman stderr:\n%s", err, msg)
		}
		return out, err
	}
	return out, nil
}

// RunInteractive executes a podman command with stdin/stdout/stderr connected to the terminal.
func (e *executor) RunInteractive(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, e.podmanPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
