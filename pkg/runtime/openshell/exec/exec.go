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

// Package exec provides an abstraction for executing openshell commands.
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

// Executor provides an interface for executing openshell commands.
type Executor interface {
	// BinaryPath returns the path to the openshell binary.
	BinaryPath() string

	// Run executes an openshell command, writing stdout and stderr to the provided writers.
	Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error

	// Output executes an openshell command and returns its standard output.
	Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error)

	// RunInteractive executes an openshell command with stdin/stdout/stderr connected to the terminal.
	RunInteractive(ctx context.Context, args ...string) error
}

// executor is the default implementation of Executor.
type executor struct {
	binaryPath string
}

// Ensure executor implements Executor at compile time.
var _ Executor = (*executor)(nil)

// New creates a new Executor instance that runs the given binary.
func New(binaryPath string) Executor {
	return &executor{binaryPath: binaryPath}
}

// BinaryPath returns the path to the openshell binary.
func (e *executor) BinaryPath() string {
	return e.binaryPath
}

// Run executes an openshell command, writing stdout and stderr to the provided writers.
func (e *executor) Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	var stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return fmt.Errorf("%w\nopenshell stderr:\n%s", err, msg)
		}
		return err
	}
	return nil
}

// Output executes an openshell command and returns its standard output.
func (e *executor) Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error) {
	var stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return out, fmt.Errorf("%w\nopenshell stderr:\n%s", err, msg)
		}
		return out, err
	}
	return out, nil
}

// RunInteractive executes an openshell command with stdin/stdout/stderr connected to the terminal.
func (e *executor) RunInteractive(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
