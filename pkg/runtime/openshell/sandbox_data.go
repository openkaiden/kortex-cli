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
	"fmt"
	"os"
	"path/filepath"
)

const sandboxDataFile = "sandbox-data.json"

type sandboxData struct {
	SourcePath string `json:"source_path"`
	ProjectID  string `json:"project_id"`
	Agent      string `json:"agent"`
	Ports      []int  `json:"ports,omitempty"`
}

func (r *openshellRuntime) sandboxDataDir(sandboxName string) string {
	return filepath.Join(r.storageDir, "sandboxes", sandboxName)
}

func (r *openshellRuntime) writeSandboxData(sandboxName string, data sandboxData) error {
	dir := r.sandboxDataDir(sandboxName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating sandbox data directory: %w", err)
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sandbox data: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, sandboxDataFile), raw, 0644); err != nil {
		return fmt.Errorf("writing sandbox data: %w", err)
	}
	return nil
}

func (r *openshellRuntime) readSandboxData(sandboxName string) (sandboxData, error) {
	path := filepath.Join(r.sandboxDataDir(sandboxName), sandboxDataFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return sandboxData{}, fmt.Errorf("reading sandbox data: %w", err)
	}

	var data sandboxData
	if err := json.Unmarshal(raw, &data); err != nil {
		return sandboxData{}, fmt.Errorf("unmarshaling sandbox data: %w", err)
	}
	return data, nil
}

func (r *openshellRuntime) removeSandboxData(sandboxName string) {
	os.RemoveAll(r.sandboxDataDir(sandboxName))
}
