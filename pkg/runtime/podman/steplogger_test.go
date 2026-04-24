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
	"os"
	"path/filepath"
	"testing"

	"github.com/openkaiden/kdn/pkg/runtime/podman/config"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// fakeStepLogger is a fake implementation of steplogger.StepLogger that records calls for testing.
type fakeStepLogger struct {
	startCalls    []stepCall
	failCalls     []error
	completeCalls int
}

type stepCall struct {
	inProgress string
	completed  string
}

// Ensure fakeStepLogger implements steplogger.StepLogger at compile time.
var _ steplogger.StepLogger = (*fakeStepLogger)(nil)

func (f *fakeStepLogger) Start(inProgress, completed string) {
	f.startCalls = append(f.startCalls, stepCall{
		inProgress: inProgress,
		completed:  completed,
	})
}

func (f *fakeStepLogger) Fail(err error) {
	f.failCalls = append(f.failCalls, err)
}

func (f *fakeStepLogger) Complete() {
	f.completeCalls++
}

// fakeConfig is a fake implementation of config.Config that returns default configurations.
type fakeConfig struct{}

// Ensure fakeConfig implements config.Config at compile time.
var _ config.Config = (*fakeConfig)(nil)

func (f *fakeConfig) LoadImage() (*config.ImageConfig, error) {
	return &config.ImageConfig{
		Version:     "latest",
		Packages:    []string{"which", "procps-ng"},
		Sudo:        []string{"/usr/bin/dnf"},
		RunCommands: []string{},
	}, nil
}

func (f *fakeConfig) LoadAgent(agentName string) (*config.AgentConfig, error) {
	return &config.AgentConfig{
		Packages:        []string{},
		RunCommands:     []string{"curl -fsSL https://claude.ai/install.sh | bash"},
		TerminalCommand: []string{"claude"},
	}, nil
}

func (f *fakeConfig) ListAgents() ([]string, error) {
	return []string{"claude"}, nil
}

func (f *fakeConfig) GenerateDefaults() error {
	return nil
}

// setupPodFiles creates per-container pod files in a temporary storage directory for testing.
// Returns the pod name that was written.
func setupPodFiles(t *testing.T, p *podmanRuntime, containerID, workspaceName string) string {
	t.Helper()
	if p.storageDir == "" {
		p.storageDir = t.TempDir()
	}
	if p.globalStorageDir == "" {
		p.globalStorageDir = t.TempDir()
	}
	approvalDir := filepath.Join(p.storageDir, "approval-handler", workspaceName)
	if err := os.MkdirAll(approvalDir, 0755); err != nil {
		t.Fatalf("failed to create approval handler dir: %v", err)
	}
	data := podTemplateData{
		Name:               workspaceName,
		OnecliWebPort:      20254,
		OnecliVersion:      defaultOnecliVersion,
		ApprovalHandlerDir: approvalDir,
	}
	if err := p.writePodFiles(containerID, data); err != nil {
		t.Fatalf("failed to write pod files for test: %v", err)
	}
	return workspaceName
}
