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

package openshell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	api "github.com/openkaiden/kdn-api/cli/go"
)

const stateOverridesFile = "state-overrides.json"

// stateOverrides manages sandbox state overrides for the stop/start no-op pattern.
// OpenShell sandboxes cannot be stopped, so we track a virtual "stopped" state locally.
type stateOverrides struct {
	mu   sync.Mutex
	path string
}

func newStateOverrides(storageDir string) *stateOverrides {
	return &stateOverrides{
		path: filepath.Join(storageDir, stateOverridesFile),
	}
}

func (s *stateOverrides) load() (map[string]api.WorkspaceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]api.WorkspaceState), nil
		}
		return nil, err
	}

	overrides := make(map[string]api.WorkspaceState)
	if err := json.Unmarshal(data, &overrides); err != nil {
		return make(map[string]api.WorkspaceState), nil
	}
	return overrides, nil
}

func (s *stateOverrides) save(overrides map[string]api.WorkspaceState) error {
	data, err := json.Marshal(overrides)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// Set records a state override for a sandbox.
func (s *stateOverrides) Set(id string, state api.WorkspaceState) error {
	overrides, err := s.load()
	if err != nil {
		return err
	}
	overrides[id] = state
	return s.save(overrides)
}

// Get returns the state override for a sandbox, or empty string if none.
func (s *stateOverrides) Get(id string) (api.WorkspaceState, bool) {
	overrides, err := s.load()
	if err != nil {
		return "", false
	}
	state, ok := overrides[id]
	return state, ok
}

// Remove deletes the state override for a sandbox.
func (s *stateOverrides) Remove(id string) error {
	overrides, err := s.load()
	if err != nil {
		return err
	}
	delete(overrides, id)
	return s.save(overrides)
}
