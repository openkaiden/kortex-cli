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

import (
	"bytes"
	"io"
	"testing"
)

func TestNoOpLogger_DiscardOutput(t *testing.T) {
	t.Parallel()

	l := NewNoOpLogger()

	if l.Stdout() != io.Discard {
		t.Error("Expected noopLogger.Stdout() to return io.Discard")
	}

	if l.Stderr() != io.Discard {
		t.Error("Expected noopLogger.Stderr() to return io.Discard")
	}
}

func TestNoOpLogger_WritesAreDiscarded(t *testing.T) {
	t.Parallel()

	l := NewNoOpLogger()

	n, err := l.Stdout().Write([]byte("stdout data"))
	if err != nil {
		t.Errorf("Unexpected error writing to Stdout: %v", err)
	}
	if n != 11 {
		t.Errorf("Expected 11 bytes written, got %d", n)
	}

	n, err = l.Stderr().Write([]byte("stderr data"))
	if err != nil {
		t.Errorf("Unexpected error writing to Stderr: %v", err)
	}
	if n != 11 {
		t.Errorf("Expected 11 bytes written, got %d", n)
	}
}

func TestTextLogger_WritesToProviders(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	l := NewTextLogger(stdout, stderr)

	if _, err := l.Stdout().Write([]byte("stdout data")); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, err := l.Stderr().Write([]byte("stderr data")); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if stdout.String() != "stdout data" {
		t.Errorf("Expected stdout to contain 'stdout data', got: %q", stdout.String())
	}
	if stderr.String() != "stderr data" {
		t.Errorf("Expected stderr to contain 'stderr data', got: %q", stderr.String())
	}
}

func TestTextLogger_IndependentWriters(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	l := NewTextLogger(stdout, stderr)

	l.Stdout().Write([]byte("out"))
	l.Stderr().Write([]byte("err"))

	if stderr.String() == stdout.String() {
		t.Error("Expected stdout and stderr to be independent writers")
	}
}
