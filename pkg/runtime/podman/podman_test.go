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

package podman

import (
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/system"
)

func TestNew(t *testing.T) {
	t.Parallel()

	rt := New()
	if rt == nil {
		t.Fatal("New() returned nil")
	}

	if rt.Type() != "podman" {
		t.Errorf("Expected type 'podman', got %s", rt.Type())
	}
}

func TestPodmanRuntime_Available(t *testing.T) {
	t.Parallel()

	t.Run("returns true when podman command exists", func(t *testing.T) {
		t.Parallel()

		fakeSys := &fakeSystem{commandExists: true}
		rt := newWithSystem(fakeSys)

		avail, ok := rt.(interface{ Available() bool })
		if !ok {
			t.Fatal("Expected runtime to implement Available interface")
		}

		if !avail.Available() {
			t.Error("Expected Available() to return true when podman exists")
		}

		if fakeSys.checkedCommand != "podman" {
			t.Errorf("Expected to check for 'podman' command, got '%s'", fakeSys.checkedCommand)
		}
	})

	t.Run("returns false when podman command does not exist", func(t *testing.T) {
		t.Parallel()

		fakeSys := &fakeSystem{commandExists: false}
		rt := newWithSystem(fakeSys)

		avail, ok := rt.(interface{ Available() bool })
		if !ok {
			t.Fatal("Expected runtime to implement Available interface")
		}

		if avail.Available() {
			t.Error("Expected Available() to return false when podman does not exist")
		}

		if fakeSys.checkedCommand != "podman" {
			t.Errorf("Expected to check for 'podman' command, got '%s'", fakeSys.checkedCommand)
		}
	})
}

// fakeSystem is a fake implementation of system.System for testing.
type fakeSystem struct {
	commandExists  bool
	checkedCommand string
}

// Ensure fakeSystem implements system.System at compile time.
var _ system.System = (*fakeSystem)(nil)

func (f *fakeSystem) CommandExists(name string) bool {
	f.checkedCommand = name
	return f.commandExists
}
