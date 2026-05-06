/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

package credential

import (
	"context"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/onecli"
)

// fakeCredential is a test implementation of Credential.
type fakeCredential struct {
	name string
}

func (f *fakeCredential) Name() string              { return f.name }
func (f *fakeCredential) ContainerFilePath() string { return "/fake/path" }
func (f *fakeCredential) Detect(_ []workspace.Mount, _ string) (string, *workspace.Mount) {
	return "", nil
}
func (f *fakeCredential) FakeFile(_ string) ([]byte, error)                            { return nil, nil }
func (f *fakeCredential) Configure(_ context.Context, _ onecli.Client, _ string) error { return nil }
func (f *fakeCredential) HostPatterns(_ string) []string                               { return nil }

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("registers a credential", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		if err := r.Register(&fakeCredential{name: "test"}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	})

	t.Run("rejects nil", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		if err := r.Register(nil); err == nil {
			t.Fatal("Register(nil) expected error, got nil")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		if err := r.Register(&fakeCredential{name: ""}); err == nil {
			t.Fatal("Register(empty name) expected error, got nil")
		}
	})

	t.Run("rejects duplicate name", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		if err := r.Register(&fakeCredential{name: "dup"}); err != nil {
			t.Fatalf("first Register() error = %v", err)
		}
		if err := r.Register(&fakeCredential{name: "dup"}); err == nil {
			t.Fatal("second Register() expected error for duplicate name, got nil")
		}
	})
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	t.Run("empty registry returns empty slice", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		if got := r.List(); len(got) != 0 {
			t.Errorf("List() = %v, want empty", got)
		}
	})

	t.Run("preserves registration order", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		_ = r.Register(&fakeCredential{name: "a"})
		_ = r.Register(&fakeCredential{name: "b"})
		_ = r.Register(&fakeCredential{name: "c"})

		got := r.List()
		if len(got) != 3 {
			t.Fatalf("List() len = %d, want 3", len(got))
		}
		names := []string{got[0].Name(), got[1].Name(), got[2].Name()}
		want := []string{"a", "b", "c"}
		for i, n := range names {
			if n != want[i] {
				t.Errorf("List()[%d].Name() = %q, want %q", i, n, want[i])
			}
		}
	})
}
