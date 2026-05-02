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

// Package runtimesetup provides centralized registration of all available runtime implementations.
package runtimesetup

import (
	"sort"

	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/openkaiden/kdn/pkg/runtime/openshell"
	"github.com/openkaiden/kdn/pkg/runtime/podman"
)

// Registrar is an interface for types that can register runtimes.
// This is implemented by instances.Manager.
type Registrar interface {
	RegisterRuntime(rt runtime.Runtime) error
}

// Available is an optional interface that runtimes can implement
// to report whether they are available in the current environment.
//
// This allows runtimes to check for:
//   - Operating system compatibility
//   - Required CLI tools or binaries
//   - Configuration prerequisites
//   - License or permission requirements
//
// Example implementation:
//
//	type myRuntime struct {}
//
//	func (r *myRuntime) Available() bool {
//	    // Check if required CLI tool exists
//	    _, err := exec.LookPath("my-tool")
//	    return err == nil
//	}
type Available interface {
	// Available returns true if the runtime is available in the current environment.
	Available() bool
}

// runtimeFactory is a function that creates a new runtime instance.
type runtimeFactory func() runtime.Runtime

// availableRuntimes is the list of all runtimes that can be registered.
// Add new runtimes here to make them available for automatic registration.
var availableRuntimes = []runtimeFactory{
	fake.New,
	openshell.New,
	podman.New,
}

// ListAvailable returns the names of all available runtimes, excluding
// internal runtimes like "fake" (used only for testing).
// It checks each runtime's availability without requiring a manager instance.
// This is useful for tab-completion and other contexts where we want to avoid
// creating on-disk state.
func ListAvailable() []string {
	return listAvailableWithFactories(availableRuntimes)
}

// listAvailableWithFactories returns the names of available runtimes from the given factories.
// This function is internal and used for testing with custom runtime lists.
func listAvailableWithFactories(factories []runtimeFactory) []string {
	var names []string

	for _, factory := range factories {
		rt := factory()

		// Skip runtimes that are not available in this environment
		if avail, ok := rt.(Available); ok && !avail.Available() {
			continue
		}

		// Skip internal runtimes not intended for display
		if rt.Type() == "fake" {
			continue
		}

		names = append(names, rt.Type())
	}

	return names
}

// ListAgents returns the names of all agents supported by available runtimes.
// It creates a temporary registry with the given storage directory, registers all
// available runtimes (which initializes StorageAware runtimes), and then queries
// each runtime that implements the AgentLister interface.
func ListAgents(runtimeStorageDir string) ([]string, error) {
	return listAgentsWithFactories(runtimeStorageDir, availableRuntimes)
}

// listAgentsWithFactories returns agent names from the given runtime factories.
// This function is internal and used for testing with custom runtime lists.
func listAgentsWithFactories(runtimeStorageDir string, factories []runtimeFactory) ([]string, error) {
	registry, err := runtime.NewRegistry(runtimeStorageDir)
	if err != nil {
		return nil, err
	}

	agentSet := make(map[string]struct{})
	for _, factory := range factories {
		rt := factory()

		// Skip runtimes that are not available in this environment
		if avail, ok := rt.(Available); ok && !avail.Available() {
			continue
		}

		// Register to initialize StorageAware runtimes with their storage directory
		if err := registry.Register(rt); err != nil {
			continue
		}

		lister, ok := rt.(runtime.AgentLister)
		if !ok {
			continue
		}

		agents, err := lister.ListAgents()
		if err != nil {
			continue
		}

		for _, agent := range agents {
			agentSet[agent] = struct{}{}
		}
	}

	agents := make([]string, 0, len(agentSet))
	for agent := range agentSet {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	return agents, nil
}

// ListDashboardRuntimeTypes returns the type names of all available runtimes
// that implement the Dashboard interface.
// It instantiates each runtime and checks for the interface, skipping unavailable runtimes.
func ListDashboardRuntimeTypes(runtimeStorageDir string) ([]string, error) {
	return listDashboardRuntimeTypesWithFactories(runtimeStorageDir, availableRuntimes)
}

// listDashboardRuntimeTypesWithFactories returns Dashboard-capable runtime type names
// from the given factories. This function is internal and used for testing.
func listDashboardRuntimeTypesWithFactories(runtimeStorageDir string, factories []runtimeFactory) ([]string, error) {
	registry, err := runtime.NewRegistry(runtimeStorageDir)
	if err != nil {
		return nil, err
	}

	var types []string
	for _, factory := range factories {
		rt := factory()

		if avail, ok := rt.(Available); ok && !avail.Available() {
			continue
		}

		if err := registry.Register(rt); err != nil {
			continue
		}

		if _, ok := rt.(runtime.Dashboard); ok {
			types = append(types, rt.Type())
		}
	}

	return types, nil
}

// RegisterAll registers all available runtimes to the given registrar.
// It skips runtimes that implement the Available interface and report false.
// Returns an error if any runtime fails to register.
func RegisterAll(registrar Registrar) error {
	return registerAllWithAvailable(registrar, availableRuntimes)
}

// registerAllWithAvailable registers the given runtimes to the registrar.
// It skips runtimes that implement the Available interface and report false.
// Returns an error if any runtime fails to register.
// This function is internal and used for testing with custom runtime lists.
func registerAllWithAvailable(registrar Registrar, factories []runtimeFactory) error {
	for _, factory := range factories {
		rt := factory()

		// Skip runtimes that are not available in this environment
		if avail, ok := rt.(Available); ok && !avail.Available() {
			continue
		}

		if err := registrar.RegisterRuntime(rt); err != nil {
			return err
		}
	}

	return nil
}

// ListFlags returns the CLI flag definitions declared by all available runtimes
// that implement the FlagProvider interface.
// Flags from unavailable runtimes and the internal "fake" runtime are excluded.
func ListFlags() []runtime.FlagDef {
	return listFlagsWithFactories(availableRuntimes)
}

// listFlagsWithFactories returns flag definitions from the given runtime factories.
func listFlagsWithFactories(factories []runtimeFactory) []runtime.FlagDef {
	seen := make(map[string]struct{})
	var flags []runtime.FlagDef

	for _, factory := range factories {
		rt := factory()

		if avail, ok := rt.(Available); ok && !avail.Available() {
			continue
		}

		if rt.Type() == "fake" {
			continue
		}

		fp, ok := rt.(runtime.FlagProvider)
		if !ok {
			continue
		}

		for _, f := range fp.Flags() {
			if _, exists := seen[f.Name]; !exists {
				seen[f.Name] = struct{}{}
				flags = append(flags, f)
			}
		}
	}

	return flags
}
