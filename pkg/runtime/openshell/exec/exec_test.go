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

package exec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func echoCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", "hello"}
	}
	return "echo", []string{"hello"}
}

func falseCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "exit", "1"}
	}
	return "false", nil
}

func stderrFailCommand(t *testing.T) (string, []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo error message >&2 & exit /b 1"}
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.sh")
	f, err := os.OpenFile(script, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("#!/bin/sh\necho 'error message' >&2\nexit 1\n")); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return "sh", []string{script}
}

func TestNew(t *testing.T) {
	t.Parallel()

	e := New("/usr/bin/openshell")
	if e.BinaryPath() != "/usr/bin/openshell" {
		t.Errorf("BinaryPath() = %q, want %q", e.BinaryPath(), "/usr/bin/openshell")
	}
}

func TestExecutor_Run_Success(t *testing.T) {
	t.Parallel()

	bin, args := echoCommand()
	e := New(bin)
	var stdout, stderr bytes.Buffer
	err := e.Run(context.Background(), &stdout, &stderr, args...)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Errorf("Expected stdout to contain 'hello', got %q", stdout.String())
	}
}

func TestExecutor_Run_Failure(t *testing.T) {
	t.Parallel()

	bin, args := falseCommand()
	e := New(bin)
	var stdout, stderr bytes.Buffer
	err := e.Run(context.Background(), &stdout, &stderr, args...)
	if err == nil {
		t.Error("Expected error from failing command")
	}
}

func TestExecutor_Run_NonexistentBinary(t *testing.T) {
	t.Parallel()

	e := New("/nonexistent/binary/path")
	var stdout, stderr bytes.Buffer
	err := e.Run(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Error("Expected error for nonexistent binary")
	}
}

func TestExecutor_Output_Success(t *testing.T) {
	t.Parallel()

	bin, args := echoCommand()
	e := New(bin)
	var stderr bytes.Buffer
	out, err := e.Output(context.Background(), &stderr, args...)
	if err != nil {
		t.Fatalf("Output() failed: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("Expected 'hello', got %q", out)
	}
}

func TestExecutor_Output_Failure(t *testing.T) {
	t.Parallel()

	bin, args := falseCommand()
	e := New(bin)
	var stderr bytes.Buffer
	_, err := e.Output(context.Background(), &stderr, args...)
	if err == nil {
		t.Error("Expected error from failing command")
	}
}

func TestExecutor_Output_NonexistentBinary(t *testing.T) {
	t.Parallel()

	e := New("/nonexistent/binary/path")
	var stderr bytes.Buffer
	_, err := e.Output(context.Background(), &stderr)
	if err == nil {
		t.Error("Expected error for nonexistent binary")
	}
}

func TestExecutor_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bin, _ := echoCommand()
	e := New(bin)
	var stdout, stderr bytes.Buffer
	err := e.Run(ctx, &stdout, &stderr, "test")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestFakeExecutor_BinaryPath(t *testing.T) {
	t.Parallel()

	f := NewFake()
	if f.BinaryPath() != "/fake/openshell" {
		t.Errorf("BinaryPath() = %q, want '/fake/openshell'", f.BinaryPath())
	}
}

func TestFakeExecutor_Run_Default(t *testing.T) {
	t.Parallel()

	f := NewFake()
	err := f.Run(context.Background(), nil, nil, "test", "args")
	if err != nil {
		t.Fatalf("Run() should succeed by default: %v", err)
	}

	if len(f.RunCalls) != 1 {
		t.Fatalf("Expected 1 RunCall, got %d", len(f.RunCalls))
	}
	if f.RunCalls[0][0] != "test" || f.RunCalls[0][1] != "args" {
		t.Errorf("Expected args [test args], got %v", f.RunCalls[0])
	}
}

func TestFakeExecutor_Output_Default(t *testing.T) {
	t.Parallel()

	f := NewFake()
	out, err := f.Output(context.Background(), nil, "sandbox", "list")
	if err != nil {
		t.Fatalf("Output() should succeed by default: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("Expected empty output, got %q", out)
	}

	if len(f.OutputCalls) != 1 {
		t.Fatalf("Expected 1 OutputCall, got %d", len(f.OutputCalls))
	}
}

func TestFakeExecutor_RunInteractive_Default(t *testing.T) {
	t.Parallel()

	f := NewFake()
	err := f.RunInteractive(context.Background(), "sandbox", "connect")
	if err != nil {
		t.Fatalf("RunInteractive() should succeed by default: %v", err)
	}

	if len(f.RunInteractiveCalls) != 1 {
		t.Fatalf("Expected 1 RunInteractiveCall, got %d", len(f.RunInteractiveCalls))
	}
}

func TestFakeExecutor_CustomFuncs(t *testing.T) {
	t.Parallel()

	f := NewFake()
	f.RunFunc = func(_ context.Context, args ...string) error {
		return nil
	}
	f.OutputFunc = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("custom output"), nil
	}
	f.RunInteractiveFunc = func(_ context.Context, args ...string) error {
		return nil
	}

	_ = f.Run(context.Background(), nil, nil, "test")
	out, _ := f.Output(context.Background(), nil, "test")
	_ = f.RunInteractive(context.Background(), "test")

	if string(out) != "custom output" {
		t.Errorf("Expected 'custom output', got %q", out)
	}
}

func TestFakeExecutor_TracksCalls(t *testing.T) {
	t.Parallel()

	f := NewFake()
	_ = f.Run(context.Background(), nil, nil, "cmd1")
	_ = f.Run(context.Background(), nil, nil, "cmd2")
	_, _ = f.Output(context.Background(), nil, "cmd3")
	_ = f.RunInteractive(context.Background(), "cmd4")

	if len(f.RunCalls) != 2 {
		t.Errorf("Expected 2 RunCalls, got %d", len(f.RunCalls))
	}
	if len(f.OutputCalls) != 1 {
		t.Errorf("Expected 1 OutputCall, got %d", len(f.OutputCalls))
	}
	if len(f.RunInteractiveCalls) != 1 {
		t.Errorf("Expected 1 RunInteractiveCall, got %d", len(f.RunInteractiveCalls))
	}
}

func TestExecutor_Run_StderrInError(t *testing.T) {
	t.Parallel()

	bin, args := stderrFailCommand(t)
	e := New(bin)
	var stdout, stderr bytes.Buffer
	err := e.Run(context.Background(), &stdout, &stderr, args...)
	if err == nil {
		t.Fatal("Expected error from failing script")
	}
	if !strings.Contains(err.Error(), "error message") {
		t.Errorf("Expected stderr in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "openshell stderr:") {
		t.Errorf("Expected 'openshell stderr:' prefix in error, got: %v", err)
	}
}

func TestExecutor_Output_StderrInError(t *testing.T) {
	t.Parallel()

	bin, args := stderrFailCommand(t)
	e := New(bin)
	var stderr bytes.Buffer
	_, err := e.Output(context.Background(), &stderr, args...)
	if err == nil {
		t.Fatal("Expected error from failing script")
	}
	if !strings.Contains(err.Error(), "error message") {
		t.Errorf("Expected stderr in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "openshell stderr:") {
		t.Errorf("Expected 'openshell stderr:' prefix in error, got: %v", err)
	}
}
