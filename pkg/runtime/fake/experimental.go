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

package fake

import (
	"github.com/openkaiden/kdn/pkg/runtime"
)

// runtimeWithExperimental wraps fakeRuntime and implements the Experimental interface.
type runtimeWithExperimental struct {
	*fakeRuntime
}

// Ensure runtimeWithExperimental implements runtime.Experimental at compile time.
var _ runtime.Experimental = (*runtimeWithExperimental)(nil)

// NewWithExperimental creates a fake runtime that implements the Experimental interface.
func NewWithExperimental() runtime.Runtime {
	return &runtimeWithExperimental{
		fakeRuntime: New().(*fakeRuntime),
	}
}

// IsExperimental implements runtime.Experimental.
func (r *runtimeWithExperimental) IsExperimental() {}

// runtimeWithExperimentalAndDisplayName wraps runtimeWithExperimental and overrides DisplayName.
type runtimeWithExperimentalAndDisplayName struct {
	*runtimeWithExperimental
	displayName string
}

// NewWithExperimentalAndDisplayName creates a fake experimental runtime with a custom DisplayName.
// Use this in tests that need to verify DisplayName is used separately from Type.
func NewWithExperimentalAndDisplayName(displayName string) runtime.Runtime {
	return &runtimeWithExperimentalAndDisplayName{
		runtimeWithExperimental: &runtimeWithExperimental{
			fakeRuntime: New().(*fakeRuntime),
		},
		displayName: displayName,
	}
}

// DisplayName overrides the embedded runtime's DisplayName.
func (r *runtimeWithExperimentalAndDisplayName) DisplayName() string {
	return r.displayName
}
