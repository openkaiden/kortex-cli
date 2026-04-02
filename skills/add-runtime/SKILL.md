---
name: add-runtime
description: Add a new runtime implementation to the kortex-cli runtime system
argument-hint: <runtime-name>
---

# Add Runtime Skill

This skill guides you through adding a new runtime implementation to the kortex-cli runtime system.

## What are Runtimes?

Runtimes provide the execution environment for workspaces on different container/VM platforms:
- **Podman**: Container-based workspaces
- **MicroVM**: Lightweight VM-based workspaces
- **Kubernetes**: Kubernetes pod-based workspaces
- **fake**: Test runtime for development

## Steps to Add a New Runtime

### 1. Create Runtime Package

Create a new directory: `pkg/runtime/<runtime-name>/`

Example: `pkg/runtime/podman/`

### 2. Implement the Runtime Interface

Create `pkg/runtime/<runtime-name>/<runtime-name>.go` with:

```go
package <runtime-name>

import (
    "context"
    "fmt"
    api "github.com/kortex-hub/kortex-cli-api/cli/go"
    "github.com/kortex-hub/kortex-cli/pkg/runtime"
    "github.com/kortex-hub/kortex-cli/pkg/logger"  // Optional: only if executing external commands
    "github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

type <runtime-name>Runtime struct {
    storageDir string
}

// Ensure implementation of runtime.Runtime at compile time
var _ runtime.Runtime = (*<runtime-name>Runtime)(nil)

// Ensure implementation of runtime.StorageAware at compile time (optional)
var _ runtime.StorageAware = (*<runtime-name>Runtime)(nil)

// New creates a new runtime instance
func New() runtime.Runtime {
    return &<runtime-name>Runtime{}
}

// Type returns the runtime type identifier
func (r *<runtime-name>Runtime) Type() string {
    return "<runtime-name>"
}

// Initialize implements runtime.StorageAware (optional)
func (r *<runtime-name>Runtime) Initialize(storageDir string) error {
    r.storageDir = storageDir
    // Optional: create subdirectories, load state, etc.
    return nil
}

// Available implements runtimesetup.Available (optional)
func (r *<runtime-name>Runtime) Available() bool {
    // Check if the runtime is available on this system
    // Example: check if CLI tool is installed
    _, err := exec.LookPath("<runtime-cli-tool>")
    return err == nil
}

// Create creates a new runtime instance
func (r *<runtime-name>Runtime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
    stepLogger := steplogger.FromContext(ctx)
    defer stepLogger.Complete()

    // Step 1: Prepare environment
    stepLogger.Start("Preparing workspace environment", "Workspace environment prepared")
    if err := r.prepareEnvironment(params); err != nil {
        stepLogger.Fail(err)
        return runtime.RuntimeInfo{}, err
    }

    // Step 2: Create instance
    stepLogger.Start("Creating workspace instance", "Workspace instance created")
    info, err := r.createInstance(ctx, params)
    if err != nil {
        stepLogger.Fail(err)
        return runtime.RuntimeInfo{}, err
    }

    // Step 3: Copy agent default settings files into the workspace home directory.
    // params.AgentSettings is a map[string][]byte (relative forward-slash path → content)
    // populated from <storage-dir>/config/<agent>/ by the instances manager.
    // For image-based runtimes (e.g., Podman), embed these files BEFORE install RUN commands
    // so agent install scripts can read and build upon the defaults.
    if len(params.AgentSettings) > 0 {
        stepLogger.Start("Copying agent settings to workspace", "Agent settings copied")
        if err := r.copyAgentSettings(ctx, info.ID, params.AgentSettings); err != nil {
            stepLogger.Fail(err)
            return runtime.RuntimeInfo{}, err
        }
    }

    return info, nil
}

// Start starts a runtime instance
func (r *<runtime-name>Runtime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
    stepLogger := steplogger.FromContext(ctx)
    defer stepLogger.Complete()

    stepLogger.Start(fmt.Sprintf("Starting workspace: %s", id), "Workspace started")
    if err := r.startInstance(ctx, id); err != nil {
        stepLogger.Fail(err)
        return runtime.RuntimeInfo{}, err
    }

    stepLogger.Start("Verifying workspace status", "Workspace status verified")
    info, err := r.getInfo(ctx, id)
    if err != nil {
        stepLogger.Fail(err)
        return runtime.RuntimeInfo{}, err
    }

    return info, nil
}

// Stop stops a runtime instance
func (r *<runtime-name>Runtime) Stop(ctx context.Context, id string) error {
    stepLogger := steplogger.FromContext(ctx)
    defer stepLogger.Complete()

    stepLogger.Start(fmt.Sprintf("Stopping workspace: %s", id), "Workspace stopped")
    if err := r.stopInstance(ctx, id); err != nil {
        stepLogger.Fail(err)
        return err
    }

    return nil
}

// Remove removes a runtime instance
func (r *<runtime-name>Runtime) Remove(ctx context.Context, id string) error {
    stepLogger := steplogger.FromContext(ctx)
    defer stepLogger.Complete()

    stepLogger.Start("Checking workspace state", "Workspace state checked")
    state, err := r.checkState(ctx, id)
    if err != nil {
        stepLogger.Fail(err)
        return err
    }

    if state == "running" {
        err := fmt.Errorf("workspace is still running, stop it first")
        stepLogger.Fail(err)
        return err
    }

    stepLogger.Start(fmt.Sprintf("Removing workspace: %s", id), "Workspace removed")
    if err := r.removeInstance(ctx, id); err != nil {
        stepLogger.Fail(err)
        return err
    }

    return nil
}

// Info retrieves information about a runtime instance
func (r *<runtime-name>Runtime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
    // Implementation: get workspace info
    
    // Map platform-specific state to valid WorkspaceState
    platformState := "running" // Get actual state from platform
    state := r.mapState(platformState)
    
    return runtime.RuntimeInfo{
        ID:    id,
        State: state,
        Info:  info,
    }, nil
}

// mapState maps platform-specific states to valid WorkspaceState values
func (r *<runtime-name>Runtime) mapState(platformState string) api.WorkspaceState {
    // Example mapping - adjust for your platform
    switch platformState {
    case "running", "active":
        return api.WorkspaceStateRunning
    case "created", "exited", "stopped", "paused":
        return api.WorkspaceStateStopped
    case "failed", "dead":
        return api.WorkspaceStateError
    default:
        return api.WorkspaceStateUnknown
    }
}
```

### 3. Register the Runtime

Edit `pkg/runtimesetup/register.go`:

1. Add import:
```go
import (
    "github.com/kortex-hub/kortex-cli/pkg/runtime"
    "github.com/kortex-hub/kortex-cli/pkg/runtime/fake"
    "github.com/kortex-hub/kortex-cli/pkg/runtime/<runtime-name>"  // Add this
)
```

2. Add to `availableRuntimes` slice:
```go
var availableRuntimes = []runtimeFactory{
    fake.New,
    <runtime-name>.New,  // Add this
}
```

### 3.5. StepLogger Integration

**IMPORTANT**: All runtime methods that accept `context.Context` MUST use StepLogger for user feedback.

**Required imports:**
```go
import (
    "github.com/kortex-hub/kortex-cli/pkg/steplogger"
)
```

**Pattern:**
1. Retrieve logger from context: `stepLogger := steplogger.FromContext(ctx)`
2. Defer completion: `defer stepLogger.Complete()`
3. Start each step: `stepLogger.Start("In progress message", "Completion message")`
4. Fail on errors: `stepLogger.Fail(err)` before returning the error

**Benefits:**
- Users see progress during long-running operations
- Clear feedback on which step failed
- Automatic silence in JSON mode
- No changes needed for JSON vs text output

**See AGENTS.md** for complete StepLogger documentation and best practices.

### 3.6. Logger Integration (for CLI Command Execution)

If your runtime executes external CLI commands (e.g., via `exec.Command`), use `pkg/logger` to route stdout/stderr to the user when `--show-logs` is passed.

**Required imports:**
```go
import (
    "github.com/kortex-hub/kortex-cli/pkg/logger"
)
```

**Pattern — retrieve from context and pass to exec:**
```go
func (r *<runtime-name>Runtime) runSomething(ctx context.Context, args ...string) error {
    l := logger.FromContext(ctx)
    cmd := exec.CommandContext(ctx, "<tool>", args...)
    cmd.Stdout = l.Stdout()
    cmd.Stderr = l.Stderr()
    return cmd.Run()
}
```

**Variable naming convention:**
- Use `stepLogger` for `steplogger.StepLogger`
- Use `l` for `logger.Logger`

**When `--show-logs` is not set**, `logger.FromContext` returns a no-op logger that discards all output, so the pattern is safe to use unconditionally.

**See AGENTS.md** for the `--show-logs` flag pattern and complete Logger documentation.

### 4. Add Tests

Create `pkg/runtime/<runtime-name>/<runtime-name>_test.go`:

```go
package <runtime-name>

import (
    "context"
    "testing"
    "github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

func TestNew(t *testing.T) {
    t.Parallel()

    rt := New()
    if rt == nil {
        t.Fatal("New() returned nil")
    }

    if rt.Type() != "<runtime-name>" {
        t.Errorf("Expected type '<runtime-name>', got %s", rt.Type())
    }
}

func TestCreate(t *testing.T) {
    t.Parallel()

    // Test basic functionality
    rt := New()
    _, err := rt.Create(context.Background(), params)
    if err != nil {
        t.Fatalf("Create() failed: %v", err)
    }
}

func TestCreate_StepLogger(t *testing.T) {
    t.Parallel()

    // Create fake step logger to track calls
    fakeLogger := &fakeStepLogger{}
    ctx := steplogger.WithLogger(context.Background(), fakeLogger)

    rt := New()
    _, err := rt.Create(ctx, params)
    if err != nil {
        t.Fatalf("Create() failed: %v", err)
    }

    // Verify step logger was used correctly
    if len(fakeLogger.startCalls) == 0 {
        t.Error("Expected Start() to be called")
    }
    if fakeLogger.completeCalls != 1 {
        t.Errorf("Expected Complete() to be called once, got %d", fakeLogger.completeCalls)
    }
}

// Fake step logger for testing
type fakeStepLogger struct {
    startCalls    []stepCall
    failCalls     []error
    completeCalls int
}

type stepCall struct {
    inProgress string
    completed  string
}

func (f *fakeStepLogger) Start(inProgress, completed string) {
    f.startCalls = append(f.startCalls, stepCall{inProgress, completed})
}

func (f *fakeStepLogger) Fail(err error) {
    f.failCalls = append(f.failCalls, err)
}

func (f *fakeStepLogger) Complete() {
    f.completeCalls++
}

// Add tests for other methods (Start, Stop, Remove)...
// Each should include both functional and StepLogger tests
```

**Reference:** See `pkg/runtime/podman/steplogger_test.go` and related test files for complete examples.

### 5. Update Copyright Headers

Run the copyright headers skill:
```bash
/copyright-headers
```

### 6. Test the Runtime

```bash
# Run tests
make test

# Build
make build

# Test with CLI (if runtime is available on your system)
./kortex-cli init --runtime <runtime-name>
```

## Required Interfaces

### Runtime Interface (required)

All runtimes MUST implement:

```go
type Runtime interface {
    Type() string
    Create(ctx context.Context, params CreateParams) (RuntimeInfo, error)
    Start(ctx context.Context, id string) (RuntimeInfo, error)
    Stop(ctx context.Context, id string) error
    Remove(ctx context.Context, id string) error
    Info(ctx context.Context, id string) (RuntimeInfo, error)
}
```

### StorageAware Interface (optional)

Implement if the runtime needs persistent storage:

```go
type StorageAware interface {
    Initialize(storageDir string) error
}
```

When implemented, the registry will:
1. Create a directory at `REGISTRY_STORAGE/<runtime-type>`
2. Call `Initialize()` with the path
3. The runtime can use this directory to persist data

### Available Interface (optional)

Implement to control runtime availability:

```go
type Available interface {
    Available() bool
}
```

Use this to:
- Check if required CLI tools are installed
- Check OS compatibility
- Check configuration prerequisites
- Check license/permission requirements

## Reference Implementation

See `pkg/runtime/fake/` for a complete reference implementation that demonstrates:
- All required Runtime interface methods
- StorageAware implementation for persistence
- Proper error handling and state management
- Comprehensive tests

## Common Patterns

### State Validation

All runtimes MUST return valid WorkspaceState values in `RuntimeInfo.State`. Valid states are:
- `running` - The instance is actively running
- `stopped` - The instance is created but not running  
- `error` - The instance encountered an error
- `unknown` - The instance state cannot be determined

**Validation is enforced at the manager boundary (fail-fast approach):**

The instances manager validates all `RuntimeInfo` returned from runtime methods. If a runtime returns an invalid state, the manager immediately returns an error identifying which runtime failed. This means:

✅ **You don't need to call `runtime.ValidateState()` in your runtime implementation**  
✅ **Invalid states are caught automatically during development**  
✅ **Clear error messages identify the problematic runtime**

**Required: Map platform-specific states to valid WorkspaceState values**

Your runtime must map platform-specific states to the four valid states:

```go
// Create() - return "stopped" for newly created instances
func (r *myRuntime) Create(ctx context.Context, params runtime.CreateParams) (runtime.RuntimeInfo, error) {
    // ... create instance logic ...
    
    return runtime.RuntimeInfo{
        ID:    id,
        State: api.WorkspaceStateStopped,  // New instances are "stopped", not "created"
        Info:  info,
    }, nil
}

// Start() - return "running" after starting
func (r *myRuntime) Start(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
    // ... start instance logic ...
    
    return runtime.RuntimeInfo{
        ID:    id,
        State: api.WorkspaceStateRunning,  // Instance is now running
        Info:  info,
    }, nil
}

// Info() - map platform state to valid WorkspaceState
func (r *myRuntime) Info(ctx context.Context, id string) (runtime.RuntimeInfo, error) {
    platformState := r.getPlatformState(id) // e.g., "created", "exited", "paused"
    
    // Map platform state to valid WorkspaceState
    state := r.mapState(platformState)
    
    return runtime.RuntimeInfo{
        ID:    id,
        State: state,
        Info:  info,
    }, nil
}

// mapState maps platform-specific states to valid WorkspaceState values
func (r *myRuntime) mapState(platformState string) api.WorkspaceState {
    switch platformState {
    case "running", "active":
        return api.WorkspaceStateRunning
    case "created", "exited", "stopped", "paused":
        return api.WorkspaceStateStopped
    case "failed", "dead":
        return api.WorkspaceStateError
    default:
        return api.WorkspaceStateUnknown
    }
}
```

**Important notes:**
- Newly created instances should return state `"stopped"`, not platform-specific values like `"created"`
- Platform-specific states must be mapped to the four valid states in your runtime
- The manager validates all states at the boundary - you don't need to validate yourself
- If you return an invalid state, you'll get a clear error during development:
  ```text
  runtime "my-runtime" returned invalid state: invalid runtime state: "created" 
  (must be one of: running, stopped, error, unknown)
  ```
- This fail-fast approach catches bugs early without requiring validation in every runtime

### Error Handling

Use the predefined errors from `pkg/runtime`:

```go
import "github.com/kortex-hub/kortex-cli/pkg/runtime"

// Instance not found
return runtime.RuntimeInfo{}, fmt.Errorf("%w: %s", runtime.ErrInstanceNotFound, id)

// Invalid parameters
return runtime.RuntimeInfo{}, fmt.Errorf("%w: name is required", runtime.ErrInvalidParams)
```

### Persistence

If using StorageAware:

```go
func (r *myRuntime) Initialize(storageDir string) error {
    r.storageDir = storageDir
    r.storageFile = filepath.Join(storageDir, "instances.json")

    // Load existing state
    return r.loadFromDisk()
}

func (r *myRuntime) Create(...) {
    // ... create instance

    // Save to disk
    if err := r.saveToDisk(); err != nil {
        return runtime.RuntimeInfo{}, fmt.Errorf("failed to persist instance: %w", err)
    }
}
```

## Usage Example

After implementing a Podman runtime:

```bash
# Initialize workspace with Podman runtime
./kortex-cli init --runtime podman

# Start workspace
./kortex-cli workspace start <workspace-id>

# Stop workspace
./kortex-cli workspace stop <workspace-id>

# Remove workspace
./kortex-cli workspace remove <workspace-id>
```

## Notes

- Runtime names should be lowercase (e.g., `podman`, `microvm`, `k8s`)
- Use the `fake` runtime as a reference implementation
- All runtimes are registered automatically via `runtimesetup.RegisterAll()`
- Commands don't need to be modified when adding new runtimes
- Only available runtimes (those with `Available() == true`) will be registered
