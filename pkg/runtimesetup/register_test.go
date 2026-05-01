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

package runtimesetup

import (
	"context"
	"fmt"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime"
)

// fakeRegistrar is a test implementation of Registrar
type fakeRegistrar struct {
	registered []runtime.Runtime
	failNext   bool
}

func (f *fakeRegistrar) RegisterRuntime(rt runtime.Runtime) error {
	if f.failNext {
		f.failNext = false
		return runtime.ErrRuntimeNotFound // reusing an error for testing
	}
	f.registered = append(f.registered, rt)
	return nil
}

// testRuntime is a simple test runtime implementation
type testRuntime struct {
	runtimeType string
	available   bool
}

func (t *testRuntime) Type() string { return t.runtimeType }

func (t *testRuntime) WorkspaceSourcesPath() string { return "/workspace/sources" }

func (t *testRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}

func (t *testRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}

func (t *testRuntime) Stop(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (t *testRuntime) Remove(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (t *testRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return runtime.RuntimeInfo{}, fmt.Errorf("not implemented")
}

func (t *testRuntime) Available() bool {
	return t.available
}

// testRuntimeWithFlags wraps testRuntime and implements runtime.FlagProvider.
type testRuntimeWithFlags struct {
	*testRuntime
	flags []runtime.FlagDef
}

func (t *testRuntimeWithFlags) Flags() []runtime.FlagDef {
	return t.flags
}

// testRuntimeWithDashboard wraps testRuntime and implements runtime.Dashboard.
type testRuntimeWithDashboard struct {
	*testRuntime
}

func (t *testRuntimeWithDashboard) GetURL(_ context.Context, _ string) (string, error) {
	return "http://localhost:8888", nil
}

func TestListDashboardRuntimeTypesWithFactories(t *testing.T) {
	t.Parallel()

	t.Run("returns type names of runtimes that implement Dashboard", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithDashboard{&testRuntime{runtimeType: "dashboard-rt", available: true}}
			},
			func() runtime.Runtime {
				return &testRuntime{runtimeType: "no-dashboard-rt", available: true}
			},
		}

		types, err := listDashboardRuntimeTypesWithFactories(storageDir, factories)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(types) != 1 {
			t.Fatalf("expected 1 type, got %d: %v", len(types), types)
		}
		if types[0] != "dashboard-rt" {
			t.Errorf("expected 'dashboard-rt', got %q", types[0])
		}
	})

	t.Run("returns empty when no runtimes implement Dashboard", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntime{runtimeType: "no-dashboard-rt", available: true}
			},
		}

		types, err := listDashboardRuntimeTypesWithFactories(storageDir, factories)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(types) != 0 {
			t.Errorf("expected 0 types, got %d: %v", len(types), types)
		}
	})

	t.Run("skips unavailable runtimes even if they implement Dashboard", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithDashboard{&testRuntime{runtimeType: "dashboard-rt", available: false}}
			},
		}

		types, err := listDashboardRuntimeTypesWithFactories(storageDir, factories)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(types) != 0 {
			t.Errorf("expected 0 types (runtime unavailable), got %d: %v", len(types), types)
		}
	})
}

func TestRegisterAll(t *testing.T) {
	t.Parallel()

	t.Run("registers all runtimes", func(t *testing.T) {
		t.Parallel()

		registrar := &fakeRegistrar{}

		// Create test runtimes
		testFactories := []runtimeFactory{
			func() runtime.Runtime { return &testRuntime{runtimeType: "test1", available: true} },
			func() runtime.Runtime { return &testRuntime{runtimeType: "test2", available: true} },
		}

		err := registerAllWithAvailable(registrar, testFactories)
		if err != nil {
			t.Fatalf("registerAllWithAvailable() failed: %v", err)
		}

		// We should have registered 2 test runtimes
		if len(registrar.registered) != 2 {
			t.Errorf("Expected 2 runtimes to be registered, got %d", len(registrar.registered))
		}

		// Check that both types are present
		types := make(map[string]bool)
		for _, rt := range registrar.registered {
			types[rt.Type()] = true
		}

		if !types["test1"] {
			t.Error("Expected 'test1' runtime to be registered")
		}
		if !types["test2"] {
			t.Error("Expected 'test2' runtime to be registered")
		}
	})

	t.Run("skips unavailable runtimes", func(t *testing.T) {
		t.Parallel()

		registrar := &fakeRegistrar{}

		// Create test runtimes with one unavailable
		testFactories := []runtimeFactory{
			func() runtime.Runtime { return &testRuntime{runtimeType: "test1", available: true} },
			func() runtime.Runtime { return &testRuntime{runtimeType: "test2", available: false} },
			func() runtime.Runtime { return &testRuntime{runtimeType: "test3", available: true} },
		}

		err := registerAllWithAvailable(registrar, testFactories)
		if err != nil {
			t.Fatalf("registerAllWithAvailable() failed: %v", err)
		}

		// We should have registered only 2 runtimes (test2 is unavailable)
		if len(registrar.registered) != 2 {
			t.Errorf("Expected 2 runtimes to be registered, got %d", len(registrar.registered))
		}

		// Check that only available runtimes are present
		types := make(map[string]bool)
		for _, rt := range registrar.registered {
			types[rt.Type()] = true
		}

		if !types["test1"] {
			t.Error("Expected 'test1' runtime to be registered")
		}
		if types["test2"] {
			t.Error("Did not expect 'test2' runtime to be registered (unavailable)")
		}
		if !types["test3"] {
			t.Error("Expected 'test3' runtime to be registered")
		}
	})

	t.Run("returns error on registration failure", func(t *testing.T) {
		t.Parallel()

		registrar := &fakeRegistrar{failNext: true}

		testFactories := []runtimeFactory{
			func() runtime.Runtime { return &testRuntime{runtimeType: "test1", available: true} },
		}

		err := registerAllWithAvailable(registrar, testFactories)
		if err == nil {
			t.Fatal("Expected error when registration fails, got nil")
		}
	})
}

func TestListFlags(t *testing.T) {
	t.Parallel()

	// ListFlags delegates to listFlagsWithFactories with the real availableRuntimes.
	// Since no real runtime currently implements FlagProvider, it should return empty.
	flags := ListFlags()
	if len(flags) != 0 {
		t.Errorf("expected 0 flags from real runtimes, got %d", len(flags))
	}
}

func TestListFlagsWithFactories(t *testing.T) {
	t.Parallel()

	t.Run("returns flags from FlagProvider runtimes", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "flagged-rt", available: true},
					flags: []runtime.FlagDef{
						{Name: "my-flag", Usage: "a test flag", Completions: []string{"a", "b"}},
					},
				}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 1 {
			t.Fatalf("expected 1 flag, got %d", len(flags))
		}
		if flags[0].Name != "my-flag" {
			t.Errorf("expected flag name 'my-flag', got %q", flags[0].Name)
		}
		if flags[0].Usage != "a test flag" {
			t.Errorf("expected usage 'a test flag', got %q", flags[0].Usage)
		}
		if len(flags[0].Completions) != 2 {
			t.Errorf("expected 2 completions, got %d", len(flags[0].Completions))
		}
	})

	t.Run("skips runtimes without FlagProvider", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntime{runtimeType: "plain-rt", available: true}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 0 {
			t.Errorf("expected 0 flags, got %d", len(flags))
		}
	})

	t.Run("skips unavailable runtimes", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "unavail-rt", available: false},
					flags: []runtime.FlagDef{
						{Name: "hidden-flag", Usage: "should not appear"},
					},
				}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 0 {
			t.Errorf("expected 0 flags (runtime unavailable), got %d", len(flags))
		}
	})

	t.Run("skips fake runtime", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "fake", available: true},
					flags: []runtime.FlagDef{
						{Name: "fake-flag", Usage: "should not appear"},
					},
				}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 0 {
			t.Errorf("expected 0 flags (fake runtime), got %d", len(flags))
		}
	})

	t.Run("deduplicates flags by name across runtimes", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "rt1", available: true},
					flags: []runtime.FlagDef{
						{Name: "shared-flag", Usage: "from rt1"},
					},
				}
			},
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "rt2", available: true},
					flags: []runtime.FlagDef{
						{Name: "shared-flag", Usage: "from rt2"},
						{Name: "unique-flag", Usage: "only in rt2"},
					},
				}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 2 {
			t.Fatalf("expected 2 flags (deduplicated), got %d", len(flags))
		}

		names := make(map[string]bool)
		for _, f := range flags {
			names[f.Name] = true
		}
		if !names["shared-flag"] {
			t.Error("expected 'shared-flag' to be present")
		}
		if !names["unique-flag"] {
			t.Error("expected 'unique-flag' to be present")
		}
	})

	t.Run("collects flags from multiple runtimes", func(t *testing.T) {
		t.Parallel()

		factories := []runtimeFactory{
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "rt-a", available: true},
					flags: []runtime.FlagDef{
						{Name: "flag-a", Usage: "from rt-a"},
					},
				}
			},
			func() runtime.Runtime {
				return &testRuntime{runtimeType: "rt-b", available: true}
			},
			func() runtime.Runtime {
				return &testRuntimeWithFlags{
					testRuntime: &testRuntime{runtimeType: "rt-c", available: true},
					flags: []runtime.FlagDef{
						{Name: "flag-c", Usage: "from rt-c"},
					},
				}
			},
		}

		flags := listFlagsWithFactories(factories)

		if len(flags) != 2 {
			t.Fatalf("expected 2 flags, got %d", len(flags))
		}

		names := make(map[string]bool)
		for _, f := range flags {
			names[f.Name] = true
		}
		if !names["flag-a"] {
			t.Error("expected 'flag-a' to be present")
		}
		if !names["flag-c"] {
			t.Error("expected 'flag-c' to be present")
		}
	})
}
