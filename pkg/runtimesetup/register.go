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
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/fake"
	"github.com/kortex-hub/kortex-cli/pkg/runtime/podman"
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
	podman.New,
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
