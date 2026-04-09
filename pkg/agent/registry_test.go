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

package agent

import (
	"errors"
	"testing"
)

// fakeAgent is a test implementation of the Agent interface
type fakeAgent struct {
	name string
}

func (f *fakeAgent) Name() string {
	return f.name
}

func (f *fakeAgent) SkipOnboarding(settings map[string][]byte, _ string) (map[string][]byte, error) {
	return settings, nil
}

func (f *fakeAgent) SetModel(settings map[string][]byte, _ string) (map[string][]byte, error) {
	return settings, nil
}

func (f *fakeAgent) SkillsDir() string {
	return ""
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("successfully registers agent", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()
		agent := &fakeAgent{name: "test-agent"}

		err := reg.Register("test-agent", agent)
		if err != nil {
			t.Errorf("Register() error = %v, want nil", err)
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()
		agent := &fakeAgent{name: "test-agent"}

		err := reg.Register("", agent)
		if err == nil {
			t.Error("Register() with empty name should return error")
		}
	})

	t.Run("returns error for nil agent", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()

		err := reg.Register("test-agent", nil)
		if err == nil {
			t.Error("Register() with nil agent should return error")
		}
	})

	t.Run("returns error for duplicate registration", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()
		agent1 := &fakeAgent{name: "test-agent"}
		agent2 := &fakeAgent{name: "test-agent"}

		err := reg.Register("test-agent", agent1)
		if err != nil {
			t.Fatalf("First Register() error = %v, want nil", err)
		}

		err = reg.Register("test-agent", agent2)
		if err == nil {
			t.Error("Register() duplicate should return error")
		}
	})
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves registered agent", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()
		agent := &fakeAgent{name: "test-agent"}

		err := reg.Register("test-agent", agent)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		retrieved, err := reg.Get("test-agent")
		if err != nil {
			t.Errorf("Get() error = %v, want nil", err)
		}

		if retrieved == nil {
			t.Fatal("Get() returned nil agent")
		}

		if retrieved.Name() != "test-agent" {
			t.Errorf("Get() returned agent with name %q, want %q", retrieved.Name(), "test-agent")
		}
	})

	t.Run("returns ErrAgentNotFound for unregistered agent", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()

		_, err := reg.Get("nonexistent")
		if err == nil {
			t.Error("Get() for nonexistent agent should return error")
		}

		if !errors.Is(err, ErrAgentNotFound) {
			t.Errorf("Get() error = %v, want ErrAgentNotFound", err)
		}
	})
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list for new registry", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()
		names := reg.List()

		if len(names) != 0 {
			t.Errorf("List() returned %d names, want 0", len(names))
		}
	})

	t.Run("returns all registered agent names", func(t *testing.T) {
		t.Parallel()

		reg := NewRegistry()

		agents := []string{"agent1", "agent2", "agent3"}
		for _, name := range agents {
			err := reg.Register(name, &fakeAgent{name: name})
			if err != nil {
				t.Fatalf("Register(%q) error = %v", name, err)
			}
		}

		names := reg.List()
		if len(names) != len(agents) {
			t.Errorf("List() returned %d names, want %d", len(names), len(agents))
		}

		// Check all expected names are present
		nameMap := make(map[string]bool)
		for _, name := range names {
			nameMap[name] = true
		}

		for _, expected := range agents {
			if !nameMap[expected] {
				t.Errorf("List() missing expected agent %q", expected)
			}
		}
	})
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	// Register an agent
	err := reg.Register("test-agent", &fakeAgent{name: "test-agent"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Concurrent reads should be safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = reg.Get("test-agent")
			_ = reg.List()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
