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
	"sync"

	workspace "github.com/kortex-hub/kortex-cli-api/workspace-configuration/go"
	"github.com/kortex-hub/kortex-cli/pkg/generator"
	"github.com/kortex-hub/kortex-cli/pkg/runtime"
)

const (
	// DefaultStorageFileName is the default filename for storing instances
	DefaultStorageFileName = "instances.json"
	// RuntimesSubdirectory is the subdirectory for runtime storage
	RuntimesSubdirectory = "runtimes"
)

// InstanceFactory is a function that creates an Instance from InstanceData
type InstanceFactory func(InstanceData) (Instance, error)

// AddOptions contains parameters for adding a new instance
type AddOptions struct {
	// Instance is the instance to add
	Instance Instance
	// RuntimeType is the type of runtime to use
	RuntimeType string
	// WorkspaceConfig is the loaded workspace configuration (optional, can be nil)
	WorkspaceConfig *workspace.WorkspaceConfiguration
}

// Manager handles instance storage and operations
type Manager interface {
	// Add registers a new instance with a runtime and returns the instance with its generated ID
	Add(ctx context.Context, opts AddOptions) (Instance, error)
	// Start starts a runtime instance by ID
	Start(ctx context.Context, id string) error
	// Stop stops a runtime instance by ID
	Stop(ctx context.Context, id string) error
	// List returns all registered instances
	List() ([]Instance, error)
	// Get retrieves a specific instance by ID
	Get(id string) (Instance, error)
	// Delete unregisters an instance by ID
	Delete(ctx context.Context, id string) error
	// Reconcile removes instances with inaccessible directories
	// Returns the list of removed instance IDs
	Reconcile() ([]string, error)
	// RegisterRuntime registers a runtime with the manager's registry
	RegisterRuntime(rt runtime.Runtime) error
}

// manager is the internal implementation of Manager
type manager struct {
	storageFile     string
	mu              sync.RWMutex
	factory         InstanceFactory
	generator       generator.Generator
	runtimeRegistry runtime.Registry
}

// Compile-time check to ensure manager implements Manager interface
var _ Manager = (*manager)(nil)

// NewManager creates a new instance manager with the given storage directory.
func NewManager(storageDir string) (Manager, error) {
	runtimesDir := filepath.Join(storageDir, RuntimesSubdirectory)
	reg, err := runtime.NewRegistry(runtimesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime registry: %w", err)
	}
	return newManagerWithFactory(storageDir, NewInstanceFromData, generator.New(), reg)
}

// newManagerWithFactory creates a new instance manager with a custom instance factory, generator, and registry.
// This is unexported and primarily useful for testing with fake instances, generators, and runtimes.
func newManagerWithFactory(storageDir string, factory InstanceFactory, gen generator.Generator, reg runtime.Registry) (Manager, error) {
	if storageDir == "" {
		return nil, errors.New("storage directory cannot be empty")
	}
	if factory == nil {
		return nil, errors.New("factory cannot be nil")
	}
	if gen == nil {
		return nil, errors.New("generator cannot be nil")
	}
	if reg == nil {
		return nil, errors.New("registry cannot be nil")
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}

	storageFile := filepath.Join(storageDir, DefaultStorageFileName)
	return &manager{
		storageFile:     storageFile,
		factory:         factory,
		generator:       gen,
		runtimeRegistry: reg,
	}, nil
}

// Add registers a new instance with a runtime.
// The instance must be created using NewInstance to ensure proper validation.
// A unique ID is generated for the instance when it's added to storage.
// If the instance name is empty, a unique name is generated from the source directory.
// The runtime instance is created but not started.
// Returns the instance with its generated ID, name, and runtime information.
func (m *manager) Add(ctx context.Context, opts AddOptions) (Instance, error) {
	if opts.Instance == nil {
		return nil, errors.New("instance cannot be nil")
	}
	if opts.RuntimeType == "" {
		return nil, errors.New("runtime type cannot be empty")
	}

	inst := opts.Instance

	m.mu.Lock()
	defer m.mu.Unlock()

	instances, err := m.loadInstances()
	if err != nil {
		return nil, err
	}

	// Generate a unique ID for the instance
	var uniqueID string
	for {
		uniqueID = m.generator.Generate()
		// Check if this ID is already in use
		duplicate := false
		for _, existing := range instances {
			if existing.GetID() == uniqueID {
				duplicate = true
				break
			}
		}
		if !duplicate {
			break
		}
	}

	// Generate a unique name if not provided
	name := inst.GetName()
	if name == "" {
		name = m.generateUniqueName(inst.GetSourceDir(), instances)
	} else {
		// Ensure the provided name is unique
		name = m.ensureUniqueName(name, instances)
	}

	// Get the runtime
	rt, err := m.runtimeRegistry.Get(opts.RuntimeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	// Create runtime instance
	runtimeInfo, err := rt.Create(ctx, runtime.CreateParams{
		Name:            name,
		SourcePath:      inst.GetSourceDir(),
		WorkspaceConfig: opts.WorkspaceConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime instance: %w", err)
	}

	// Create a new instance with the unique ID, name, and runtime info
	instanceWithID := &instance{
		ID:        uniqueID,
		Name:      name,
		SourceDir: inst.GetSourceDir(),
		ConfigDir: inst.GetConfigDir(),
		Runtime: RuntimeData{
			Type:       opts.RuntimeType,
			InstanceID: runtimeInfo.ID,
			State:      runtimeInfo.State,
			Info:       runtimeInfo.Info,
		},
	}

	instances = append(instances, instanceWithID)
	if err := m.saveInstances(instances); err != nil {
		return nil, err
	}

	return instanceWithID, nil
}

// Start starts a runtime instance by ID.
func (m *manager) Start(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instances, err := m.loadInstances()
	if err != nil {
		return err
	}

	// Find the instance
	var instanceToStart Instance
	var index int
	found := false
	for i, instance := range instances {
		if instance.GetID() == id {
			instanceToStart = instance
			index = i
			found = true
			break
		}
	}

	if !found {
		return ErrInstanceNotFound
	}

	runtimeData := instanceToStart.GetRuntimeData()
	if runtimeData.Type == "" || runtimeData.InstanceID == "" {
		return errors.New("instance has no runtime configured")
	}

	// Get the runtime
	rt, err := m.runtimeRegistry.Get(runtimeData.Type)
	if err != nil {
		return fmt.Errorf("failed to get runtime: %w", err)
	}

	// Start the runtime instance
	runtimeInfo, err := rt.Start(ctx, runtimeData.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to start runtime instance: %w", err)
	}

	// Update the instance with new runtime state
	updatedInstance := &instance{
		ID:        instanceToStart.GetID(),
		Name:      instanceToStart.GetName(),
		SourceDir: instanceToStart.GetSourceDir(),
		ConfigDir: instanceToStart.GetConfigDir(),
		Runtime: RuntimeData{
			Type:       runtimeData.Type,
			InstanceID: runtimeData.InstanceID,
			State:      runtimeInfo.State,
			Info:       runtimeInfo.Info,
		},
	}

	instances[index] = updatedInstance
	return m.saveInstances(instances)
}

// Stop stops a runtime instance by ID.
func (m *manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instances, err := m.loadInstances()
	if err != nil {
		return err
	}

	// Find the instance
	var instanceToStop Instance
	var index int
	found := false
	for i, instance := range instances {
		if instance.GetID() == id {
			instanceToStop = instance
			index = i
			found = true
			break
		}
	}

	if !found {
		return ErrInstanceNotFound
	}

	runtimeData := instanceToStop.GetRuntimeData()
	if runtimeData.Type == "" || runtimeData.InstanceID == "" {
		return errors.New("instance has no runtime configured")
	}

	// Get the runtime
	rt, err := m.runtimeRegistry.Get(runtimeData.Type)
	if err != nil {
		return fmt.Errorf("failed to get runtime: %w", err)
	}

	// Stop the runtime instance
	err = rt.Stop(ctx, runtimeData.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to stop runtime instance: %w", err)
	}

	// Get updated runtime info
	runtimeInfo, err := rt.Info(ctx, runtimeData.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get runtime info: %w", err)
	}

	// Update the instance with new runtime state
	updatedInstance := &instance{
		ID:        instanceToStop.GetID(),
		Name:      instanceToStop.GetName(),
		SourceDir: instanceToStop.GetSourceDir(),
		ConfigDir: instanceToStop.GetConfigDir(),
		Runtime: RuntimeData{
			Type:       runtimeData.Type,
			InstanceID: runtimeData.InstanceID,
			State:      runtimeInfo.State,
			Info:       runtimeInfo.Info,
		},
	}

	instances[index] = updatedInstance
	return m.saveInstances(instances)
}

// List returns all registered instances
func (m *manager) List() ([]Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadInstances()
}

// Get retrieves a specific instance by ID
func (m *manager) Get(id string) (Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances, err := m.loadInstances()
	if err != nil {
		return nil, err
	}

	// Look up by ID
	for _, instance := range instances {
		if instance.GetID() == id {
			return instance, nil
		}
	}

	return nil, ErrInstanceNotFound
}

// Delete unregisters an instance by ID.
// Before removing from storage, it removes the runtime instance.
// If runtime removal fails, the instance is NOT removed from storage and an error is returned.
func (m *manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instances, err := m.loadInstances()
	if err != nil {
		return err
	}

	// Find the instance to delete
	var instanceToDelete Instance
	found := false
	filtered := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.GetID() != id {
			filtered = append(filtered, instance)
		} else {
			instanceToDelete = instance
			found = true
		}
	}

	if !found {
		return ErrInstanceNotFound
	}

	// Runtime cleanup
	runtimeInfo := instanceToDelete.GetRuntimeData()
	if runtimeInfo.Type != "" && runtimeInfo.InstanceID != "" {
		rt, err := m.runtimeRegistry.Get(runtimeInfo.Type)
		if err != nil {
			return fmt.Errorf("failed to get runtime: %w", err)
		}
		// Remove runtime instance (must succeed before removing from storage)
		if err := rt.Remove(ctx, runtimeInfo.InstanceID); err != nil {
			return fmt.Errorf("failed to remove runtime instance: %w", err)
		}
	}

	// Remove from manager storage
	return m.saveInstances(filtered)
}

// Reconcile removes instances with inaccessible directories
// Returns the list of removed instance IDs
func (m *manager) Reconcile() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instances, err := m.loadInstances()
	if err != nil {
		return nil, err
	}

	removed := []string{}
	accessible := make([]Instance, 0, len(instances))

	for _, instance := range instances {
		if instance.IsAccessible() {
			accessible = append(accessible, instance)
		} else {
			removed = append(removed, instance.GetID())
		}
	}

	if len(removed) > 0 {
		if err := m.saveInstances(accessible); err != nil {
			return nil, err
		}
	}

	return removed, nil
}

// RegisterRuntime registers a runtime with the manager's registry.
func (m *manager) RegisterRuntime(rt runtime.Runtime) error {
	return m.runtimeRegistry.Register(rt)
}

// generateUniqueName generates a unique name from the source directory
// by extracting the last component of the path and adding an increment if needed
func (m *manager) generateUniqueName(sourceDir string, instances []Instance) string {
	// Extract the last component of the source directory
	baseName := filepath.Base(sourceDir)
	return m.ensureUniqueName(baseName, instances)
}

// ensureUniqueName ensures the name is unique by adding an increment if needed
func (m *manager) ensureUniqueName(name string, instances []Instance) string {
	// Check if the name is already in use
	nameExists := func(checkName string) bool {
		for _, inst := range instances {
			if inst.GetName() == checkName {
				return true
			}
		}
		return false
	}

	// If the name is not in use, return it
	if !nameExists(name) {
		return name
	}

	// Find a unique name by adding an increment
	counter := 2
	for {
		uniqueName := fmt.Sprintf("%s-%d", name, counter)
		if !nameExists(uniqueName) {
			return uniqueName
		}
		counter++
	}
}

// loadInstances reads instances from the storage file
func (m *manager) loadInstances() ([]Instance, error) {
	// If file doesn't exist, return empty list
	if _, err := os.Stat(m.storageFile); os.IsNotExist(err) {
		return []Instance{}, nil
	}

	data, err := os.ReadFile(m.storageFile)
	if err != nil {
		return nil, err
	}

	// Empty file case
	if len(data) == 0 {
		return []Instance{}, nil
	}

	// Unmarshal into InstanceData slice
	var instancesData []InstanceData
	if err := json.Unmarshal(data, &instancesData); err != nil {
		return nil, err
	}

	// Convert to Instance slice using the factory
	instances := make([]Instance, len(instancesData))
	for i, data := range instancesData {
		inst, err := m.factory(data)
		if err != nil {
			return nil, err
		}
		instances[i] = inst
	}

	return instances, nil
}

// saveInstances writes instances to the storage file
func (m *manager) saveInstances(instances []Instance) error {
	// Convert to InstanceData slice for marshaling
	instancesData := make([]InstanceData, len(instances))
	for i, inst := range instances {
		instancesData[i] = inst.Dump()
	}

	data, err := json.MarshalIndent(instancesData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.storageFile, data, 0644)
}
