---
name: working-with-instances-manager
description: Guide to using the instances manager API for workspace management and project detection
argument-hint: ""
---

# Working with the Instances Manager

The instances manager provides the API for managing workspace instances throughout their lifecycle. This skill covers the manager API and project detection functionality.

## Overview

The instances manager handles:
- Adding and removing workspace instances
- Listing and retrieving instance information
- Starting and stopping instances via runtimes
- Project detection and grouping
- Configuration merging (workspace, project, agent configs)
- Agent settings files and automatic onboarding configuration
- Interactive terminal sessions with running instances

## Creating the Manager

In command `preRun`, create the manager from the storage flag:

```go
storageDir, _ := cmd.Flags().GetString("storage")
manager, err := instances.NewManager(storageDir)
if err != nil {
    return fmt.Errorf("failed to create manager: %w", err)
}
```

## Manager API

### Add - Create New Instance

Add a new workspace instance to the manager:

```go
instance, err := instances.NewInstance(instances.NewInstanceParams{
    SourceDir: sourceDir,
    ConfigDir: configDir,
    Name:      name, // Optional: user-provided name, sanitized by manager
})
if err != nil {
    return fmt.Errorf("failed to create instance: %w", err)
}

addedInstance, err := manager.Add(ctx, instances.AddOptions{
    Instance:        instance,
    RuntimeType:     "fake",
    WorkspaceConfig: workspaceConfig,  // From .kaiden/workspace.json
    Project:         "custom-project",  // Optional: overrides auto-detection
    Agent:           "claude",          // Optional: agent name for agent-specific config
    Model:           "claude-sonnet-4", // Optional: model ID for agent (takes precedence over settings)
})
if err != nil {
    return fmt.Errorf("failed to add instance: %w", err)
}
```

The `Add()` method:
1. Sanitizes and generates a unique workspace name:
   - If `Name` is empty, it is derived from the source directory basename
   - The name is sanitized: lowercased, spaces and invalid characters replaced with hyphens
   - A numeric suffix (`-2`, `-3`, …) is appended if the name is already in use
2. Detects project ID (or uses custom override)
3. Loads project config (global `""` + project-specific merged)
4. Loads agent config (if agent name provided)
5. Merges configs: workspace → global → project → agent
6. Reads agent settings files from `<storage-dir>/config/<agent>/` into `map[string][]byte`
7. Calls agent's `SkipOnboarding()` method if agent is registered (e.g., Claude agent automatically sets onboarding flags)
8. Calls agent's `SetModel()` method if model is specified (takes precedence over model in settings files)
9. Calls agent's `SetMCPServers()` method if the merged config contains MCP servers (writes them into agent settings)
10. Passes merged config and modified agent settings to runtime for injection into workspace

**Name sanitization rules:** valid characters are `[a-z0-9._-]`. Uppercase letters are lowercased; any run of invalid characters (spaces, `@`, `+`, etc.) is collapsed into a single hyphen; leading and trailing hyphens are stripped. An empty result falls back to `"workspace"`.

### List - Get All Instances

List all registered workspace instances:

```go
instancesList, err := manager.List()
if err != nil {
    return fmt.Errorf("failed to list instances: %w", err)
}

for _, instance := range instancesList {
    fmt.Printf("ID: %s, State: %s, Project: %s\n",
        instance.ID, instance.State, instance.Project)
}
```

### Get - Retrieve Specific Instance

Get a specific instance by name or ID:

```go
instance, err := manager.Get(nameOrID)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", nameOrID)
    }
    return fmt.Errorf("instance not found: %w", err)
}

fmt.Printf("Found instance: %s (State: %s)\n", instance.ID, instance.State)
```

**Key Points:**
- The `Get()` method accepts either a workspace name or ID
- It first tries to match by ID, then falls back to matching by name
- This allows commands to accept user-friendly names while still supporting IDs
- The method always returns the instance with its ID, regardless of how it was looked up

### Delete - Remove Instance

Delete an instance from the manager (requires ID):

```go
err := manager.Delete(ctx, id)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", id)
    }
    return fmt.Errorf("failed to delete instance: %w", err)
}
```

**For commands accepting name or ID:**

Commands should resolve the name or ID to an instance first, then use the ID:

```go
// Resolve name or ID to get the instance
instance, err := manager.Get(nameOrID)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", nameOrID)
    }
    return err
}

// Use the resolved ID
err = manager.Delete(ctx, instance.GetID())
if err != nil {
    return fmt.Errorf("failed to delete instance: %w", err)
}
```

### Start - Start Instance Runtime

Start a stopped instance (requires ID):

```go
err := manager.Start(ctx, id)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", id)
    }
    return fmt.Errorf("failed to start instance: %w", err)
}
```

**For commands accepting name or ID:**

Commands should resolve the name or ID to an instance first, then use the ID:

```go
// Resolve name or ID to get the instance
instance, err := manager.Get(nameOrID)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", nameOrID)
    }
    return err
}

// Use the resolved ID
instanceID := instance.GetID()
err = manager.Start(ctx, instanceID)
if err != nil {
    return fmt.Errorf("failed to start instance: %w", err)
}

// Output the ID (not the name)
fmt.Fprintln(cmd.OutOrStdout(), instanceID)
```

### Stop - Stop Instance Runtime

Stop a running instance (requires ID):

```go
err := manager.Stop(ctx, id)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", id)
    }
    return fmt.Errorf("failed to stop instance: %w", err)
}
```

**For commands accepting name or ID:**

Commands should resolve the name or ID to an instance first, then use the ID:

```go
// Resolve name or ID to get the instance
instance, err := manager.Get(nameOrID)
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s", nameOrID)
    }
    return err
}

// Use the resolved ID
instanceID := instance.GetID()
err = manager.Stop(ctx, instanceID)
if err != nil {
    return fmt.Errorf("failed to stop instance: %w", err)
}

// Output the ID (not the name)
fmt.Fprintln(cmd.OutOrStdout(), instanceID)
```

### Terminal - Interactive Terminal Session

Connect to a running instance with an interactive terminal (requires ID):

```go
err := manager.Terminal(cmd.Context(), id, []string{"bash"})
if err != nil {
    if errors.Is(err, instances.ErrInstanceNotFound) {
        return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", id)
    }
    return err
}
```

**Terminal Method Behavior:**
- Verifies the instance exists and is in a running state
- Checks if the runtime implements the `runtime.Terminal` interface
- Delegates to the runtime's Terminal implementation
- Returns an error if the instance is not running or runtime doesn't support terminals

**Key Points:**
- Uses a read lock (doesn't modify instance state)
- Command is a slice of strings: `[]string{"bash"}` or `[]string{"claude-code", "--debug"}`
- Returns `ErrInstanceNotFound` if instance doesn't exist
- Returns an error if instance state is not "running"
- Returns an error if the runtime doesn't implement `runtime.Terminal` interface

**For commands accepting name or ID:**

Commands should resolve the name or ID to an instance first, then use the ID:

```go
func (w *workspaceTerminalCmd) run(cmd *cobra.Command, args []string) error {
    // Resolve name or ID to get the instance
    instance, err := w.manager.Get(w.nameOrID)
    if err != nil {
        if errors.Is(err, instances.ErrInstanceNotFound) {
            return fmt.Errorf("workspace not found: %s\nUse 'workspace list' to see available workspaces", w.nameOrID)
        }
        return err
    }

    // Get the actual ID (in case user provided a name)
    instanceID := instance.GetID()

    // Start terminal session
    err = w.manager.Terminal(cmd.Context(), instanceID, w.command)
    if err != nil {
        return err
    }
    return nil
}
```

## Agent Registry and Automatic Onboarding

The manager integrates with the agent registry to provide automatic onboarding configuration for supported agents.

### Registering Agents

Register all available agents using the centralized registration:

```go
import "github.com/openkaiden/kdn/pkg/agentsetup"

// In preRun or initialization code
if err := agentsetup.RegisterAll(manager); err != nil {
    return fmt.Errorf("failed to register agents: %w", err)
}
```

This registers all available agents (e.g., Claude) with the manager's agent registry.

### Automatic Agent Onboarding

When adding an instance with an agent name, the manager automatically:

1. **Reads agent settings files** from `<storage-dir>/config/<agent>/` (e.g., `config/claude/.claude.json`)
2. **Looks up the agent** in the registry by name
3. **Calls `SkipOnboarding()`** if the agent is registered, passing:
   - Current agent settings map (file paths → content)
   - Workspace sources path from the runtime (e.g., `/workspace/sources`)
4. **Receives modified settings** with onboarding flags automatically set

**Example - Claude Agent:**

For the Claude agent, `SkipOnboarding()` automatically:
- Sets `hasCompletedOnboarding: true` to skip the first-run wizard
- Adds `hasTrustDialogAccepted: true` for the workspace sources directory
- Preserves any custom settings you've configured (theme, preferences, etc.)

`SetMCPServers()` writes MCP server entries into `.claude.json` at the top-level `mcpServers` key (user scope). Command-based servers use `type: "stdio"`, URL-based servers use `type: "sse"`. Existing MCP server entries in the file are preserved.

**Graceful Fallback:**

If an agent name is provided but not registered:
- Settings files are read as-is
- No `SkipOnboarding()` modification occurs
- Instance creation succeeds normally

This allows forward compatibility with new agents before they implement the onboarding interface.

### Testing with Agent Registry

```go
// Create manager with agent registry
manager, _ := instances.NewManager(storageDir)

// Register a specific agent for testing
claudeAgent := agent.NewClaude()
if err := manager.RegisterAgent("claude", claudeAgent); err != nil {
    t.Fatalf("Failed to register agent: %v", err)
}

// Add instance - Claude's SkipOnboarding will be called automatically
instance, err := manager.Add(ctx, instances.AddOptions{
    Instance:    inst,
    RuntimeType: "fake",
    Agent:       "claude",
})
```

See `pkg/instances/manager_test.go` (TestManager_Add_AppliesAgentOnboarding) for comprehensive test examples.

## Project Detection and Grouping

Each workspace has a `project` field that enables grouping workspaces belonging to the same project across branches, forks, or subdirectories.

### Project Identifier Detection

The manager automatically detects the project identifier when adding instances:

1. **Git repository with remote**: Uses repository remote URL (without `.git`) plus relative path
   - Checks `upstream` remote first (useful for forks)
   - Falls back to `origin` remote if `upstream` doesn't exist
   - Example: `https://github.com/openkaiden/kdn/` (at root) or `https://github.com/openkaiden/kdn/pkg/git` (in subdirectory)

2. **Git repository without remote**: Uses repository root directory plus relative path
   - Example: `/home/user/local-repo` (at root) or `/home/user/local-repo/pkg/utils` (in subdirectory)

3. **Non-git directory**: Uses the source directory path
   - Example: `/tmp/workspace`

### Custom Project Override

Users can override auto-detection with the `--project` flag:

```go
// Add instance with custom project
addedInstance, err := manager.Add(ctx, instances.AddOptions{
    Instance:        instance,
    RuntimeType:     "fake",
    WorkspaceConfig: workspaceConfig,
    Project:         "custom-project-id", // Optional: overrides auto-detection
})
```

### Implementation Details

- **Package**: `pkg/git` provides git repository detection with testable abstractions
- **Detector Interface**: `git.Detector` with `DetectRepository(ctx, dir)` method
- **Executor Pattern**: `git.Executor` abstracts git command execution for testing
- **Manager Integration**: `manager.detectProject()` is called during `Add()` if no custom project is provided

### Testing with Fake Git Detector

```go
// Use fake git detector in tests
gitDetector := newFakeGitDetectorWithRepo(
    "/repo/root",
    "https://github.com/user/repo",
    "pkg/subdir", // relative path
)

manager, _ := newManagerWithFactory(
    storageDir,
    fakeInstanceFactory,
    newFakeGenerator(),
    newTestRegistry(tmpDir),
    gitDetector,
)
```

See `pkg/instances/manager_project_test.go` for comprehensive test examples.

## Error Handling

Common errors from the manager:

```go
// Instance not found
if errors.Is(err, instances.ErrInstanceNotFound) {
    return fmt.Errorf("workspace not found: %s", id)
}

// Runtime not found
if errors.Is(err, runtime.ErrRuntimeNotFound) {
    return fmt.Errorf("runtime not found: %s", runtimeType)
}

// Instance already exists
if errors.Is(err, instances.ErrInstanceExists) {
    return fmt.Errorf("workspace already exists: %s", id)
}
```

## Example: Complete Command Implementation

```go
type myCmd struct {
    manager instances.Manager
}

func (c *myCmd) preRun(cmd *cobra.Command, args []string) error {
    storageDir, _ := cmd.Flags().GetString("storage")

    manager, err := instances.NewManager(storageDir)
    if err != nil {
        return fmt.Errorf("failed to create manager: %w", err)
    }

    // Register runtimes
    if err := runtimesetup.RegisterAll(manager); err != nil {
        return fmt.Errorf("failed to register runtimes: %w", err)
    }

    // Register agents
    if err := agentsetup.RegisterAll(manager); err != nil {
        return fmt.Errorf("failed to register agents: %w", err)
    }

    c.manager = manager
    return nil
}

func (c *myCmd) run(cmd *cobra.Command, args []string) error {
    // Use manager to list instances
    instances, err := c.manager.List()
    if err != nil {
        return fmt.Errorf("failed to list instances: %w", err)
    }

    for _, instance := range instances {
        cmd.Printf("ID: %s, State: %s, Project: %s\n",
            instance.ID, instance.State, instance.Project)
    }

    return nil
}
```

## Related Skills

- `/working-with-config-system` - Configuration merging and multi-level configs
- `/working-with-runtime-system` - Runtime system architecture
- `/implementing-command-patterns` - Command implementation patterns

## References

- **Manager Interface**: `pkg/instances/manager.go`
- **Git Detection**: `pkg/git/`
- **Project Tests**: `pkg/instances/manager_project_test.go`
- **Example Commands**: `pkg/cmd/init.go`, `pkg/cmd/workspace_list.go`, `pkg/cmd/workspace_terminal.go`
