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

package logger

import "io"

// textLogger is an implementation of Logger that writes to the provided writers.
type textLogger struct {
	stdout io.Writer
	stderr io.Writer
}

// Compile-time check to ensure textLogger implements Logger interface.
var _ Logger = (*textLogger)(nil)

// NewTextLogger creates a new logger that writes to the given stdout and stderr writers.
// If either writer is nil, it defaults to io.Discard.
func NewTextLogger(stdout, stderr io.Writer) Logger {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &textLogger{
		stdout: stdout,
		stderr: stderr,
	}
}

// Stdout returns the writer for standard output.
func (t *textLogger) Stdout() io.Writer { return t.stdout }

// Stderr returns the writer for standard error.
func (t *textLogger) Stderr() io.Writer { return t.stderr }
