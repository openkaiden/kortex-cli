---
name: working-with-runtime-system
description: Guide to understanding and working with the kdn runtime system architecture
argument-hint: ""
---

# Working with the Runtime System

The runtime system provides a pluggable architecture for managing workspaces on different container/VM platforms (Podman, MicroVM, Kubernetes, etc.). This skill provides detailed guidance on understanding and working with the runtime system.

## Overview

The runtime system enables kdn to support multiple backend platforms through a common interface. Each runtime implementation handles the platform-specific details of creating, starting, stopping, and managing workspace instances.

## Key Components

- **Runtime Interface** (`pkg/runtime/runtime.go`): Contract all runtimes must implement
- **Registry** (`pkg/runtime/registry.go`): Manages runtime registration and discovery
- **Runtime Implementations** (`pkg/runtime/<runtime-name>/`): Platform-specific packages (e.g., `fake`)
- **Centralized Registration** (`pkg/runtimesetup/register.go`): Automatically registers all available runtimes

## Runtime Registration in Commands

Commands use `runtimesetup.RegisterAll()` to automatically register all available runtimes:

```go
import "github.com/openkaiden/kdn/pkg/runtimesetup"

// In command preRun
manager, err := instances.NewManager(storageDir)
if err != nil {
    return err
}

// Register all available runtimes
if err := runtimesetup.RegisterAll(manager); err != nil {
    return err
}
```

This automatically registers all runtimes from `pkg/runtimesetup/register.go` that report as available (e.g., only registers Podman if `podman` CLI is installed).

## Optional Runtime Interfaces

Some runtimes may implement additional optional interfaces to provide extended functionality beyond the base Runtime interface. These are checked at runtime using type assertions, allowing runtimes to opt-in to features they support.

### StorageAware Interface

The StorageAware interface enables runtimes to persist data in a dedicated storage directory managed by the registry.

```go
type StorageAware interface {
    Initialize(storageDir string) error
}
```

**How it works:**

When a runtime implements StorageAware, the registry will:
1. Create a directory at `<registry-storage>/<runtime-type>/`
2. Call `Initialize(storageDir)` with the path
3. The runtime can use this directory to persist instance data, configuration, or other state

**Example implementation:**

```go
type myRuntime struct {
    storageDir  string
    storageFile string
    instances   map[string]Instance
}

// Implement StorageAware
func (r *myRuntime) Initialize(storageDir string) error {
    r.storageDir = storageDir
    r.storageFile = filepath.Join(storageDir, "instances.json")

    // Load existing state from disk
    return r.loadFromDisk()
}

func (r *myRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
    // ... create instance logic ...

    // Persist to storage directory
    if err := r.saveToDisk(); err != nil {
        return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist instance: %w", err)
    }

    return info, nil
}
```

### Listing Runtimes

`runtimesetup.ListRuntimes()` returns structured information about all available runtimes (excluding the internal `fake` runtime). It is used by `kdn runtime list`.

The function relies on two mandatory methods that all runtimes must implement:

```go
// Description returns a human-readable description of the runtime.
Description() string

// Local reports whether the runtime executes workspaces on the local machine.
Local() bool
```

### AgentLister Interface

The AgentLister interface enables runtimes to report which agents they support. This is used by the `info` command to discover available agents without requiring direct knowledge of runtime-specific configuration.

```go
type AgentLister interface {
    ListAgents() ([]string, error)
}
```

**How it works:**

When a runtime implements AgentLister, the `runtimesetup.ListAgents()` function will:
1. Create a registry and register all available runtimes (triggering StorageAware initialization)
2. Query each runtime that implements AgentLister
3. Collect and deduplicate agent names across all runtimes

**Example implementation (Podman runtime):**

```go
func (p *podmanRuntime) ListAgents() ([]string, error) {
    if p.config == nil {
        return []string{}, nil
    }
    return p.config.ListAgents()
}
```

This pattern decouples agent discovery from runtime-specific configuration details, allowing the `info` command to query agents generically through the runtime interface.

### FlagProvider Interface

The FlagProvider interface enables runtimes to declare CLI flags that appear on the `init` command. This decouples runtime-specific options from the command layer.

```go
type FlagDef struct {
    Name        string
    Usage       string
    Completions []string
}

type FlagProvider interface {
    Flags() []FlagDef
}
```

**How it works:**

When a runtime implements FlagProvider:
1. `runtimesetup.ListFlags()` discovers and deduplicates flags from all available FlagProvider runtimes
2. The `init` command registers them as cobra flags (with shell completions if `Completions` is non-empty)
3. Changed flag values are collected into a `map[string]string` and passed through `AddOptions.RuntimeOptions` → `CreateParams.RuntimeOptions`
4. The runtime reads its values from `params.RuntimeOptions` in `Create()`

**Example implementation:**

```go
func (r *myRuntime) Flags() []runtime.FlagDef {
    return []runtime.FlagDef{{
        Name:        "my-driver",
        Usage:       "Driver to use (podman, vm)",
        Completions: []string{"podman", "vm"},
    }}
}

func (r *myRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
    driver := params.RuntimeOptions["my-driver"]
    // ... use driver value ...
}
```

### Terminal Interface

The Terminal interface enables interactive terminal sessions for connecting to running instances. This is used by the `terminal` command.

```go
type Terminal interface {
    // Terminal starts an interactive terminal session inside a running instance.
    // The agent parameter is used to load agent-specific configuration for the terminal session.
    // The command is executed with stdin/stdout/stderr connected directly to the user's terminal.
    Terminal(ctx context.Context, agent string, instanceID string, command []string) error
}
```

**Example implementation (Podman runtime):**

```go
func (p *podmanRuntime) Terminal(ctx context.Context, agent string, instanceID string, command []string) error {
    if agent == "" {
        return fmt.Errorf("%w: agent is required", runtime.ErrInvalidParams)
    }
    if instanceID == "" {
        return fmt.Errorf("%w: instance ID is required", runtime.ErrInvalidParams)
    }
    if len(command) == 0 {
        return fmt.Errorf("%w: command is required", runtime.ErrInvalidParams)
    }

    // Build podman exec -it <container> <command...>
    args := []string{"exec", "-it", instanceID}
    args = append(args, command...)

    return p.executor.RunInteractive(ctx, args...)
}
```

**How optional interfaces work:**

The Terminal interface follows the same pattern as `StorageAware` - it's optional, and runtimes that don't support interactive sessions simply don't implement it. The instances manager checks for Terminal support at runtime using type assertion:

```go
if terminalRuntime, ok := runtime.(Terminal); ok {
    return terminalRuntime.Terminal(ctx, agent, instanceID, command)
}
return errors.New("runtime does not support terminal sessions")
```

This pattern allows runtimes to provide additional capabilities without requiring all runtimes to implement every possible feature.

## State Validation

All runtimes must return valid WorkspaceState values in `RuntimeInfo.State`. The instances manager enforces validation at the boundary using a **fail-fast approach**.

### Valid States

The following four states are the only valid values (defined in `github.com/openkaiden/kdn-api/cli/go`):

- **`running`** - The instance is actively running
- **`stopped`** - The instance is created but not running
- **`error`** - The instance encountered an error
- **`unknown`** - The instance state cannot be determined

### Boundary Validation (Manager Layer)

The instances manager validates all `RuntimeInfo` values returned from runtimes at three boundaries:

1. **`Add()`** - validates state after `runtime.Create()`
2. **`Start()`** - validates state after `runtime.Start()`
3. **`Stop()`** - validates state after `runtime.Info()`

If a runtime returns an invalid state, the manager immediately returns an error:

```go
// In pkg/instances/manager.go
runtimeInfo, err := rt.Create(ctx, params)
if err != nil {
    return nil, fmt.Errorf("failed to create runtime instance: %w", err)
}

// Validate state at boundary
if err := runtime.ValidateState(runtimeInfo.State); err != nil {
    return nil, fmt.Errorf("runtime %q returned invalid state: %w", runtimeType, err)
}
```

**Benefits of boundary validation:**
- ✅ **Centralized enforcement** - validation in one place, not N runtimes
- ✅ **Runtime developers can't forget** - automatic validation
- ✅ **Fail-fast** - bugs caught during development with clear error messages
- ✅ **Single source of truth** - manager is the gatekeeper

### State Mapping in Runtimes

Runtime implementations must map platform-specific states to the four valid states. You do **NOT** need to call `runtime.ValidateState()` yourself - the manager does this automatically.

**Example: Podman runtime state mapping**

```go
// In pkg/runtime/podman/info.go
func mapPodmanState(podmanState string) api.WorkspaceState {
    switch podmanState {
    case "running":
        return api.WorkspaceStateRunning
    case "created", "exited", "stopped", "paused", "removing":
        return api.WorkspaceStateStopped
    case "dead":
        return api.WorkspaceStateError
    default:
        return api.WorkspaceStateUnknown
    }
}

func (p *podmanRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
    // Get podman-specific state
    podmanState := getPodmanContainerState(id)
    
    // Map to valid WorkspaceState (no validation needed - manager handles it)
    state := mapPodmanState(podmanState)
    
    return runtime.RuntimeInfo{
        ID:    id,
        State: state,
        Info:  info,
    }, nil
}
```

### Error Messages

If a runtime returns an invalid state, the error message clearly identifies the problem:

```text
runtime "my-runtime" returned invalid state: invalid runtime state: "created" 
(must be one of: running, stopped, error, unknown)
```

This tells you:
1. Which runtime has the bug
2. What invalid state was returned
3. What the valid states are

### Best Practices

- **Map platform states** to valid WorkspaceState values in your runtime
- **Return "stopped"** for newly created instances, not platform-specific values like "created"
- **Don't call `runtime.ValidateState()`** yourself - the manager handles this
- **Document your mapping logic** for maintainability
- **Write tests** with invalid states to verify the manager catches them

**Reference implementation:** See `pkg/runtime/podman/info.go` for complete state mapping example

**Boundary validation tests:** See `pkg/instances/manager_test.go` for tests that verify the manager rejects invalid states

## Mount Path Requirements

When implementing a new runtime, the container-side path used for `$SOURCES` must **not be a direct child of `/`**.

- `/sources` — **not ok**: `$SOURCES/..` resolves to `/`, which means the containment check in `pkg/config/config.go` would accept any path as a sibling mount, including escaping paths like `/etc`
- `/workspace/sources` — **ok**: `$SOURCES/..` resolves to `/workspace`, a safe shared root for sibling repos
- `/mnt/sources` — **ok**
- `/mnt/sub/sources` — **ok**

Users can mount sibling source directories using `$SOURCES/../sibling`. The containment check validates that `$SOURCES`-based targets stay within the **parent** of `$SOURCES`. If `$SOURCES` is mounted at a root-level directory, that parent is `/` and the check provides no protection.

There is no specific depth requirement for `$HOME`.

**Podman runtime paths (reference):**

```go
var containerWorkspaceSources = path.Join("/workspace", "sources") // parent is /workspace ✓
var containerHome = path.Join("/home", constants.ContainerUser)    // no depth constraint
```

## Adding a New Runtime

Use the `/add-runtime` skill which provides step-by-step instructions for creating a new runtime implementation. The `fake` runtime in `pkg/runtime/fake/` serves as a reference implementation.

## Related Skills

- `/add-runtime` - Step-by-step guide to create a new runtime implementation
- `/working-with-steplogger` - Add progress feedback to runtime operations
- `/working-with-podman-runtime-config` - Configure the Podman runtime

## References

- **Runtime Interface**: `pkg/runtime/runtime.go`
- **Registry**: `pkg/runtime/registry.go`
- **Registration**: `pkg/runtimesetup/register.go`
- **Reference Implementation**: `pkg/runtime/fake/`
- **Podman Implementation**: `pkg/runtime/podman/`
