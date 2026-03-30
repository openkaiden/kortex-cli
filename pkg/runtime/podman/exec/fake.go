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
	"context"
	"io"
)

// FakeExecutor is a fake implementation of Executor for testing.
type FakeExecutor struct {
	// RunFunc is called when Run is invoked. If nil, Run returns nil.
	RunFunc func(ctx context.Context, args ...string) error

	// OutputFunc is called when Output is invoked. If nil, Output returns empty bytes.
	OutputFunc func(ctx context.Context, args ...string) ([]byte, error)

	// RunInteractiveFunc is called when RunInteractive is invoked. If nil, RunInteractive returns nil.
	RunInteractiveFunc func(ctx context.Context, args ...string) error

	// RunCalls tracks all calls to Run with their arguments.
	RunCalls [][]string

	// OutputCalls tracks all calls to Output with their arguments.
	OutputCalls [][]string

	// RunInteractiveCalls tracks all calls to RunInteractive with their arguments.
	RunInteractiveCalls [][]string
}

// Ensure FakeExecutor implements Executor at compile time.
var _ Executor = (*FakeExecutor)(nil)

// NewFake creates a new FakeExecutor.
func NewFake() *FakeExecutor {
	return &FakeExecutor{
		RunCalls:            make([][]string, 0),
		OutputCalls:         make([][]string, 0),
		RunInteractiveCalls: make([][]string, 0),
	}
}

// Run executes the RunFunc if set, otherwise returns nil.
func (f *FakeExecutor) Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	f.RunCalls = append(f.RunCalls, args)
	if f.RunFunc != nil {
		return f.RunFunc(ctx, args...)
	}
	return nil
}

// Output executes the OutputFunc if set, otherwise returns empty bytes.
func (f *FakeExecutor) Output(ctx context.Context, stderr io.Writer, args ...string) ([]byte, error) {
	f.OutputCalls = append(f.OutputCalls, args)
	if f.OutputFunc != nil {
		return f.OutputFunc(ctx, args...)
	}
	return []byte{}, nil
}

// RunInteractive executes the RunInteractiveFunc if set, otherwise returns nil.
func (f *FakeExecutor) RunInteractive(ctx context.Context, args ...string) error {
	f.RunInteractiveCalls = append(f.RunInteractiveCalls, args)
	if f.RunInteractiveFunc != nil {
		return f.RunInteractiveFunc(ctx, args...)
	}
	return nil
}

// AssertRunCalledWith checks if Run was called with the expected arguments.
func (f *FakeExecutor) AssertRunCalledWith(t interface {
	Errorf(format string, args ...interface{})
}, expectedArgs ...string) {
	for _, call := range f.RunCalls {
		if argsEqual(call, expectedArgs) {
			return
		}
	}
	t.Errorf("Expected Run to be called with %v, but it was called with: %v", expectedArgs, f.RunCalls)
}

// AssertOutputCalledWith checks if Output was called with the expected arguments.
func (f *FakeExecutor) AssertOutputCalledWith(t interface {
	Errorf(format string, args ...interface{})
}, expectedArgs ...string) {
	for _, call := range f.OutputCalls {
		if argsEqual(call, expectedArgs) {
			return
		}
	}
	t.Errorf("Expected Output to be called with %v, but it was called with: %v", expectedArgs, f.OutputCalls)
}

// AssertRunInteractiveCalledWith checks if RunInteractive was called with the expected arguments.
func (f *FakeExecutor) AssertRunInteractiveCalledWith(t interface {
	Errorf(format string, args ...interface{})
}, expectedArgs ...string) {
	for _, call := range f.RunInteractiveCalls {
		if argsEqual(call, expectedArgs) {
			return
		}
	}
	t.Errorf("Expected RunInteractive to be called with %v, but it was called with: %v", expectedArgs, f.RunInteractiveCalls)
}

// argsEqual compares two slices of strings for equality.
func argsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
