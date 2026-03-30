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

// noopLogger is a no-op implementation of Logger that discards all output.
type noopLogger struct{}

// Compile-time check to ensure noopLogger implements Logger interface.
var _ Logger = (*noopLogger)(nil)

// NewNoOpLogger creates a new no-op logger.
func NewNoOpLogger() Logger {
	return &noopLogger{}
}

// Stdout returns a writer that discards all output.
func (n *noopLogger) Stdout() io.Writer { return io.Discard }

// Stderr returns a writer that discards all output.
func (n *noopLogger) Stderr() io.Writer { return io.Discard }
