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

package instances

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	api "github.com/openkaiden/kdn-api/cli/go"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/agent"
	"github.com/openkaiden/kdn/pkg/git"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/fake"
	"github.com/openkaiden/kdn/pkg/secretservice"
)

// fakeInstance is a test double for the Instance interface
type fakeInstance struct {
	id         string
	name       string
	sourceDir  string
	configDir  string
	accessible bool
	runtime    RuntimeData
	project    string
	agent      string
	model      string
}

// Compile-time check to ensure fakeInstance implements Instance interface
var _ Instance = (*fakeInstance)(nil)

func (f *fakeInstance) GetID() string {
	return f.id
}

func (f *fakeInstance) GetName() string {
	return f.name
}

func (f *fakeInstance) GetSourceDir() string {
	return f.sourceDir
}

func (f *fakeInstance) GetConfigDir() string {
	return f.configDir
}

func (f *fakeInstance) IsAccessible() bool {
	return f.accessible
}

func (f *fakeInstance) GetRuntimeType() string {
	return f.runtime.Type
}

func (f *fakeInstance) GetRuntimeData() RuntimeData {
	return f.runtime
}

func (f *fakeInstance) GetProject() string {
	return f.project
}

func (f *fakeInstance) GetAgent() string {
	return f.agent
}

func (f *fakeInstance) GetModel() string {
	return f.model
}

func (f *fakeInstance) Dump() InstanceData {
	return InstanceData{
		ID:   f.id,
		Name: f.name,
		Paths: InstancePaths{
			Source:        f.sourceDir,
			Configuration: f.configDir,
		},
		Runtime: f.runtime,
		Project: f.project,
		Agent:   f.agent,
	}
}

// newFakeInstanceParams contains the parameters for creating a fake instance
type newFakeInstanceParams struct {
	ID         string
	Name       string
	SourceDir  string
	ConfigDir  string
	Accessible bool
	Runtime    RuntimeData
	Project    string
	Agent      string
	Model      string
}

// newFakeInstance creates a new fake instance for testing
func newFakeInstance(params newFakeInstanceParams) Instance {
	return &fakeInstance{
		id:         params.ID,
		name:       params.Name,
		sourceDir:  params.SourceDir,
		configDir:  params.ConfigDir,
		accessible: params.Accessible,
		runtime:    params.Runtime,
		project:    params.Project,
		agent:      params.Agent,
		model:      params.Model,
	}
}

// fakeInstanceFactory creates fake instances from InstanceData for testing
func fakeInstanceFactory(data InstanceData) (Instance, error) {
	if data.ID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	if data.Name == "" {
		return nil, errors.New("instance name cannot be empty")
	}
	if data.Paths.Source == "" {
		return nil, ErrInvalidPath
	}
	if data.Paths.Configuration == "" {
		return nil, ErrInvalidPath
	}
	// For testing, we assume instances are accessible by default
	// Tests can verify accessibility behavior separately
	return &fakeInstance{
		id:         data.ID,
		name:       data.Name,
		sourceDir:  data.Paths.Source,
		configDir:  data.Paths.Configuration,
		accessible: true,
		runtime:    data.Runtime,
		project:    data.Project,
		agent:      data.Agent,
		model:      data.Model,
	}, nil
}

// fakeGenerator is a test double for the generator.Generator interface
type fakeGenerator struct {
	counter int
	mu      sync.Mutex
}

func newFakeGenerator() *fakeGenerator {
	return &fakeGenerator{counter: 0}
}

func (g *fakeGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	// Generate a deterministic ID with hex characters to avoid all-numeric
	return fmt.Sprintf("test-id-%064x", g.counter)
}

// fakeSequentialGenerator returns a predefined sequence of IDs
type fakeSequentialGenerator struct {
	ids       []string
	callCount int
	mu        sync.Mutex
}

func newFakeSequentialGenerator(ids []string) *fakeSequentialGenerator {
	return &fakeSequentialGenerator{ids: ids, callCount: 0}
}

func (g *fakeSequentialGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.callCount >= len(g.ids) {
		// If we've exhausted the predefined IDs, return the last one
		return g.ids[len(g.ids)-1]
	}
	id := g.ids[g.callCount]
	g.callCount++
	return id
}

// trackingAgent is a test double for the Agent interface that tracks method calls
type trackingAgent struct {
	name                   string
	skillsDir              string
	skipOnboardingCalled   bool
	skipOnboardingSettings map[string][]byte
	skipOnboardingPath     string
	setModelCalled         bool
	setModelSettings       map[string][]byte
	setModelID             string
	setMCPServersCalled    bool
	mu                     sync.Mutex
}

// Compile-time check to ensure trackingAgent implements agent.Agent interface
var _ agent.Agent = (*trackingAgent)(nil)

func newTrackingAgent(name string) *trackingAgent {
	return &trackingAgent{name: name}
}

func newTrackingAgentWithSkillsDir(name, skillsDir string) *trackingAgent {
	return &trackingAgent{name: name, skillsDir: skillsDir}
}

func (t *trackingAgent) Name() string {
	return t.name
}

func (t *trackingAgent) SkipOnboarding(settings map[string][]byte, workspaceSourcesPath string) (map[string][]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skipOnboardingCalled = true
	t.skipOnboardingSettings = settings
	t.skipOnboardingPath = workspaceSourcesPath
	if settings == nil {
		settings = make(map[string][]byte)
	}
	return settings, nil
}

func (t *trackingAgent) SetModel(settings map[string][]byte, modelID string) (map[string][]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.setModelCalled = true
	t.setModelSettings = settings
	t.setModelID = modelID
	if settings == nil {
		settings = make(map[string][]byte)
	}
	return settings, nil
}

func (t *trackingAgent) WasSetModelCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setModelCalled
}

func (t *trackingAgent) GetSetModelID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setModelID
}

func (t *trackingAgent) SetMCPServers(settings map[string][]byte, _ *workspace.McpConfiguration) (map[string][]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.setMCPServersCalled = true
	if settings == nil {
		settings = make(map[string][]byte)
	}
	return settings, nil
}

func (t *trackingAgent) WasSetMCPServersCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setMCPServersCalled
}

func (t *trackingAgent) WasSkipOnboardingCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.skipOnboardingCalled
}

func (t *trackingAgent) SkillsDir() string {
	return t.skillsDir
}

// erroringSetModelAgent is a test double that returns an error from SetModel
type erroringSetModelAgent struct {
	name string
}

// Compile-time check to ensure erroringSetModelAgent implements agent.Agent interface
var _ agent.Agent = (*erroringSetModelAgent)(nil)

func newErroringSetModelAgent(name string) *erroringSetModelAgent {
	return &erroringSetModelAgent{name: name}
}

func (e *erroringSetModelAgent) Name() string {
	return e.name
}

func (e *erroringSetModelAgent) SkipOnboarding(settings map[string][]byte, _ string) (map[string][]byte, error) {
	if settings == nil {
		settings = make(map[string][]byte)
	}
	return settings, nil
}

func (e *erroringSetModelAgent) SetModel(_ map[string][]byte, _ string) (map[string][]byte, error) {
	return nil, errors.New("simulated SetModel error")
}

func (e *erroringSetModelAgent) SkillsDir() string {
	return ""
}

func (e *erroringSetModelAgent) SetMCPServers(settings map[string][]byte, _ *workspace.McpConfiguration) (map[string][]byte, error) {
	return settings, nil
}

// newTestRegistry creates a runtime registry with a fake runtime for testing
func newTestRegistry(storageDir string) runtime.Registry {
	runtimesDir := filepath.Join(storageDir, RuntimesSubdirectory)
	reg, err := runtime.NewRegistry(runtimesDir)
	if err != nil {
		panic(fmt.Sprintf("failed to create test registry: %v", err))
	}
	_ = reg.Register(fake.New())
	return reg
}

// fakeGitDetector is a test double for the git.Detector interface
type fakeGitDetector struct {
	repoInfo *git.RepositoryInfo
	err      error
}

func newFakeGitDetector() *fakeGitDetector {
	return &fakeGitDetector{
		err: git.ErrNotGitRepository,
	}
}

func (f *fakeGitDetector) DetectRepository(ctx context.Context, dir string) (*git.RepositoryInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.repoInfo, nil
}

func TestNewManager(t *testing.T) {
	t.Parallel()

	t.Run("creates manager with valid storage directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, err := NewManager(tmpDir)
		if err != nil {
			t.Fatalf("NewManager() unexpected error = %v", err)
		}
		if manager == nil {
			t.Fatal("NewManager() returned nil manager")
		}

		// Verify storage directory exists
		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			t.Error("Storage directory was not created")
		}
	})

	t.Run("creates storage directory if not exists", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "nested", "storage")

		manager, err := NewManager(nestedDir)
		if err != nil {
			t.Fatalf("NewManager() unexpected error = %v", err)
		}
		if manager == nil {
			t.Fatal("NewManager() returned nil manager")
		}

		// Verify nested directory was created
		info, err := os.Stat(nestedDir)
		if err != nil {
			t.Fatalf("Nested directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Storage path is not a directory")
		}
	})

	t.Run("returns error for empty storage directory", func(t *testing.T) {
		t.Parallel()

		_, err := NewManager("")
		if err == nil {
			t.Error("NewManager() expected error for empty storage dir, got nil")
		}
	})

	t.Run("verifies storage file path is correct", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, err := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		if err != nil {
			t.Fatalf("newManagerWithFactory() unexpected error = %v", err)
		}

		// We can't directly access storageFile since it's on the unexported struct,
		// but we can verify behavior by adding an instance and checking file creation
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		_, addErr := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})
		if addErr != nil {
			t.Fatalf("Failed to add instance: %v", addErr)
		}

		expectedFile := filepath.Join(tmpDir, DefaultStorageFileName)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Storage file was not created at expected path: %v", expectedFile)
		}
	})
}

func TestManager_Add(t *testing.T) {
	t.Parallel()

	t.Run("adds valid instance successfully", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, err := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})
		if err != nil {
			t.Fatalf("Add() unexpected error = %v", err)
		}
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}
		if added.GetID() == "" {
			t.Error("Add() returned instance with empty ID")
		}

		// Verify instance was added
		instances, _ := manager.List()
		if len(instances) != 1 {
			t.Errorf("List() returned %d instances, want 1", len(instances))
		}
	})

	t.Run("returns error for nil instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		_, err := manager.Add(context.Background(), AddOptions{Instance: nil, RuntimeType: "fake"})
		if err == nil {
			t.Error("Add() expected error for nil instance, got nil")
		}
	})

	t.Run("generates unique IDs when adding instances", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		// Create a sequential generator that returns duplicate ID first, then unique ones
		// Sequence: "duplicate-id", "duplicate-id", "unique-id-1"
		// When adding first instance: gets "duplicate-id"
		// When adding second instance: gets "duplicate-id" (skip), then "unique-id-1"
		gen := newFakeSequentialGenerator([]string{
			"duplicate-id-0000000000000000000000000000000000000000000000000000000a",
			"duplicate-id-0000000000000000000000000000000000000000000000000000000a",
			"unique-id-1-0000000000000000000000000000000000000000000000000000000b",
		})
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, gen, newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		// Create instances without IDs (empty ID)
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})

		added1, _ := manager.Add(ctx, AddOptions{Instance: inst1, RuntimeType: "fake"})
		added2, _ := manager.Add(ctx, AddOptions{Instance: inst2, RuntimeType: "fake"})

		id1 := added1.GetID()
		id2 := added2.GetID()

		if id1 == "" {
			t.Error("First instance has empty ID")
		}
		if id2 == "" {
			t.Error("Second instance has empty ID")
		}
		if id1 == id2 {
			t.Errorf("Manager generated duplicate IDs: %v", id1)
		}

		// Verify the manager skipped the duplicate and used the third ID
		expectedID1 := "duplicate-id-0000000000000000000000000000000000000000000000000000000a"
		expectedID2 := "unique-id-1-0000000000000000000000000000000000000000000000000000000b"
		if id1 != expectedID1 {
			t.Errorf("First instance ID = %v, want %v", id1, expectedID1)
		}
		if id2 != expectedID2 {
			t.Errorf("Second instance ID = %v, want %v", id2, expectedID2)
		}

		// Verify the generator was called 3 times (once for first instance, twice for second)
		if gen.callCount != 3 {
			t.Errorf("Generator was called %d times, want 3", gen.callCount)
		}
	})

	t.Run("verifies persistence to JSON file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		// Check that JSON file exists and is readable
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		data, err := os.ReadFile(storageFile)
		if err != nil {
			t.Fatalf("Failed to read storage file: %v", err)
		}
		if len(data) == 0 {
			t.Error("Storage file is empty")
		}
	})

	t.Run("can add multiple instances", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})
		inst3 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source3"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config3"),
			Accessible: true,
		})

		_, _ = manager.Add(ctx, AddOptions{Instance: inst1, RuntimeType: "fake"})
		_, _ = manager.Add(ctx, AddOptions{Instance: inst2, RuntimeType: "fake"})
		_, _ = manager.Add(ctx, AddOptions{Instance: inst3, RuntimeType: "fake"})

		instances, _ := manager.List()
		if len(instances) != 3 {
			t.Errorf("List() returned %d instances, want 3", len(instances))
		}
	})
}

func TestManager_List(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list when no instances exist", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("List() returned %d instances, want 0", len(instances))
		}
	})

	t.Run("returns all added instances", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})

		_, _ = manager.Add(ctx, AddOptions{Instance: inst1, RuntimeType: "fake"})
		_, _ = manager.Add(ctx, AddOptions{Instance: inst2, RuntimeType: "fake"})

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 2 {
			t.Errorf("List() returned %d instances, want 2", len(instances))
		}
	})

	t.Run("handles empty storage file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		// Create empty storage file
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		os.WriteFile(storageFile, []byte{}, 0644)

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("List() returned %d instances, want 0 for empty file", len(instances))
		}
	})
}

func TestManager_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves existing instance by ID", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added, _ := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		retrieved, err := manager.Get(generatedID)
		if err != nil {
			t.Fatalf("Get() unexpected error = %v", err)
		}
		if retrieved.GetID() != generatedID {
			t.Errorf("Get() returned instance with ID = %v, want %v", retrieved.GetID(), generatedID)
		}
		if retrieved.GetSourceDir() != expectedSource {
			t.Errorf("Get() returned instance with SourceDir = %v, want %v", retrieved.GetSourceDir(), expectedSource)
		}
		if retrieved.GetConfigDir() != expectedConfig {
			t.Errorf("Get() returned instance with ConfigDir = %v, want %v", retrieved.GetConfigDir(), expectedConfig)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		_, err := manager.Get("nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Get() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("retrieves existing instance by name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		expectedName := "my-workspace"
		inst := newFakeInstance(newFakeInstanceParams{
			Name:       expectedName,
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added, _ := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		// Retrieve by name
		retrieved, err := manager.Get(expectedName)
		if err != nil {
			t.Fatalf("Get() unexpected error = %v", err)
		}
		if retrieved.GetID() != generatedID {
			t.Errorf("Get() returned instance with ID = %v, want %v", retrieved.GetID(), generatedID)
		}
		if retrieved.GetName() != expectedName {
			t.Errorf("Get() returned instance with Name = %v, want %v", retrieved.GetName(), expectedName)
		}
		if retrieved.GetSourceDir() != expectedSource {
			t.Errorf("Get() returned instance with SourceDir = %v, want %v", retrieved.GetSourceDir(), expectedSource)
		}
	})

	t.Run("returns error for nonexistent name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		_, err := manager.Get("nonexistent-name")
		if err != ErrInstanceNotFound {
			t.Errorf("Get() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("ID takes precedence over name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		// Create first instance
		inst1 := newFakeInstance(newFakeInstanceParams{
			Name:       "workspace-one",
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		added1, _ := manager.Add(context.Background(), AddOptions{Instance: inst1, RuntimeType: "fake"})
		id1 := added1.GetID()

		// Create second instance
		inst2 := newFakeInstance(newFakeInstanceParams{
			Name:       "workspace-two",
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})
		_, _ = manager.Add(context.Background(), AddOptions{Instance: inst2, RuntimeType: "fake"})

		// Retrieve using the ID of first instance (should return first instance, not second)
		retrieved, err := manager.Get(id1)
		if err != nil {
			t.Fatalf("Get() unexpected error = %v", err)
		}
		if retrieved.GetID() != id1 {
			t.Errorf("Get() returned instance with ID = %v, want %v", retrieved.GetID(), id1)
		}
		if retrieved.GetName() != "workspace-one" {
			t.Errorf("Get() returned instance with Name = %v, want %v", retrieved.GetName(), "workspace-one")
		}
	})
}

func TestManager_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing instance successfully", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  sourceDir,
			ConfigDir:  configDir,
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		err := manager.Delete(ctx, generatedID)
		if err != nil {
			t.Fatalf("Delete() unexpected error = %v", err)
		}

		// Verify instance was deleted
		_, err = manager.Get(generatedID)
		if err != ErrInstanceNotFound {
			t.Errorf("Get() after Delete() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		err := manager.Delete(context.Background(), "nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Delete() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("deletes only specified instance", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		source1 := filepath.Join(instanceTmpDir, "source1")
		config1 := filepath.Join(instanceTmpDir, "config1")
		source2 := filepath.Join(instanceTmpDir, "source2")
		config2 := filepath.Join(instanceTmpDir, "config2")
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  source1,
			ConfigDir:  config1,
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  source2,
			ConfigDir:  config2,
			Accessible: true,
		})
		added1, _ := manager.Add(ctx, AddOptions{Instance: inst1, RuntimeType: "fake"})
		added2, _ := manager.Add(ctx, AddOptions{Instance: inst2, RuntimeType: "fake"})

		id1 := added1.GetID()
		id2 := added2.GetID()

		manager.Delete(ctx, id1)

		// Verify inst2 still exists
		_, err := manager.Get(id2)
		if err != nil {
			t.Errorf("Get(id2) after Delete(id1) unexpected error = %v", err)
		}

		// Verify inst1 is gone
		_, err = manager.Get(id1)
		if err != ErrInstanceNotFound {
			t.Errorf("Get(id1) after Delete(id1) error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("verifies deletion is persisted", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager1, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added, _ := manager1.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		manager1.Delete(ctx, generatedID)

		// Create new manager with same storage
		manager2, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		_, err := manager2.Get(generatedID)
		if err != ErrInstanceNotFound {
			t.Errorf("Get() from new manager error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("returns error when runtime Remove fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  sourceDir,
			ConfigDir:  configDir,
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		// Start the instance (fake runtime doesn't allow removing running instances)
		err := manager.Start(ctx, generatedID)
		if err != nil {
			t.Fatalf("Start() unexpected error = %v", err)
		}

		// Try to delete while running - should fail
		err = manager.Delete(ctx, generatedID)
		if err == nil {
			t.Fatal("Delete() expected error when runtime Remove fails, got nil")
		}

		// Verify instance was NOT deleted from manager storage
		_, err = manager.Get(generatedID)
		if err != nil {
			t.Errorf("Get() after failed Delete() unexpected error = %v, instance should still exist", err)
		}
	})
}

func TestManager_Start(t *testing.T) {
	t.Parallel()

	t.Run("starts instance successfully", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		// After Add, state should be "stopped"
		if added.GetRuntimeData().State != "stopped" {
			t.Errorf("After Add, state = %v, want 'stopped'", added.GetRuntimeData().State)
		}

		// Start the instance
		err := manager.Start(ctx, added.GetID())
		if err != nil {
			t.Fatalf("Start() unexpected error = %v", err)
		}

		// Verify state is now "running"
		updated, _ := manager.Get(added.GetID())
		if updated.GetRuntimeData().State != "running" {
			t.Errorf("After Start, state = %v, want 'running'", updated.GetRuntimeData().State)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		err := manager.Start(context.Background(), "nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Start() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("returns error for instance without runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		// Custom factory that creates instances without runtime
		noRuntimeFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: true,
				runtime:    RuntimeData{}, // Empty runtime
			}, nil
		}
		manager, _ := newManagerWithFactory(tmpDir, noRuntimeFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		// Manually save instance without runtime to simulate old data
		instances := []Instance{&fakeInstance{
			id:         added.GetID(),
			name:       added.GetName(),
			sourceDir:  added.GetSourceDir(),
			configDir:  added.GetConfigDir(),
			accessible: true,
			runtime:    RuntimeData{}, // No runtime
		}}
		data := make([]InstanceData, len(instances))
		for i, instance := range instances {
			data[i] = instance.Dump()
		}
		jsonData, _ := json.Marshal(data)
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		os.WriteFile(storageFile, jsonData, 0644)

		err := manager.Start(ctx, added.GetID())
		if err == nil {
			t.Error("Start() expected error for instance without runtime, got nil")
		}
		if err != nil && err.Error() != "instance has no runtime configured" {
			t.Errorf("Start() error = %v, want 'instance has no runtime configured'", err)
		}
	})

	t.Run("persists state change to storage", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager1, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added, _ := manager1.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})
		manager1.Start(ctx, added.GetID())

		// Create new manager with same storage
		manager2, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		retrieved, _ := manager2.Get(added.GetID())

		if retrieved.GetRuntimeData().State != "running" {
			t.Errorf("State from new manager = %v, want 'running'", retrieved.GetRuntimeData().State)
		}
	})

	t.Run("preserves model after start", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Model: "claude-sonnet-4-20250514"})

		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("Start() unexpected error = %v", err)
		}

		updated, _ := manager.Get(added.GetID())
		if updated.GetModel() != "claude-sonnet-4-20250514" {
			t.Errorf("After Start, GetModel() = %q, want %q", updated.GetModel(), "claude-sonnet-4-20250514")
		}
	})
}

func TestManager_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stops instance successfully", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})
		manager.Start(ctx, added.GetID())

		// Verify state is "running"
		running, _ := manager.Get(added.GetID())
		if running.GetRuntimeData().State != "running" {
			t.Errorf("Before Stop, state = %v, want 'running'", running.GetRuntimeData().State)
		}

		// Stop the instance
		err := manager.Stop(ctx, added.GetID())
		if err != nil {
			t.Fatalf("Stop() unexpected error = %v", err)
		}

		// Verify state is now "stopped"
		updated, _ := manager.Get(added.GetID())
		if updated.GetRuntimeData().State != "stopped" {
			t.Errorf("After Stop, state = %v, want 'stopped'", updated.GetRuntimeData().State)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		err := manager.Stop(context.Background(), "nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Stop() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("returns error for instance without runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		// Manually save instance without runtime to simulate old data
		instances := []Instance{&fakeInstance{
			id:         added.GetID(),
			name:       added.GetName(),
			sourceDir:  added.GetSourceDir(),
			configDir:  added.GetConfigDir(),
			accessible: true,
			runtime:    RuntimeData{}, // No runtime
		}}
		data := make([]InstanceData, len(instances))
		for i, instance := range instances {
			data[i] = instance.Dump()
		}
		jsonData, _ := json.Marshal(data)
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		os.WriteFile(storageFile, jsonData, 0644)

		err := manager.Stop(ctx, added.GetID())
		if err == nil {
			t.Error("Stop() expected error for instance without runtime, got nil")
		}
		if err != nil && err.Error() != "instance has no runtime configured" {
			t.Errorf("Stop() error = %v, want 'instance has no runtime configured'", err)
		}
	})

	t.Run("persists state change to storage", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager1, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added, _ := manager1.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})
		manager1.Start(ctx, added.GetID())
		manager1.Stop(ctx, added.GetID())

		// Create new manager with same storage
		manager2, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		retrieved, _ := manager2.Get(added.GetID())

		if retrieved.GetRuntimeData().State != "stopped" {
			t.Errorf("State from new manager = %v, want 'stopped'", retrieved.GetRuntimeData().State)
		}
	})

	t.Run("can stop created instance that was never started", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		// State is "stopped", try to stop
		err := manager.Stop(ctx, added.GetID())
		if err == nil {
			t.Error("Stop() expected error for instance in 'stopped' state, got nil")
		}
	})

	t.Run("preserves project field after Start", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		// Create git detector that returns a project
		gitDetector := &fakeGitDetector{
			repoInfo: &git.RepositoryInfo{
				RootDir:      "/repo/root",
				RemoteURL:    "https://github.com/openkaiden/kdn",
				RelativePath: "",
			},
		}
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), gitDetector)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})

		// Add instance (project should be auto-detected)
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})

		// Verify project was set
		if added.GetProject() != "https://github.com/openkaiden/kdn/" {
			t.Errorf("After Add, project = %v, want 'https://github.com/openkaiden/kdn/'", added.GetProject())
		}

		// Start the instance
		err := manager.Start(ctx, added.GetID())
		if err != nil {
			t.Fatalf("Start() unexpected error = %v", err)
		}

		// Verify project is still set after Start
		updated, _ := manager.Get(added.GetID())
		if updated.GetProject() != "https://github.com/openkaiden/kdn/" {
			t.Errorf("After Start, project = %v, want 'https://github.com/openkaiden/kdn/'", updated.GetProject())
		}
	})

	t.Run("preserves project field after Stop", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()

		// Create git detector that returns a project
		gitDetector := &fakeGitDetector{
			repoInfo: &git.RepositoryInfo{
				RootDir:      "/repo/root",
				RemoteURL:    "https://github.com/openkaiden/kdn",
				RelativePath: "",
			},
		}
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), gitDetector)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})

		// Add and start instance
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake"})
		manager.Start(ctx, added.GetID())

		// Verify project was set
		running, _ := manager.Get(added.GetID())
		if running.GetProject() != "https://github.com/openkaiden/kdn/" {
			t.Errorf("After Start, project = %v, want 'https://github.com/openkaiden/kdn/'", running.GetProject())
		}

		// Stop the instance
		err := manager.Stop(ctx, added.GetID())
		if err != nil {
			t.Fatalf("Stop() unexpected error = %v", err)
		}

		// Verify project is still set after Stop
		updated, _ := manager.Get(added.GetID())
		if updated.GetProject() != "https://github.com/openkaiden/kdn/" {
			t.Errorf("After Stop, project = %v, want 'https://github.com/openkaiden/kdn/'", updated.GetProject())
		}
	})

	t.Run("preserves model after stop", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, _ := manager.Add(ctx, AddOptions{Instance: inst, RuntimeType: "fake", Model: "claude-sonnet-4-20250514"})

		if err := manager.Start(ctx, added.GetID()); err != nil {
			t.Fatalf("Start() unexpected error = %v", err)
		}
		if err := manager.Stop(ctx, added.GetID()); err != nil {
			t.Fatalf("Stop() unexpected error = %v", err)
		}

		updated, _ := manager.Get(added.GetID())
		if updated.GetModel() != "claude-sonnet-4-20250514" {
			t.Errorf("After Stop, GetModel() = %q, want %q", updated.GetModel(), "claude-sonnet-4-20250514")
		}
	})
}

func TestManager_Reconcile(t *testing.T) {
	t.Parallel()

	t.Run("removes instances with inaccessible source directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, _ := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		// Add instance that is inaccessible
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "nonexistent-source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: false,
		})
		_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}
	})

	t.Run("removes instances with inaccessible config directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, _ := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		// Add instance that is inaccessible
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "nonexistent-config"),
			Accessible: false,
		})
		_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}
	})

	t.Run("returns list of removed IDs", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, _ := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inaccessibleSource := filepath.Join(instanceTmpDir, "nonexistent-source")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  inaccessibleSource,
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: false,
		})
		added, _ := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 || removed[0] != generatedID {
			t.Errorf("Reconcile() removed = %v, want [%v]", removed, generatedID)
		}
	})

	t.Run("keeps accessible instances", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tmpDir := t.TempDir()
		instanceTmpDir := t.TempDir()

		accessibleSource := filepath.Join(instanceTmpDir, "accessible-source")
		inaccessibleSource := filepath.Join(instanceTmpDir, "nonexistent-source")

		// Custom factory that checks source directory to determine accessibility
		mixedFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			accessible := data.Paths.Source == accessibleSource
			return &fakeInstance{
				id:         data.ID,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: accessible,
			}, nil
		}
		manager, _ := newManagerWithFactory(tmpDir, mixedFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		accessibleConfig := filepath.Join(instanceTmpDir, "accessible-config")

		// Add accessible instance
		accessible := newFakeInstance(newFakeInstanceParams{
			SourceDir:  accessibleSource,
			ConfigDir:  accessibleConfig,
			Accessible: true,
		})
		_, _ = manager.Add(ctx, AddOptions{Instance: accessible, RuntimeType: "fake"})

		// Add inaccessible instance
		inaccessible := newFakeInstance(newFakeInstanceParams{
			SourceDir:  inaccessibleSource,
			ConfigDir:  filepath.Join(instanceTmpDir, "nonexistent-config"),
			Accessible: false,
		})
		_, _ = manager.Add(ctx, AddOptions{Instance: inaccessible, RuntimeType: "fake"})

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}

		// Verify accessible instance still exists
		instances, _ := manager.List()
		if len(instances) != 1 {
			t.Errorf("List() after Reconcile() returned %d instances, want 1", len(instances))
		}
		if instances[0].GetSourceDir() != accessibleSource {
			t.Errorf("Remaining instance SourceDir = %v, want %v", instances[0].GetSourceDir(), accessibleSource)
		}
	})

	t.Run("returns empty list when all instances are accessible", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("Reconcile() removed %d instances, want 0", len(removed))
		}
	})

	t.Run("handles empty instance list", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("Reconcile() removed %d instances, want 0", len(removed))
		}
	})
}

func TestManager_Persistence(t *testing.T) {
	t.Parallel()

	t.Run("data persists across different manager instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		instanceTmpDir := t.TempDir()

		// Create first manager and add instance
		manager1, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added, _ := manager1.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		// Create second manager with same storage
		manager2, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		instances, err := manager2.List()
		if err != nil {
			t.Fatalf("List() from second manager unexpected error = %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("List() from second manager returned %d instances, want 1", len(instances))
		}
		if instances[0].GetID() != generatedID {
			t.Errorf("Instance ID = %v, want %v", instances[0].GetID(), generatedID)
		}
		if instances[0].GetSourceDir() != expectedSource {
			t.Errorf("Instance SourceDir = %v, want %v", instances[0].GetSourceDir(), expectedSource)
		}
	})

	t.Run("verifies correct JSON serialization", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added, _ := manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})

		generatedID := added.GetID()

		// Read and verify JSON content
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		data, err := os.ReadFile(storageFile)
		if err != nil {
			t.Fatalf("Failed to read storage file: %v", err)
		}

		// Unmarshal JSON data
		var instances []InstanceData
		if err := json.Unmarshal(data, &instances); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify we have exactly one instance
		if len(instances) != 1 {
			t.Fatalf("Expected 1 instance in JSON, got %d", len(instances))
		}

		// Verify the instance values
		if instances[0].ID != generatedID {
			t.Errorf("JSON ID = %v, want %v", instances[0].ID, generatedID)
		}
		if instances[0].Paths.Source != expectedSource {
			t.Errorf("JSON Paths.Source = %v, want %v", instances[0].Paths.Source, expectedSource)
		}
		if instances[0].Paths.Configuration != expectedConfig {
			t.Errorf("JSON Paths.Configuration = %v, want %v", instances[0].Paths.Configuration, expectedConfig)
		}
	})
}

func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("thread safety with concurrent Add operations", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				sourceDir := filepath.Join(instanceTmpDir, "source", string(rune('a'+id)))
				configDir := filepath.Join(instanceTmpDir, "config", string(rune('a'+id)))
				inst := newFakeInstance(newFakeInstanceParams{
					SourceDir:  sourceDir,
					ConfigDir:  configDir,
					Accessible: true,
				})
				_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})
			}(i)
		}

		wg.Wait()

		instances, _ := manager.List()
		if len(instances) != numGoroutines {
			t.Errorf("After concurrent adds, List() returned %d instances, want %d", len(instances), numGoroutines)
		}
	})

	t.Run("thread safety with concurrent reads", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		instanceTmpDir := t.TempDir()
		// Add some instances first
		for i := 0; i < 5; i++ {
			sourceDir := filepath.Join(instanceTmpDir, "source", string(rune('a'+i)))
			configDir := filepath.Join(instanceTmpDir, "config", string(rune('a'+i)))
			inst := newFakeInstance(newFakeInstanceParams{
				SourceDir:  sourceDir,
				ConfigDir:  configDir,
				Accessible: true,
			})
			_, _ = manager.Add(context.Background(), AddOptions{Instance: inst, RuntimeType: "fake"})
		}

		var wg sync.WaitGroup
		numGoroutines := 20

		// Concurrent List operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.List()
			}()
		}

		wg.Wait()

		// Verify data is still consistent
		instances, _ := manager.List()
		if len(instances) != 5 {
			t.Errorf("After concurrent reads, List() returned %d instances, want 5", len(instances))
		}
	})
}

func TestSanitizeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid name unchanged", input: "my-project", want: "my-project"},
		{name: "spaces replaced with hyphens", input: "my project", want: "my-project"},
		{name: "uppercase lowercased", input: "MyProject", want: "myproject"},
		{name: "uppercase with spaces", input: "My Project", want: "my-project"},
		{name: "name from issue 255", input: "Lemminx", want: "lemminx"},
		{name: "consecutive invalid chars collapsed", input: "foo  bar", want: "foo-bar"},
		{name: "leading and trailing hyphens trimmed", input: " -foo- ", want: "foo"},
		{name: "all invalid chars returns workspace", input: "---", want: "workspace"},
		{name: "empty string returns workspace", input: "", want: "workspace"},
		{name: "dots and underscores preserved", input: "my.project_v2", want: "my.project_v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestManager_ensureUniqueName(t *testing.T) {
	t.Parallel()

	t.Run("returns original name when no conflict", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		// Cast to concrete type to access unexported methods
		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "workspace1",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id2",
				Name:       "workspace2",
				SourceDir:  "/path/source2",
				ConfigDir:  "/path/config2",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace", result)
		}
	})

	t.Run("adds increment when name conflicts", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myworkspace",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-2" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-2", result)
		}
	})

	t.Run("increments until unique name is found", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myworkspace",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id2",
				Name:       "myworkspace-2",
				SourceDir:  "/path/source2",
				ConfigDir:  "/path/config2",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id3",
				Name:       "myworkspace-3",
				SourceDir:  "/path/source3",
				ConfigDir:  "/path/config3",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-4" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-4", result)
		}
	})

	t.Run("handles double digit increments", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		// Create instances with names up to myworkspace-10
		instances := []Instance{}
		instances = append(instances, newFakeInstance(newFakeInstanceParams{
			ID:         "id0",
			Name:       "myworkspace",
			SourceDir:  "/path/source0",
			ConfigDir:  "/path/config0",
			Accessible: true,
		}))
		for i := 2; i <= 10; i++ {
			name := fmt.Sprintf("myworkspace-%d", i)
			id := fmt.Sprintf("id%d", i)
			instances = append(instances, newFakeInstance(newFakeInstanceParams{
				ID:         id,
				Name:       name,
				SourceDir:  fmt.Sprintf("/path/source%d", i),
				ConfigDir:  fmt.Sprintf("/path/config%d", i),
				Accessible: true,
			}))
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-11" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-11", result)
		}
	})

	t.Run("works with empty instance list", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace", result)
		}
	})
}

func TestManager_generateUniqueName(t *testing.T) {
	t.Parallel()

	t.Run("generates name from source directory basename", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.generateUniqueName("/home/user/myproject", instances)

		if result != "myproject" {
			t.Errorf("generateUniqueName() = %v, want myproject", result)
		}
	})

	t.Run("handles conflicting names from different paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myproject",
				SourceDir:  "/home/user/myproject",
				ConfigDir:  "/home/user/myproject/.kaiden",
				Accessible: true,
			}),
		}

		result := mgr.generateUniqueName("/home/otheruser/myproject", instances)

		if result != "myproject-2" {
			t.Errorf("generateUniqueName() = %v, want myproject-2", result)
		}
	})

	t.Run("handles Windows-style paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		// filepath.Base works cross-platform
		result := mgr.generateUniqueName(filepath.Join("C:", "Users", "user", "myproject"), instances)

		if result != "myproject" {
			t.Errorf("generateUniqueName() = %v, want myproject", result)
		}
	})

	t.Run("handles current directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		// Get the current working directory
		wd, _ := os.Getwd()
		expectedName := sanitizeName(filepath.Base(wd))

		result := mgr.generateUniqueName(wd, instances)

		if result != expectedName {
			t.Errorf("generateUniqueName() = %v, want %v", result, expectedName)
		}
	})

	t.Run("sanitizes spaces in directory name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.generateUniqueName("/tmp/my project", instances)

		if result != "my-project" {
			t.Errorf("generateUniqueName() = %v, want my-project", result)
		}
	})

	t.Run("sanitizes uppercase directory name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.generateUniqueName("/tmp/MyProject", instances)

		if result != "myproject" {
			t.Errorf("generateUniqueName() = %v, want myproject", result)
		}
	})
}

func TestManager_mergeConfigurations(t *testing.T) {
	t.Parallel()

	t.Run("returns workspace config when no project or agent configs exist", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		// Create workspace config
		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// Should have workspace config
		if result.Environment == nil || len(*result.Environment) != 1 {
			t.Errorf("Expected 1 environment variable, got %v", result.Environment)
		}
	})

	t.Run("merges workspace and project config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create projects.json with project-specific config
		projectsJSON := `{
  "github.com/user/repo": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte(projectsJSON), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		// Create workspace config
		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		// Should have both workspace and project variables
		if result.Environment == nil || len(*result.Environment) != 2 {
			t.Fatalf("Expected 2 environment variables, got %v", result.Environment)
		}

		env := *result.Environment
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		if envMap["WORKSPACE_VAR"] != "workspace-value" {
			t.Errorf("Expected WORKSPACE_VAR=workspace-value, got %v", envMap["WORKSPACE_VAR"])
		}
		if envMap["PROJECT_VAR"] != "project-value" {
			t.Errorf("Expected PROJECT_VAR=project-value, got %v", envMap["PROJECT_VAR"])
		}
	})

	t.Run("merges workspace and global project config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create projects.json with global config only
		projectsJSON := `{
  "": {
    "environment": [
      {
        "name": "GLOBAL_VAR",
        "value": "global-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte(projectsJSON), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		// Create workspace config
		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		// Should have both workspace and global variables
		if result.Environment == nil || len(*result.Environment) != 2 {
			t.Fatalf("Expected 2 environment variables, got %v", result.Environment)
		}

		env := *result.Environment
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		if envMap["WORKSPACE_VAR"] != "workspace-value" {
			t.Errorf("Expected WORKSPACE_VAR=workspace-value, got %v", envMap["WORKSPACE_VAR"])
		}
		if envMap["GLOBAL_VAR"] != "global-value" {
			t.Errorf("Expected GLOBAL_VAR=global-value, got %v", envMap["GLOBAL_VAR"])
		}
	})

	t.Run("merges workspace, project, and agent config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create projects.json
		projectsJSON := `{
  "github.com/user/repo": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte(projectsJSON), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		// Create agents.json
		agentsJSON := `{
  "claude": {
    "environment": [
      {
        "name": "AGENT_VAR",
        "value": "agent-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		// Create workspace config
		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "claude")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		// Should have workspace, project, and agent variables
		if result.Environment == nil || len(*result.Environment) != 3 {
			t.Fatalf("Expected 3 environment variables, got %v", result.Environment)
		}

		env := *result.Environment
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		if envMap["WORKSPACE_VAR"] != "workspace-value" {
			t.Errorf("Expected WORKSPACE_VAR=workspace-value, got %v", envMap["WORKSPACE_VAR"])
		}
		if envMap["PROJECT_VAR"] != "project-value" {
			t.Errorf("Expected PROJECT_VAR=project-value, got %v", envMap["PROJECT_VAR"])
		}
		if envMap["AGENT_VAR"] != "agent-value" {
			t.Errorf("Expected AGENT_VAR=agent-value, got %v", envMap["AGENT_VAR"])
		}
	})

	t.Run("merges all config levels with proper precedence", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create projects.json with global and project-specific
		projectsJSON := `{
  "": {
    "environment": [
      {
        "name": "GLOBAL_VAR",
        "value": "global-value"
      },
      {
        "name": "OVERRIDE_ME",
        "value": "global-override"
      }
    ]
  },
  "github.com/user/repo": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      },
      {
        "name": "OVERRIDE_ME",
        "value": "project-override"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte(projectsJSON), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		// Create agents.json
		agentsJSON := `{
  "claude": {
    "environment": [
      {
        "name": "AGENT_VAR",
        "value": "agent-value"
      },
      {
        "name": "OVERRIDE_ME",
        "value": "agent-override"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(agentsJSON), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		// Create workspace config
		workspaceValue := "workspace-value"
		overrideValue := "workspace-override"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
				{Name: "OVERRIDE_ME", Value: &overrideValue},
			},
		}

		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "claude")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		if result.Environment == nil {
			t.Fatal("Expected non-nil environment")
		}

		env := *result.Environment
		envMap := make(map[string]string)
		for _, e := range env {
			if e.Value != nil {
				envMap[e.Name] = *e.Value
			}
		}

		// Check all unique variables are present
		if envMap["WORKSPACE_VAR"] != "workspace-value" {
			t.Errorf("Expected WORKSPACE_VAR=workspace-value, got %v", envMap["WORKSPACE_VAR"])
		}
		if envMap["GLOBAL_VAR"] != "global-value" {
			t.Errorf("Expected GLOBAL_VAR=global-value, got %v", envMap["GLOBAL_VAR"])
		}
		if envMap["PROJECT_VAR"] != "project-value" {
			t.Errorf("Expected PROJECT_VAR=project-value, got %v", envMap["PROJECT_VAR"])
		}
		if envMap["AGENT_VAR"] != "agent-value" {
			t.Errorf("Expected AGENT_VAR=agent-value, got %v", envMap["AGENT_VAR"])
		}

		// OVERRIDE_ME should be from agent (highest precedence)
		if envMap["OVERRIDE_ME"] != "agent-override" {
			t.Errorf("Expected OVERRIDE_ME=agent-override (from agent), got %v", envMap["OVERRIDE_ME"])
		}
	})

	t.Run("handles nil workspace config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		result, err := mgr.mergeConfigurations("github.com/user/repo", nil, "")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		// Should return empty config
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
	})

	t.Run("returns error for invalid project config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create invalid projects.json
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte("not valid json"), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		_, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "")
		if err == nil {
			t.Error("Expected error for invalid project config, got nil")
		}
	})

	t.Run("returns error for invalid agent config", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create invalid agents.json
		if err := os.WriteFile(filepath.Join(configDir, "agents.json"), []byte("not valid json"), 0644); err != nil {
			t.Fatalf("Failed to write agents.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		_, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "claude")
		if err == nil {
			t.Error("Expected error for invalid agent config, got nil")
		}
	})

	t.Run("handles empty agent name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create projects.json
		projectsJSON := `{
  "github.com/user/repo": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ]
  }
}`
		if err := os.WriteFile(filepath.Join(configDir, "projects.json"), []byte(projectsJSON), 0644); err != nil {
			t.Fatalf("Failed to write projects.json: %v", err)
		}

		m, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())
		mgr := m.(*manager)

		workspaceValue := "workspace-value"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Environment: &[]workspace.EnvironmentVariable{
				{Name: "WORKSPACE_VAR", Value: &workspaceValue},
			},
		}

		// Empty agent name should skip agent config loading
		result, err := mgr.mergeConfigurations("github.com/user/repo", workspaceCfg, "")
		if err != nil {
			t.Fatalf("mergeConfigurations() unexpected error = %v", err)
		}

		// Should have workspace and project variables only (no agent)
		if result.Environment == nil || len(*result.Environment) != 2 {
			t.Errorf("Expected 2 environment variables, got %v", result.Environment)
		}
	})
}

func TestReadAgentSettings(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when agent name is empty", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mgr := &manager{storageDir: tmpDir}

		result, err := mgr.readAgentSettings(tmpDir, "")
		if err != nil {
			t.Fatalf("readAgentSettings() unexpected error = %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil result for empty agent name, got %v", result)
		}
	})

	t.Run("returns nil when directory does not exist", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mgr := &manager{storageDir: tmpDir}

		result, err := mgr.readAgentSettings(tmpDir, "claude")
		if err != nil {
			t.Fatalf("readAgentSettings() unexpected error = %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil result when directory does not exist, got %v", result)
		}
	})

	t.Run("reads files into map with relative paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mgr := &manager{storageDir: tmpDir}

		// Create agent settings directory with a nested file
		agentDir := filepath.Join(tmpDir, "config", "claude")
		if err := os.MkdirAll(filepath.Join(agentDir, ".claude"), 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		settingsContent := []byte(`{"theme":"dark"}`)
		if err := os.WriteFile(filepath.Join(agentDir, ".claude", "settings.json"), settingsContent, 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		result, err := mgr.readAgentSettings(tmpDir, "claude")
		if err != nil {
			t.Fatalf("readAgentSettings() unexpected error = %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("Expected 1 file in result, got %d", len(result))
		}

		content, ok := result[".claude/settings.json"]
		if !ok {
			t.Error("Expected key '.claude/settings.json' in result map")
		}
		if string(content) != `{"theme":"dark"}` {
			t.Errorf("Expected content %q, got %q", `{"theme":"dark"}`, string(content))
		}
	})

	t.Run("reads multiple files recursively", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mgr := &manager{storageDir: tmpDir}

		agentDir := filepath.Join(tmpDir, "config", "claude")
		if err := os.MkdirAll(filepath.Join(agentDir, ".claude"), 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		files := map[string][]byte{
			".claude/settings.json": []byte(`{"theme":"dark"}`),
			".gitconfig":            []byte("[user]\n\tname = Agent\n"),
		}
		for relPath, content := range files {
			dest := filepath.Join(agentDir, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				t.Fatalf("Failed to create parent dir for %s: %v", relPath, err)
			}
			if err := os.WriteFile(dest, content, 0600); err != nil {
				t.Fatalf("Failed to write %s: %v", relPath, err)
			}
		}

		result, err := mgr.readAgentSettings(tmpDir, "claude")
		if err != nil {
			t.Fatalf("readAgentSettings() unexpected error = %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("Expected 2 files in result, got %d", len(result))
		}
		for relPath, expectedContent := range files {
			got, ok := result[relPath]
			if !ok {
				t.Errorf("Expected key %q in result map", relPath)
				continue
			}
			if string(got) != string(expectedContent) {
				t.Errorf("Content mismatch for %q: expected %q, got %q", relPath, expectedContent, got)
			}
		}
	})

	t.Run("returns empty map for empty directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mgr := &manager{storageDir: tmpDir}

		// Create empty agent settings directory
		agentDir := filepath.Join(tmpDir, "config", "claude")
		if err := os.MkdirAll(agentDir, 0755); err != nil {
			t.Fatalf("Failed to create agent directory: %v", err)
		}

		result, err := mgr.readAgentSettings(tmpDir, "claude")
		if err != nil {
			t.Fatalf("readAgentSettings() unexpected error = %v", err)
		}

		if len(result) != 0 {
			t.Errorf("Expected empty map for empty directory, got %d entries", len(result))
		}
	})
}

// invalidStateRuntime is a test runtime that returns invalid states for testing boundary validation
type invalidStateRuntime struct {
	createState string
	startState  string
	infoState   string
}

func (r *invalidStateRuntime) Type() string {
	return "invalid-state-runtime"
}

func (r *invalidStateRuntime) WorkspaceSourcesPath() string {
	return "/workspace/sources"
}

func (r *invalidStateRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	state := r.createState
	if state == "" {
		state = "invalid-created-state"
	}
	return runtime.RuntimeInfo{
		ID:    "test-instance-id",
		State: api.WorkspaceState(state),
		Info:  map[string]string{},
	}, nil
}

func (r *invalidStateRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	state := r.startState
	if state == "" {
		state = "invalid-started-state"
	}
	return runtime.RuntimeInfo{
		ID:    id,
		State: api.WorkspaceState(state),
		Info:  map[string]string{},
	}, nil
}

func (r *invalidStateRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (r *invalidStateRuntime) Remove(ctx context.Context, id string) error {
	return nil
}

func (r *invalidStateRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	state := r.infoState
	if state == "" {
		state = "invalid-info-state"
	}
	return runtime.RuntimeInfo{
		ID:    id,
		State: api.WorkspaceState(state),
		Info:  map[string]string{},
	}, nil
}

// Compile-time check to ensure invalidStateRuntime implements runtime.Runtime
var _ runtime.Runtime = (*invalidStateRuntime)(nil)

func TestManager_Add_RejectsInvalidState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		createState string
		wantErr     string
	}{
		{
			name:        "rejects created state",
			createState: "created",
			wantErr:     `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "created" (must be one of: running, stopped, error, unknown)`,
		},
		{
			name:        "rejects arbitrary invalid state",
			createState: "foo",
			wantErr:     `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "foo" (must be one of: running, stopped, error, unknown)`,
		},
		{
			name:        "rejects paused state",
			createState: "paused",
			wantErr:     `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "paused" (must be one of: running, stopped, error, unknown)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storageDir := t.TempDir()
			manager, err := NewManager(storageDir)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			// Register the invalid state runtime
			invalidRT := &invalidStateRuntime{createState: tt.createState}
			if err := manager.RegisterRuntime(invalidRT); err != nil {
				t.Fatalf("Failed to register runtime: %v", err)
			}

			// Create test directories
			instanceTmpDir := t.TempDir()
			sourceDir := filepath.Join(instanceTmpDir, "source")
			configDir := filepath.Join(instanceTmpDir, "config")
			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Fatalf("Failed to create source directory: %v", err)
			}
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatalf("Failed to create config directory: %v", err)
			}

			inst, err := NewInstance(NewInstanceParams{
				SourceDir: sourceDir,
				ConfigDir: configDir,
			})
			if err != nil {
				t.Fatalf("Failed to create instance: %v", err)
			}

			// Attempt to add instance - should fail with invalid state
			_, err = manager.Add(context.Background(), AddOptions{
				Instance:    inst,
				RuntimeType: "invalid-state-runtime",
				Agent:       "test-agent",
			})

			if err == nil {
				t.Fatal("Expected error when adding instance with invalid state, got nil")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("Add() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestManager_Add_AppliesAgentOnboarding(t *testing.T) {
	t.Parallel()

	t.Run("calls agent SkipOnboarding when agent is registered", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register Claude agent
		claudeAgent := agent.NewClaude()
		if err := manager.RegisterAgent("claude", claudeAgent); err != nil {
			t.Fatalf("Failed to register claude agent: %v", err)
		}

		// Create agent settings directory with initial settings
		agentDir := filepath.Join(storageDir, "config", "claude")
		if err := os.MkdirAll(filepath.Join(agentDir, ".claude"), 0755); err != nil {
			t.Fatalf("Failed to create agent settings directory: %v", err)
		}

		// Write initial settings without onboarding flags
		initialSettings := []byte(`{"theme":"dark"}`)
		if err := os.WriteFile(filepath.Join(agentDir, ".claude", "settings.json"), initialSettings, 0600); err != nil {
			t.Fatalf("Failed to write initial settings: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with claude agent
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "claude",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify instance was created
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		// Verify the runtime received modified agent settings
		// Since we're using the fake runtime, we can't directly inspect what was passed,
		// but we can verify the agent settings file was read and the instance was created successfully
		if added.GetID() == "" {
			t.Error("Add() returned instance with empty ID")
		}
	})

	t.Run("handles agent not in registry gracefully", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Create agent settings directory
		agentDir := filepath.Join(storageDir, "config", "unknown-agent")
		if err := os.MkdirAll(filepath.Join(agentDir, ".config"), 0755); err != nil {
			t.Fatalf("Failed to create agent settings directory: %v", err)
		}

		// Write settings file
		settings := []byte(`{"key":"value"}`)
		if err := os.WriteFile(filepath.Join(agentDir, ".config", "settings.json"), settings, 0600); err != nil {
			t.Fatalf("Failed to write settings: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with unknown agent - should succeed and use settings as-is
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "unknown-agent",
		})
		if err != nil {
			t.Fatalf("Add() with unknown agent error = %v", err)
		}

		if added == nil {
			t.Fatal("Add() returned nil instance")
		}
	})

	t.Run("handles no agent settings directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register Claude agent
		claudeAgent := agent.NewClaude()
		if err := manager.RegisterAgent("claude", claudeAgent); err != nil {
			t.Fatalf("Failed to register claude agent: %v", err)
		}

		// Do NOT create agent settings directory

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with claude agent but no settings - should succeed
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "claude",
		})
		if err != nil {
			t.Fatalf("Add() with no agent settings error = %v", err)
		}

		if added == nil {
			t.Fatal("Add() returned nil instance")
		}
	})

	t.Run("propagates SkipOnboarding errors", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register Claude agent
		claudeAgent := agent.NewClaude()
		if err := manager.RegisterAgent("claude", claudeAgent); err != nil {
			t.Fatalf("Failed to register claude agent: %v", err)
		}

		// Create agent settings directory with invalid JSON
		agentDir := filepath.Join(storageDir, "config", "claude")
		if err := os.MkdirAll(agentDir, 0755); err != nil {
			t.Fatalf("Failed to create agent settings directory: %v", err)
		}

		// Write invalid JSON settings to .claude.json
		invalidSettings := []byte(`{invalid json}`)
		if err := os.WriteFile(filepath.Join(agentDir, ".claude.json"), invalidSettings, 0600); err != nil {
			t.Fatalf("Failed to write invalid settings: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with claude agent and invalid settings - should fail
		_, err = manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "claude",
		})
		if err == nil {
			t.Fatal("Add() with invalid agent settings should return error")
		}

		// Verify error message mentions agent onboarding
		if !strings.Contains(err.Error(), "agent onboarding") {
			t.Errorf("Add() error = %q, want error containing 'agent onboarding'", err.Error())
		}
	})
}

func TestManager_Add_AppliesAgentModel(t *testing.T) {
	t.Parallel()

	t.Run("calls agent SetModel when model is specified", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register tracking agent to verify SetModel is called
		trackingAgent := newTrackingAgent("test-agent")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with agent and model
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "test-agent",
			Model:       "model-from-flag",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify instance was created
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		// Verify SetModel was called
		if !trackingAgent.WasSetModelCalled() {
			t.Error("SetModel() was not called")
		}

		// Verify SetModel was called with the correct model ID
		if trackingAgent.GetSetModelID() != "model-from-flag" {
			t.Errorf("SetModel() called with model ID %q, want %q", trackingAgent.GetSetModelID(), "model-from-flag")
		}

		// Verify SkipOnboarding was also called
		if !trackingAgent.WasSkipOnboardingCalled() {
			t.Error("SkipOnboarding() was not called")
		}
	})

	t.Run("does not call SetModel when model is empty", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register tracking agent to verify SetModel is not called
		trackingAgent := newTrackingAgent("test-agent")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with agent but no model
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "test-agent",
			Model:       "", // Empty model
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify instance was created
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		// Verify SetModel was NOT called
		if trackingAgent.WasSetModelCalled() {
			t.Error("SetModel() should not be called when model is empty")
		}

		// Verify SkipOnboarding was still called
		if !trackingAgent.WasSkipOnboardingCalled() {
			t.Error("SkipOnboarding() was not called")
		}
	})

	t.Run("stores model in returned instance when specified", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Model:       "claude-sonnet-4-20250514",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		if added.GetModel() != "claude-sonnet-4-20250514" {
			t.Errorf("GetModel() = %q, want %q", added.GetModel(), "claude-sonnet-4-20250514")
		}

		// Verify model is persisted: reload and check
		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(instances) != 1 {
			t.Fatalf("Expected 1 instance, got %d", len(instances))
		}
		if instances[0].GetModel() != "claude-sonnet-4-20250514" {
			t.Errorf("GetModel() after reload = %q, want %q", instances[0].GetModel(), "claude-sonnet-4-20250514")
		}
	})

	t.Run("stores empty model in returned instance when not specified", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			// Model intentionally omitted
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		if added.GetModel() != "" {
			t.Errorf("GetModel() = %q, want empty string", added.GetModel())
		}
	})

	t.Run("does not call SetModel when agent is not registered", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Note: Not registering any agent

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with unknown agent and model - should succeed (agent settings used as-is)
		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "unknown-agent",
			Model:       "some-model",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify instance was created
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		if added.GetID() == "" {
			t.Error("Add() returned instance with empty ID")
		}
	})

	t.Run("propagates SetModel errors", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		// Register fake runtime
		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		// Register agent that errors on SetModel but not SkipOnboarding
		erroringAgent := newErroringSetModelAgent("erroring-agent")
		if err := manager.RegisterAgent("erroring-agent", erroringAgent); err != nil {
			t.Fatalf("Failed to register erroring agent: %v", err)
		}

		// Create test instance
		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		// Add instance with erroring agent and model - should fail due to SetModel error
		_, err = manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "erroring-agent",
			Model:       "some-model",
		})
		if err == nil {
			t.Fatal("Add() should return error when SetModel fails")
		}

		// Verify error message mentions agent model settings
		if !strings.Contains(err.Error(), "agent model settings") {
			t.Errorf("Add() error = %q, want error containing 'agent model settings'", err.Error())
		}

		// Verify the underlying error is propagated
		if !strings.Contains(err.Error(), "simulated SetModel error") {
			t.Errorf("Add() error = %q, want error containing 'simulated SetModel error'", err.Error())
		}
	})
}

func TestManager_Start_RejectsInvalidState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		startState string
		wantErr    string
	}{
		{
			name:       "rejects created state",
			startState: "created",
			wantErr:    `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "created" (must be one of: running, stopped, error, unknown)`,
		},
		{
			name:       "rejects arbitrary invalid state",
			startState: "booting",
			wantErr:    `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "booting" (must be one of: running, stopped, error, unknown)`,
		},
		{
			name:       "rejects exited state",
			startState: "exited",
			wantErr:    `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "exited" (must be one of: running, stopped, error, unknown)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storageDir := t.TempDir()
			manager, err := NewManager(storageDir)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			// Register a runtime that returns valid state for Create but invalid for Start
			mixedRT := &invalidStateRuntime{
				createState: "stopped", // Valid for Create
				startState:  tt.startState,
			}
			if err := manager.RegisterRuntime(mixedRT); err != nil {
				t.Fatalf("Failed to register runtime: %v", err)
			}

			// Create test directories
			instanceTmpDir := t.TempDir()
			sourceDir := filepath.Join(instanceTmpDir, "source")
			configDir := filepath.Join(instanceTmpDir, "config")
			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Fatalf("Failed to create source directory: %v", err)
			}
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatalf("Failed to create config directory: %v", err)
			}

			inst, err := NewInstance(NewInstanceParams{
				SourceDir: sourceDir,
				ConfigDir: configDir,
			})
			if err != nil {
				t.Fatalf("Failed to create instance: %v", err)
			}

			// Add instance (should succeed with valid state)
			added, err := manager.Add(context.Background(), AddOptions{
				Instance:    inst,
				RuntimeType: "invalid-state-runtime",
				Agent:       "test-agent",
			})
			if err != nil {
				t.Fatalf("Failed to add instance: %v", err)
			}

			// Start instance - should fail with invalid state
			err = manager.Start(context.Background(), added.GetID())

			if err == nil {
				t.Fatal("Expected error when starting instance with invalid state, got nil")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("Start() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestManager_Stop_RejectsInvalidState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		infoState string
		wantErr   string
	}{
		{
			name:      "rejects created state from Info",
			infoState: "created",
			wantErr:   `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "created" (must be one of: running, stopped, error, unknown)`,
		},
		{
			name:      "rejects arbitrary invalid state from Info",
			infoState: "stopping",
			wantErr:   `runtime "invalid-state-runtime" returned invalid state: invalid runtime state: "stopping" (must be one of: running, stopped, error, unknown)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storageDir := t.TempDir()
			manager, err := NewManager(storageDir)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			// Register a runtime that returns valid states for Create/Start but invalid for Info
			mixedRT := &invalidStateRuntime{
				createState: "stopped", // Valid for Create
				startState:  "running", // Valid for Start
				infoState:   tt.infoState,
			}
			if err := manager.RegisterRuntime(mixedRT); err != nil {
				t.Fatalf("Failed to register runtime: %v", err)
			}

			// Create test directories
			instanceTmpDir := t.TempDir()
			sourceDir := filepath.Join(instanceTmpDir, "source")
			configDir := filepath.Join(instanceTmpDir, "config")
			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Fatalf("Failed to create source directory: %v", err)
			}
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatalf("Failed to create config directory: %v", err)
			}

			inst, err := NewInstance(NewInstanceParams{
				SourceDir: sourceDir,
				ConfigDir: configDir,
			})
			if err != nil {
				t.Fatalf("Failed to create instance: %v", err)
			}

			// Add instance (should succeed)
			added, err := manager.Add(context.Background(), AddOptions{
				Instance:    inst,
				RuntimeType: "invalid-state-runtime",
				Agent:       "test-agent",
			})
			if err != nil {
				t.Fatalf("Failed to add instance: %v", err)
			}

			// Start instance (should succeed)
			if err := manager.Start(context.Background(), added.GetID()); err != nil {
				t.Fatalf("Failed to start instance: %v", err)
			}

			// Stop instance - should fail because Info() returns invalid state
			err = manager.Stop(context.Background(), added.GetID())

			if err == nil {
				t.Fatal("Expected error when stopping instance with invalid state from Info, got nil")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("Stop() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// spyRuntime is a test double for runtime.Runtime that captures the params passed to Create.
type spyRuntime struct {
	wrapped          runtime.Runtime
	lastCreateParams runtime.CreateParams
	mu               sync.Mutex
}

var _ runtime.Runtime = (*spyRuntime)(nil)

func newSpyRuntime(wrapped runtime.Runtime) *spyRuntime {
	return &spyRuntime{wrapped: wrapped}
}

func (s *spyRuntime) Type() string { return s.wrapped.Type() }

func (s *spyRuntime) WorkspaceSourcesPath() string { return s.wrapped.WorkspaceSourcesPath() }

func (s *spyRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
	s.mu.Lock()
	s.lastCreateParams = params
	s.mu.Unlock()
	return s.wrapped.Create(ctx, params)
}

func (s *spyRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return s.wrapped.Start(ctx, id)
}

func (s *spyRuntime) Stop(ctx context.Context, id string) error { return s.wrapped.Stop(ctx, id) }

func (s *spyRuntime) Remove(ctx context.Context, id string) error {
	return s.wrapped.Remove(ctx, id)
}

func (s *spyRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
	return s.wrapped.Info(ctx, id)
}

func (s *spyRuntime) LastCreateParams() runtime.CreateParams {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCreateParams
}

func TestManager_Add_ConvertsSkillsToMounts(t *testing.T) {
	t.Parallel()

	t.Run("skills are converted to mounts using agent SkillsDir", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		spy := newSpyRuntime(fake.New())
		if err := manager.RegisterRuntime(spy); err != nil {
			t.Fatalf("Failed to register spy runtime: %v", err)
		}

		trackingAgent := newTrackingAgentWithSkillsDir("test-agent", "$HOME/.claude/skills")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		skillsPath := "/absolute/path/to/skills"
		workspaceCfg := &workspace.WorkspaceConfiguration{
			Skills: &[]string{skillsPath},
		}

		_, err = manager.Add(context.Background(), AddOptions{
			Instance:        inst,
			RuntimeType:     "fake",
			Agent:           "test-agent",
			WorkspaceConfig: workspaceCfg,
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		params := spy.LastCreateParams()
		if params.WorkspaceConfig == nil || params.WorkspaceConfig.Mounts == nil {
			t.Fatal("Expected mounts to be set in CreateParams")
		}

		mounts := *params.WorkspaceConfig.Mounts
		if len(mounts) != 1 {
			t.Fatalf("Expected 1 mount, got %d: %v", len(mounts), mounts)
		}

		if mounts[0].Host != skillsPath {
			t.Errorf("Mount host = %q, want %q", mounts[0].Host, skillsPath)
		}
		wantTarget := "$HOME/.claude/skills/skills"
		if mounts[0].Target != wantTarget {
			t.Errorf("Mount target = %q, want %q", mounts[0].Target, wantTarget)
		}
		if mounts[0].Ro == nil || !*mounts[0].Ro {
			t.Error("Expected mount to be read-only")
		}
	})

	t.Run("duplicate skill basenames produce an error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		spy := newSpyRuntime(fake.New())
		if err := manager.RegisterRuntime(spy); err != nil {
			t.Fatalf("Failed to register spy runtime: %v", err)
		}

		trackingAgent := newTrackingAgentWithSkillsDir("test-agent", "$HOME/.claude/skills")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		workspaceCfg := &workspace.WorkspaceConfiguration{
			Skills: &[]string{"/path/one/my-skill", "/path/two/my-skill"},
		}

		_, err = manager.Add(context.Background(), AddOptions{
			Instance:        inst,
			RuntimeType:     "fake",
			Agent:           "test-agent",
			WorkspaceConfig: workspaceCfg,
		})
		if err == nil {
			t.Fatal("Expected Add() to return an error for duplicate skill targets, got nil")
		}
	})

	t.Run("skills are not converted when agent has no SkillsDir", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		spy := newSpyRuntime(fake.New())
		if err := manager.RegisterRuntime(spy); err != nil {
			t.Fatalf("Failed to register spy runtime: %v", err)
		}

		trackingAgent := newTrackingAgentWithSkillsDir("test-agent", "")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		workspaceCfg := &workspace.WorkspaceConfiguration{
			Skills: &[]string{"/some/skills"},
		}

		_, err = manager.Add(context.Background(), AddOptions{
			Instance:        inst,
			RuntimeType:     "fake",
			Agent:           "test-agent",
			WorkspaceConfig: workspaceCfg,
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		params := spy.LastCreateParams()
		if params.WorkspaceConfig != nil && params.WorkspaceConfig.Mounts != nil {
			t.Errorf("Expected no mounts when agent has empty SkillsDir, got %v", *params.WorkspaceConfig.Mounts)
		}
	})
}

func TestManager_Add_AppliesAgentMCPServers(t *testing.T) {
	t.Parallel()

	t.Run("calls SetMCPServers when workspace config has MCP", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		trackingAgent := newTrackingAgent("test-agent")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		mcp := &workspace.McpConfiguration{
			Commands: &[]workspace.McpCommand{
				{Name: "my-tool", Command: "mytool"},
			},
		}

		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "test-agent",
			WorkspaceConfig: &workspace.WorkspaceConfiguration{
				Mcp: mcp,
			},
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		if !trackingAgent.WasSetMCPServersCalled() {
			t.Error("SetMCPServers() was not called")
		}
	})

	t.Run("does not call SetMCPServers when workspace config has no MCP", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		manager, err := NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if err := manager.RegisterRuntime(fake.New()); err != nil {
			t.Fatalf("Failed to register fake runtime: %v", err)
		}

		trackingAgent := newTrackingAgent("test-agent")
		if err := manager.RegisterAgent("test-agent", trackingAgent); err != nil {
			t.Fatalf("Failed to register tracking agent: %v", err)
		}

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		added, err := manager.Add(context.Background(), AddOptions{
			Instance:    inst,
			RuntimeType: "fake",
			Agent:       "test-agent",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}

		if trackingAgent.WasSetMCPServersCalled() {
			t.Error("SetMCPServers() should not be called when no MCP config is provided")
		}
	})
}

// fakeSecretServiceImpl is a test implementation of the SecretService interface
type fakeSecretServiceImpl struct {
	name           string
	hostPattern    string
	path           string
	envVars        []string
	headerName     string
	headerTemplate string
}

func (f *fakeSecretServiceImpl) Name() string           { return f.name }
func (f *fakeSecretServiceImpl) HostPattern() string    { return f.hostPattern }
func (f *fakeSecretServiceImpl) Path() string           { return f.path }
func (f *fakeSecretServiceImpl) EnvVars() []string      { return f.envVars }
func (f *fakeSecretServiceImpl) HeaderName() string     { return f.headerName }
func (f *fakeSecretServiceImpl) HeaderTemplate() string { return f.headerTemplate }

func TestManager_RegisterSecretService(t *testing.T) {
	t.Parallel()

	t.Run("registers secret service successfully", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		svc := &fakeSecretServiceImpl{name: "github", hostPattern: `github\.com`, headerName: "Authorization"}

		err := manager.RegisterSecretService(svc)
		if err != nil {
			t.Errorf("RegisterSecretService() error = %v, want nil", err)
		}
	})

	t.Run("returns error for duplicate registration", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, _ := newManagerWithFactory(tmpDir, fakeInstanceFactory, newFakeGenerator(), newTestRegistry(tmpDir), agent.NewRegistry(), secretservice.NewRegistry(), newFakeGitDetector())

		svc1 := &fakeSecretServiceImpl{name: "github", headerName: "Authorization"}
		svc2 := &fakeSecretServiceImpl{name: "github", headerName: "X-Token"}

		err := manager.RegisterSecretService(svc1)
		if err != nil {
			t.Fatalf("First RegisterSecretService() error = %v, want nil", err)
		}

		err = manager.RegisterSecretService(svc2)
		if err == nil {
			t.Error("RegisterSecretService() duplicate should return error")
		}
	})
}
