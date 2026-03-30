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

// Package logger provides interfaces and implementations for routing
// stdout/stderr output from runtime CLI commands to the user.
package logger

import "io"

// Logger provides writers for routing stdout and stderr of runtime CLI commands.
type Logger interface {
	// Stdout returns the writer for standard output from runtime commands.
	Stdout() io.Writer

	// Stderr returns the writer for standard error from runtime commands.
	Stderr() io.Writer
}
