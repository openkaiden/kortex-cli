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
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	api "github.com/openkaiden/kdn-api/cli/go"
	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/agent"
	"github.com/openkaiden/kdn/pkg/config"
	"github.com/openkaiden/kdn/pkg/containerurl"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/generator"
	"github.com/openkaiden/kdn/pkg/git"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/project"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/secret"
	"github.com/openkaiden/kdn/pkg/secretservice"
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
	// WorkspaceConfig is the workspace-level configuration from .kaiden/workspace.json (optional, can be nil)
	WorkspaceConfig *workspace.WorkspaceConfiguration
	// Project is an optional custom project identifier to override auto-detection
	Project string
	// Agent is an optional agent name for loading agent-specific configuration
	Agent string
	// Model is an optional model ID to configure for the agent
	Model string
	// RuntimeOptions contains runtime-specific flag values from the CLI.
	RuntimeOptions map[string]string
}

// Manager handles instance storage and operations
type Manager interface {
	// Add registers a new instance with a runtime and returns the instance with its generated ID
	Add(ctx context.Context, opts AddOptions) (Instance, error)
	// Start starts a runtime instance by ID
	Start(ctx context.Context, id string) error
	// Stop stops a runtime instance by ID
	Stop(ctx context.Context, id string) error
	// Terminal starts an interactive terminal session, auto-starting the instance if needed
	Terminal(ctx context.Context, id string, command []string) error
	// List returns all registered instances
	List() ([]Instance, error)
	// Get retrieves a specific instance by name or ID
	Get(nameOrID string) (Instance, error)
	// Delete unregisters an instance by ID
	Delete(ctx context.Context, id string) error
	// Reconcile removes instances with inaccessible directories
	// Returns the list of removed instance IDs
	Reconcile() ([]string, error)
	// GetDashboardURL returns the dashboard URL for a workspace instance by name or ID.
	// Returns ErrInstanceNotFound if the workspace does not exist.
	// Returns ErrDashboardNotSupported if the runtime does not implement the Dashboard interface.
	GetDashboardURL(ctx context.Context, nameOrID string) (string, error)
	// GetRuntime retrieves a registered runtime by type.
	// Returns an error if the runtime type is not registered.
	GetRuntime(runtimeType string) (runtime.Runtime, error)
	// RegisterRuntime registers a runtime with the manager's registry
	RegisterRuntime(rt runtime.Runtime) error
	// RegisterAgent registers an agent with the manager's registry
	RegisterAgent(name string, agent agent.Agent) error
	// RegisterSecretService registers a secret service with the manager's registry
	RegisterSecretService(service secretservice.SecretService) error
	// RegisterCredential registers a credential with the manager's registry
	RegisterCredential(c credential.Credential) error
}

// manager is the internal implementation of Manager
type manager struct {
	storageFile           string
	storageDir            string
	mu                    sync.RWMutex
	factory               InstanceFactory
	generator             generator.Generator
	runtimeRegistry       runtime.Registry
	agentRegistry         agent.Registry
	secretServiceRegistry secretservice.Registry
	credentialRegistry    credential.Registry
	secretStore           secret.Store
	projectDetector       project.Detector
	now                   func() time.Time
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
	agentReg := agent.NewRegistry()
	secretServiceReg := secretservice.NewRegistry()
	credReg := credential.NewRegistry()
	secretStore := secret.NewStore(storageDir)
	return newManagerWithFactory(storageDir, NewInstanceFromData, generator.New(), reg, agentReg, secretServiceReg, credReg, secretStore, project.NewDetector(git.NewDetector()), time.Now)
}

// newManagerWithFactory creates a new instance manager with a custom instance factory, generator, registry, and project detector.
// This is unexported and primarily useful for testing with fake instances, generators, runtimes, and project detectors.
func newManagerWithFactory(storageDir string, factory InstanceFactory, gen generator.Generator, reg runtime.Registry, agentReg agent.Registry, secretServiceReg secretservice.Registry, credReg credential.Registry, secretStore secret.Store, detector project.Detector, clock func() time.Time) (Manager, error) {
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
	if agentReg == nil {
		return nil, errors.New("agent registry cannot be nil")
	}
	if secretServiceReg == nil {
		return nil, errors.New("secret service registry cannot be nil")
	}
	if credReg == nil {
		return nil, errors.New("credential registry cannot be nil")
	}
	if secretStore == nil {
		return nil, errors.New("secret store cannot be nil")
	}
	if detector == nil {
		return nil, errors.New("project detector cannot be nil")
	}
	if clock == nil {
		return nil, errors.New("clock cannot be nil")
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}

	storageFile := filepath.Join(storageDir, DefaultStorageFileName)
	mgr := &manager{
		storageFile:           storageFile,
		storageDir:            storageDir,
		factory:               factory,
		generator:             gen,
		runtimeRegistry:       reg,
		agentRegistry:         agentReg,
		secretServiceRegistry: secretServiceReg,
		credentialRegistry:    credReg,
		secretStore:           secretStore,
		projectDetector:       detector,
		now:                   clock,
	}

	if err := mgr.migrateTimestamps(); err != nil {
		return nil, fmt.Errorf("failed to migrate instance timestamps: %w", err)
	}

	return mgr, nil
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
		// Ensure the provided name is sanitized and unique
		name = m.ensureUniqueName(sanitizeName(name), instances)
	}

	// Use custom project identifier if provided, otherwise auto-detect
	project := opts.Project
	if project == "" {
		project = m.projectDetector.DetectProject(ctx, inst.GetSourceDir())
	}

	// Merge configurations from all levels: workspace -> project (global + specific) -> agent
	mergedConfig, err := m.mergeConfigurations(project, opts.WorkspaceConfig, opts.Agent)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configurations: %w", err)
	}

	// Get the runtime
	rt, err := m.runtimeRegistry.Get(opts.RuntimeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	// Build a URL rewriter for container networking. On WSL2 this probes
	// each service port to determine whether host.containers.internal or
	// native-host.internal is reachable. On other platforms it defaults to
	// host.containers.internal.
	var rewriter containerurl.URLRewriter = containerurl.DefaultRewriter()
	if provider, ok := rt.(runtime.ContainerURLRewriterProvider); ok {
		rewriter = provider.BuildURLRewriter(ctx)
	}

	// Allow the runtime to transform the workspace config for its environment
	// (e.g. rewriting localhost URLs for container networking).
	if transformer, ok := rt.(runtime.ConfigTransformer); ok && mergedConfig != nil {
		if err := transformer.TransformConfig(mergedConfig, rewriter); err != nil {
			return nil, fmt.Errorf("failed to transform workspace config: %w", err)
		}
	}

	// Map workspace secrets to OneCLI secret definitions and collect env vars
	var onecliSecrets []onecli.CreateSecretInput
	secretEnvVars := make(map[string]string)
	if mergedConfig != nil && mergedConfig.Secrets != nil && len(*mergedConfig.Secrets) > 0 {
		mapper := onecli.NewSecretMapper(m.secretServiceRegistry)
		for i, name := range *mergedConfig.Secrets {
			item, value, err := m.secretStore.Get(name)
			if err != nil {
				return nil, fmt.Errorf("failed to get secret %q at index %d: %w", name, i, err)
			}
			inputs, err := mapper.Map(item, value)
			if err != nil {
				return nil, fmt.Errorf("failed to map secret %q at index %d: %w", name, i, err)
			}
			onecliSecrets = append(onecliSecrets, inputs...)

			if item.Type != secret.TypeOther {
				if svc, svcErr := m.secretServiceRegistry.Get(item.Type); svcErr == nil {
					for _, envVar := range svc.EnvVars() {
						secretEnvVars[envVar] = "placeholder"
					}
				}
			} else {
				for _, envVar := range item.Envs {
					secretEnvVars[envVar] = "placeholder"
				}
			}
		}
	}

	// Collect unique placeholder values for pre-approving injected API keys
	seen := make(map[string]struct{})
	var approvedKeys []string
	for _, v := range secretEnvVars {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			approvedKeys = append(approvedKeys, v)
		}
	}

	// Read agent settings files from storage config directory
	agentSettings, err := m.readAgentSettings(m.storageDir, opts.Agent)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent settings: %w", err)
	}

	// Modify agent settings to skip onboarding if agent has implementation
	if opts.Agent != "" {
		if agentImpl, err := m.agentRegistry.Get(opts.Agent); err == nil {
			// Get workspace sources path from runtime before creating instance
			workspaceSourcesPath := rt.WorkspaceSourcesPath()
			agentSettings, err = agentImpl.SkipOnboarding(agentSettings, workspaceSourcesPath, approvedKeys)
			if err != nil {
				return nil, fmt.Errorf("failed to apply agent onboarding settings: %w", err)
			}

			// Set model if specified
			if opts.Model != "" {
				agentSettings, err = agentImpl.SetModel(agentSettings, opts.Model)
				if err != nil {
					return nil, fmt.Errorf("failed to apply agent model settings: %w", err)
				}
			}

			// Convert skills directories to mounts using the agent's skills directory
			if skillsDir := agentImpl.SkillsDir(); skillsDir != "" && mergedConfig != nil && mergedConfig.Skills != nil {
				roTrue := true
				seenTargets := make(map[string]string)
				for _, skillsPath := range *mergedConfig.Skills {
					target := path.Join(skillsDir, filepath.Base(skillsPath))
					if existing, ok := seenTargets[target]; ok {
						return nil, fmt.Errorf("skills %q and %q have the same target directory %q", existing, skillsPath, target)
					}
					seenTargets[target] = skillsPath
					mount := workspace.Mount{
						Host:   skillsPath,
						Target: target,
						Ro:     &roTrue,
					}
					if mergedConfig.Mounts == nil {
						mounts := []workspace.Mount{mount}
						mergedConfig.Mounts = &mounts
					} else {
						*mergedConfig.Mounts = append(*mergedConfig.Mounts, mount)
					}
				}
			}
			// Configure MCP servers if specified in workspace config
			if mergedConfig != nil && mergedConfig.Mcp != nil {
				agentSettings, err = agentImpl.SetMCPServers(agentSettings, mergedConfig.Mcp)
				if err != nil {
					return nil, fmt.Errorf("failed to apply agent MCP server settings: %w", err)
				}
			}
		}
		// If agent not found in registry, use settings as-is (not all agents may be implemented)
	}

	// Rewrite localhost URLs in agent settings for container reachability.
	// This is a runtime concern — agents produce localhost URLs and the
	// runtime rewrites them based on network topology.
	agentSettings = containerurl.RewriteSettings(agentSettings, rewriter)

	// Create runtime instance with merged configuration
	runtimeInfo, err := rt.Create(ctx, runtime.CreateParams{
		Name:               name,
		SourcePath:         inst.GetSourceDir(),
		WorkspaceConfig:    mergedConfig,
		WorkspaceConfigDir: inst.GetConfigDir(),
		Agent:              opts.Agent,
		AgentSettings:      agentSettings,
		OnecliSecrets:      onecliSecrets,
		SecretEnvVars:      secretEnvVars,
		ProjectID:          project,
		RuntimeOptions:     opts.RuntimeOptions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime instance: %w", err)
	}

	// Validate state at boundary
	if err := runtime.ValidateState(runtimeInfo.State); err != nil {
		return nil, fmt.Errorf("runtime %q returned invalid state: %w", opts.RuntimeType, err)
	}

	// Create a new instance with the unique ID, name, runtime info, project, agent, and model
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
		Project:   project,
		Agent:     opts.Agent,
		Model:     opts.Model,
		CreatedAt: m.now(),
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

	// Validate state at boundary
	if err := runtime.ValidateState(runtimeInfo.State); err != nil {
		return fmt.Errorf("runtime %q returned invalid state: %w", runtimeData.Type, err)
	}

	// Merge info: preserve existing fields (e.g. onecli_web_port set at Create time),
	// then overlay any updated fields returned by Start.
	mergedInfo := make(map[string]string, len(runtimeData.Info)+len(runtimeInfo.Info))
	maps.Copy(mergedInfo, runtimeData.Info)
	maps.Copy(mergedInfo, runtimeInfo.Info)

	startedAt := m.now()

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
			Info:       mergedInfo,
		},
		Project:   instanceToStart.GetProject(),
		Agent:     instanceToStart.GetAgent(),
		Model:     instanceToStart.GetModel(),
		CreatedAt: instanceToStart.GetCreatedAt(),
		StartedAt: startedAt,
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

	// Validate state at boundary
	if err := runtime.ValidateState(runtimeInfo.State); err != nil {
		return fmt.Errorf("runtime %q returned invalid state: %w", runtimeData.Type, err)
	}

	// Merge info: preserve existing fields (e.g. onecli_web_port set at Create time),
	// then overlay any updated fields returned by Stop.
	mergedInfo := make(map[string]string, len(runtimeData.Info)+len(runtimeInfo.Info))
	maps.Copy(mergedInfo, runtimeData.Info)
	maps.Copy(mergedInfo, runtimeInfo.Info)

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
			Info:       mergedInfo,
		},
		Project:   instanceToStop.GetProject(),
		Agent:     instanceToStop.GetAgent(),
		Model:     instanceToStop.GetModel(),
		CreatedAt: instanceToStop.GetCreatedAt(),
		// StartedAt is intentionally zero on stop: the instance is no longer running
	}

	instances[index] = updatedInstance
	return m.saveInstances(instances)
}

// Terminal starts an interactive terminal session in a running instance.
// If the instance is not running, it will be automatically started first.
func (m *manager) Terminal(ctx context.Context, id string, command []string) error {
	// Check instance state without holding the lock for the entire operation,
	// because Start() needs to acquire a write lock.
	runtimeData, err := m.terminalCheckState(id)
	if err != nil {
		return err
	}

	// Auto-start the instance if it is not running
	if runtimeData.State != api.WorkspaceStateRunning {
		if err := m.Start(ctx, id); err != nil {
			return fmt.Errorf("failed to auto-start instance: %w", err)
		}
	}

	// Re-read the instance under RLock and connect to terminal
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances, err := m.loadInstances()
	if err != nil {
		return err
	}

	var instanceToConnect Instance
	for _, instance := range instances {
		if instance.GetID() == id {
			instanceToConnect = instance
			break
		}
	}

	if instanceToConnect == nil {
		return ErrInstanceNotFound
	}

	runtimeData = instanceToConnect.GetRuntimeData()

	// Get the runtime
	rt, err := m.runtimeRegistry.Get(runtimeData.Type)
	if err != nil {
		return fmt.Errorf("failed to get runtime: %w", err)
	}

	// Type-assert to Terminal interface
	terminalRT, ok := rt.(runtime.Terminal)
	if !ok {
		return fmt.Errorf("runtime %s does not support terminal sessions", runtimeData.Type)
	}

	// Start terminal session, passing the agent name
	return terminalRT.Terminal(ctx, runtimeData.InstanceID, instanceToConnect.GetAgent(), command)
}

// terminalCheckState validates that the instance exists and has a runtime configured.
// It returns the runtime data for state inspection.
func (m *manager) terminalCheckState(id string) (RuntimeData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances, err := m.loadInstances()
	if err != nil {
		return RuntimeData{}, err
	}

	for _, instance := range instances {
		if instance.GetID() == id {
			runtimeData := instance.GetRuntimeData()
			if runtimeData.Type == "" || runtimeData.InstanceID == "" {
				return RuntimeData{}, errors.New("instance has no runtime configured")
			}
			return runtimeData, nil
		}
	}

	return RuntimeData{}, ErrInstanceNotFound
}

// List returns all registered instances
func (m *manager) List() ([]Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadInstances()
}

// Get retrieves a specific instance by name or ID.
// It first attempts to match by ID, then falls back to matching by name.
func (m *manager) Get(nameOrID string) (Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances, err := m.loadInstances()
	if err != nil {
		return nil, err
	}

	// Try ID match first (backward compatible)
	for _, instance := range instances {
		if instance.GetID() == nameOrID {
			return instance, nil
		}
	}

	// Fall back to name match
	for _, instance := range instances {
		if instance.GetName() == nameOrID {
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

// GetDashboardURL returns the dashboard URL for a workspace instance by name or ID.
func (m *manager) GetDashboardURL(ctx context.Context, nameOrID string) (string, error) {
	instance, err := m.Get(nameOrID)
	if err != nil {
		return "", err
	}

	runtimeData := instance.GetRuntimeData()
	if runtimeData.State != api.WorkspaceStateRunning {
		return "", fmt.Errorf("workspace is not running (state: %s)", runtimeData.State)
	}

	rt, err := m.runtimeRegistry.Get(runtimeData.Type)
	if err != nil {
		return "", fmt.Errorf("failed to get runtime: %w", err)
	}

	dashboardRuntime, ok := rt.(runtime.Dashboard)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrDashboardNotSupported, runtimeData.Type)
	}

	return dashboardRuntime.GetURL(ctx, runtimeData.InstanceID)
}

// GetRuntime retrieves a registered runtime by type.
func (m *manager) GetRuntime(runtimeType string) (runtime.Runtime, error) {
	return m.runtimeRegistry.Get(runtimeType)
}

// RegisterRuntime registers a runtime with the manager's registry.
func (m *manager) RegisterRuntime(rt runtime.Runtime) error {
	if err := m.runtimeRegistry.Register(rt); err != nil {
		return err
	}
	if aware, ok := rt.(runtime.SecretServiceRegistryAware); ok {
		aware.SetSecretServiceRegistry(m.secretServiceRegistry)
	}
	if aware, ok := rt.(runtime.CredentialRegistryAware); ok {
		aware.SetCredentialRegistry(m.credentialRegistry)
	}
	return nil
}

// RegisterAgent registers an agent with the manager's registry.
func (m *manager) RegisterAgent(name string, agent agent.Agent) error {
	return m.agentRegistry.Register(name, agent)
}

// RegisterSecretService registers a secret service with the manager's registry.
func (m *manager) RegisterSecretService(service secretservice.SecretService) error {
	return m.secretServiceRegistry.Register(service)
}

// RegisterCredential registers a credential with the manager's registry.
func (m *manager) RegisterCredential(c credential.Credential) error {
	return m.credentialRegistry.Register(c)
}

var invalidNameChars = regexp.MustCompile(`[^a-z0-9._-]+`)

// sanitizeName converts a name to a valid workspace name by lowercasing it
// and replacing invalid characters with hyphens.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = invalidNameChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-._")
	if name == "" {
		return "workspace"
	}
	return name
}

// generateUniqueName generates a unique name from the source directory
// by extracting the last component of the path and adding an increment if needed
func (m *manager) generateUniqueName(sourceDir string, instances []Instance) string {
	baseName := filepath.Base(sourceDir)
	return m.ensureUniqueName(sanitizeName(baseName), instances)
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

// mergeConfigurations loads and merges configurations from all levels:
// workspace -> project (global + specific) -> agent
func (m *manager) mergeConfigurations(projectID string, workspaceConfig *workspace.WorkspaceConfiguration, agentName string) (*workspace.WorkspaceConfiguration, error) {
	merger := config.NewMerger()
	result := workspaceConfig

	// Load project-specific configuration (includes global config merged with project-specific)
	projectLoader, err := config.NewProjectConfigLoader(m.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create project config loader: %w", err)
	}

	projectConfig, err := projectLoader.Load(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load project configuration: %w", err)
	}

	// Merge workspace config with project config
	result = merger.Merge(result, projectConfig)

	// Load agent-specific configuration if agent name is specified
	if agentName != "" {
		agentLoader, err := config.NewAgentConfigLoader(m.storageDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent config loader: %w", err)
		}

		agentConfig, err := agentLoader.Load(agentName)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent configuration: %w", err)
		}

		// Merge with agent config (highest precedence)
		result = merger.Merge(result, agentConfig)
	}

	return result, nil
}

// migrateTimestamps backfills CreatedAt for instances persisted before timestamp
// support was added. It runs once at manager construction, computes the missing
// timestamp using the injected clock, and writes the result back to disk so
// subsequent loads see the authoritative value.
func (m *manager) migrateTimestamps() error {
	instances, err := m.loadInstances()
	if err != nil {
		return err
	}

	migrated := false
	for i, inst := range instances {
		if inst.GetCreatedAt().IsZero() {
			data := inst.Dump()
			data.CreatedAt = m.now()
			updated, err := m.factory(data)
			if err != nil {
				return err
			}
			instances[i] = updated
			migrated = true
		}
	}

	if migrated {
		return m.saveInstances(instances)
	}
	return nil
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

// readAgentSettings reads all files from {storageDir}/config/{agentName}/ into a map.
// Keys are relative paths using forward slashes; values are file contents.
// Returns nil (no error) if the directory does not exist.
func (m *manager) readAgentSettings(storageDir, agentName string) (map[string][]byte, error) {
	if agentName == "" {
		return nil, nil
	}

	// Validate agentName to prevent path traversal
	if strings.Contains(agentName, "/") || strings.Contains(agentName, "\\") || strings.Contains(agentName, "..") {
		return nil, fmt.Errorf("invalid agent name: %q", agentName)
	}

	agentSettingsDir := filepath.Join(storageDir, "config", agentName)
	if _, err := os.Stat(agentSettingsDir); os.IsNotExist(err) {
		return nil, nil
	}

	settings := make(map[string][]byte)
	err := fs.WalkDir(os.DirFS(agentSettingsDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(filepath.Join(agentSettingsDir, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("failed to read agent settings file %s: %w", path, err)
		}
		settings[path] = content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk agent settings directory: %w", err)
	}

	return settings, nil
}
