---
name: working-with-config-system
description: Guide to workspace configuration for environment variables, mount points, and skills at multiple levels
argument-hint: ""
---

# Working with the Config System

The config system manages **workspace configuration** for injecting environment variables and mounting directories into workspaces. This is different from runtime-specific configuration (e.g., Podman image settings).

**What this config system controls:**
- Environment variables to inject into workspace containers/VMs
- Additional directories to mount, with explicit host and container paths
- Skills directories to provide to agents inside the workspace

**What this does NOT control:**
- Runtime-specific settings (e.g., Podman container image, packages to install)
- See `/working-with-podman-runtime-config` for runtime-specific configuration

## Overview

The multi-level configuration system allows users to customize workspace settings at different levels:
- **Workspace-level config** (`.kaiden/workspace.json`) - Shared project configuration committed to repository
  - Can be configured using the `--workspace-configuration` flag of the `init` command (path to directory containing `workspace.json`)
- **Project-specific config** (`~/.kdn/config/projects.json`) - User's custom config for specific projects
- **Global config** (empty string `""` key in `projects.json`) - Settings applied to all projects
- **Agent-specific config** (`~/.kdn/config/agents.json`) - Per-agent overrides (e.g., Claude, Goose)

These configurations control what gets injected **into** workspaces (environment variables, mounts), not how the workspace runtime is built or configured.

## Agent Default Settings Files

In addition to the env/mount configuration above, kdn supports **default settings files** that are baked directly into the workspace image at `init` time.

**Location:** `~/.kdn/config/<agent>/` (one directory per agent name)

Any file placed in this directory is copied into the agent user's home directory (`/home/agent/`) inside the container image, preserving the directory structure. For example:

```text
~/.kdn/config/claude/
└── .claude.json          → /home/agent/.claude.json inside the image
```

This is distinct from `agents.json`:
- `agents.json` — injects **environment variables and mounts** at runtime
- `config/<agent>/` — embeds **dotfiles / settings files** directly into the image at build time

**Automatic Onboarding Configuration:**

For supported agents (e.g., Claude), kdn automatically modifies settings files to skip onboarding prompts:

1. Files are read from `config/<agent>/` (e.g., `config/claude/.claude.json`)
2. If the agent is registered, its `SkipOnboarding()` method is called
3. The agent automatically adds necessary flags (e.g., `hasCompletedOnboarding`, `hasTrustDialogAccepted`)
4. Modified settings are embedded into the container image

This means you can optionally customize agent preferences (theme, etc.) in the settings files, and kdn will automatically add the onboarding flags.

**Model Configuration:**

When the `--model` flag is provided during `init`, kdn does two things with the model:

1. **Persists it in instance data**: The model ID is stored in `InstanceData.Model` and saved to `instances.json`. It can be retrieved via `instance.GetModel()` and is shown in `init` verbose output and `kdn list` (`AGENT/MODEL` column and JSON `model` field).

2. **Writes it to agent settings**: Calls the agent's `SetModel()` method to configure the model in the agent-specific settings file baked into the container image:
   - Claude: `model` field in `.claude/settings.json`
   - Goose: `GOOSE_MODEL` field in `.config/goose/config.yaml`
   - Cursor: `model` object in `.cursor/cli-config.json`

The `--model` flag takes precedence over any model already defined in the settings files. If no model is specified, `GetModel()` returns an empty string and the `model` field is omitted from JSON output.

**Implementation:** `manager.readAgentSettings(storageDir, agentName)` in `pkg/instances/manager.go` walks this directory and returns a `map[string][]byte` (relative forward-slash path → content). If the agent is registered in the agent registry, the manager calls the agent's `SkipOnboarding()` method to modify the settings. If a model ID is provided, the manager then calls the agent's `SetModel()` method to configure the model in the appropriate settings file. The final map is passed to the runtime via `runtime.CreateParams.AgentSettings`. The Podman runtime writes the files into the build context and adds a `COPY --chown=agent:agent agent-settings/. /home/agent/` instruction to the Containerfile. The model is also stored directly in the `instance` struct and persisted in `instances.json` via `InstanceData.Model`.

## Key Components

- **Config Interface** (`pkg/config/config.go`): Interface for managing configuration directories
- **ConfigMerger** (`pkg/config/merger.go`): Merges multiple `WorkspaceConfiguration` objects
- **AgentConfigLoader** (`pkg/config/agents.go`): Loads agent-specific configuration
- **ProjectConfigLoader** (`pkg/config/projects.go`): Loads project and global configuration
- **Manager Integration** (`pkg/instances/manager.go`): Handles config loading and merging during instance creation
- **WorkspaceConfiguration Model**: Imported from `github.com/openkaiden/kdn-api/workspace-configuration/go`

## Configuration File Locations

All user-specific configuration files are stored under the storage directory (default: `~/.kdn`, configurable via `--storage` flag or `KDN_STORAGE` environment variable):

- **Agent configs**: `<storage-dir>/config/agents.json`
- **Project configs**: `<storage-dir>/config/projects.json`
- **Workspace configs**: `.kaiden/workspace.json` (in workspace directory)
  - Created/configured via `kdn init --workspace-configuration <directory-path>`

## Configuration Precedence

Configurations are merged from lowest to highest priority (highest wins):
1. **Agent-specific configuration** (from `agents.json`) - HIGHEST PRIORITY
2. **Project-specific configuration** (from `projects.json` using project ID)
3. **Global project configuration** (from `projects.json` using empty string `""` key)
4. **Workspace-level configuration** (from `.kaiden/workspace.json`) - LOWEST PRIORITY

## Configuration Structure

### Workspace Configuration (`workspace.json`)

The `workspace.json` file controls what gets injected into the workspace:

```json
{
  "environment": [
    {
      "name": "DEBUG",
      "value": "true"
    },
    {
      "name": "API_KEY",
      "secret": "github-token"
    }
  ],
  "mounts": [
    {"host": "$SOURCES/../main", "target": "$SOURCES/../main"},
    {"host": "$HOME/.ssh", "target": "$HOME/.ssh", "ro": true},
    {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true},
    {"host": "/absolute/path/to/data", "target": "/workspace/data", "ro": true}
  ],
  "skills": [
    "/absolute/path/to/commit-skill",
    "$HOME/review-skill"
  ]
}
```

**Creating workspace configuration:**

Use the `--workspace-configuration` flag with the `init` command to specify a directory containing `workspace.json`:

```bash
# Create workspace with custom configuration directory
kdn init /path/to/workspace --workspace-configuration /path/to/config-dir
# This will look for /path/to/config-dir/workspace.json
```

**Fields:**
- `environment` - Environment variables to set in the workspace (optional)
  - `name` - Variable name (must be valid Unix environment variable name)
  - `value` - Hardcoded value (mutually exclusive with `secret`, empty strings allowed)
  - `secret` - Secret reference (mutually exclusive with `value`, cannot be empty)
- `mounts` - List of directories to mount into the workspace (optional)
  - Each entry is an object with `host`, `target`, and optional `ro` fields
  - `host` - Path on the host filesystem (absolute path, or starts with `$SOURCES` or `$HOME`)
  - `target` - Path inside the container (absolute path, or starts with `$SOURCES` or `$HOME`)
  - `ro` - If `true`, mount is read-only (optional, defaults to read-write)
  - `$SOURCES` expands to the workspace sources directory on host, `/workspace/sources` in container
  - `$HOME` expands to the user's home directory on host, `/home/<container-user>` in container
- `skills` - List of skill directories to provide to the agent (optional)
  - Each entry is a path to a single skill directory on the host (containing a `SKILL.md` and related files)
  - Paths must be absolute or start with `$HOME` (`$SOURCES` is not supported)
  - Each directory is mounted read-only into the agent's skills directory using the directory's basename as the skill name
  - The target path is agent-specific (e.g., `$HOME/.claude/skills/<basename>/` for Claude Code)

### Agent Configuration (`agents.json`)

Agent-specific overrides for environment variables and mounts:

```json
{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      }
    ],
    "mounts": [
      {"host": "$HOME/.claude-config", "target": "$HOME/.claude-config", "ro": true}
    ]
  },
  "goose": {
    "environment": [
      {
        "name": "GOOSE_MODE",
        "value": "verbose"
      }
    ]
  }
}
```

### Project Configuration (`projects.json`)

Project-specific and global settings for environment variables and mounts:

```json
{
  "": {
    "mounts": [
      {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true},
      {"host": "$HOME/.ssh", "target": "$HOME/.ssh", "ro": true}
    ]
  },
  "https://github.com/openkaiden/kdn/": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ],
    "mounts": [
      {"host": "$SOURCES/../kaiden-common", "target": "$SOURCES/../kaiden-common"}
    ]
  },
  "/home/user/my/project": {
    "environment": [
      {
        "name": "LOCAL_DEV",
        "value": "true"
      }
    ]
  }
}
```

**Special Keys:**
- Empty string `""` represents global/default configuration applied to all projects
- Useful for common settings like SSH keys, Git config that should be mounted in all workspaces
- Project-specific configs override global config

## Using the Config Interface

### Loading Workspace Configuration

```go
import (
    "github.com/openkaiden/kdn/pkg/config"
    workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
)

// Create a config manager for a workspace
cfg, err := config.NewConfig("/path/to/workspace/.kaiden")
if err != nil {
    return err
}

// Load and validate the workspace configuration
workspaceCfg, err := cfg.Load()
if err != nil {
    if errors.Is(err, config.ErrConfigNotFound) {
        // workspace.json doesn't exist, use defaults
    } else if errors.Is(err, config.ErrInvalidConfig) {
        // Configuration validation failed
    } else {
        return err
    }
}

// Access configuration values (note: fields are pointers)
if workspaceCfg.Environment != nil {
    for _, env := range *workspaceCfg.Environment {
        // Use env.Name, env.Value, env.Secret
    }
}

if workspaceCfg.Mounts != nil {
    for _, m := range *workspaceCfg.Mounts {
        // Use m.Host, m.Target, m.Ro
    }
}

if workspaceCfg.Skills != nil {
    for _, s := range *workspaceCfg.Skills {
        // Use s (host path to skill directory)
    }
}
```

## Using the Multi-Level Config System

The Manager handles all configuration loading and merging automatically:

```go
// In command code (e.g., init command)
addedInstance, err := manager.Add(ctx, instances.AddOptions{
    Instance:        instance,
    RuntimeType:     "fake",
    WorkspaceConfig: workspaceConfig,  // From .kaiden/workspace.json or --workspace-configuration directory
    Project:         "custom-project",  // Optional override
    Agent:           "claude",          // Optional agent name
    Model:           "claude-sonnet-4", // Optional model ID (takes precedence over settings)
})
```

The Manager's `Add()` method:
1. Detects project ID (or uses custom override)
2. Loads project config (global `""` + project-specific merged)
3. Loads agent config (if agent name provided)
4. Merges configs: workspace → global → project → agent
5. Calls agent's `SkipOnboarding()` if agent is registered
6. Calls agent's `SetModel()` if model is specified (takes precedence over settings)
7. Converts `Skills` entries into `Mounts` using the agent's `SkillsDir()` (agent-specific target path)
8. Passes merged config to runtime for injection into workspace

## Merging Behavior

- **Environment variables**: Later configs override earlier ones by name
  - If the same variable appears in multiple configs, the one from the higher-precedence config wins
- **Mounts**: Deduplicated by `host`+`target` pair (preserves order, removes duplicates)
- **Skills**: Deduplicated by path value (preserves order, base skills first then override)

**Example Merge Flow:**

Given:
- Workspace config: `DEBUG=workspace`, `WORKSPACE_VAR=value1`
- Global config: `GLOBAL_VAR=global`
- Project config: `DEBUG=project`, `PROJECT_VAR=value2`
- Agent config: `DEBUG=agent`, `AGENT_VAR=value3`

Result: `DEBUG=agent`, `WORKSPACE_VAR=value1`, `GLOBAL_VAR=global`, `PROJECT_VAR=value2`, `AGENT_VAR=value3`

## Loading Configuration Programmatically

```go
import "github.com/openkaiden/kdn/pkg/config"

// Load project config (includes global + project-specific merged)
projectLoader, err := config.NewProjectConfigLoader(storageDir)
projectConfig, err := projectLoader.Load("github.com/user/repo")

// Load agent config
agentLoader, err := config.NewAgentConfigLoader(storageDir)
agentConfig, err := agentLoader.Load("claude")

// Merge configurations
merger := config.NewMerger()
merged := merger.Merge(workspaceConfig, projectConfig)
merged = merger.Merge(merged, agentConfig)
```

## Configuration Validation

The `Load()` method automatically validates the configuration and returns `ErrInvalidConfig` if any of these rules are violated:

### Environment Variables

- Name cannot be empty
- Name must be a valid Unix environment variable name (starts with letter or underscore, followed by letters, digits, or underscores)
- Exactly one of `value` or `secret` must be defined
- Secret references cannot be empty strings
- Empty values are allowed (valid use case: set env var to empty string)

### Mount Paths

- Each mount must have a non-empty `host` and `target`
- Both `host` and `target` must be absolute paths OR start with `$SOURCES` or `$HOME`
- Relative paths (e.g., `../foo`) are not allowed
- `$SOURCES`-based container targets must not escape above `/workspace`
- `$HOME`-based container targets must not escape above `/home/<container-user>`

### Skills

- Each entry cannot be empty
- Each path must be an absolute path or start with `$HOME` (`$SOURCES` is not supported)
- Duplicate paths (within or across config levels) are deduplicated by the merger

## Error Handling

- `config.ErrInvalidPath` - Configuration path is empty or invalid
- `config.ErrConfigNotFound` - The `workspace.json` file is not found
- `config.ErrInvalidConfig` - Configuration validation failed (includes detailed error message)
- `config.ErrInvalidAgentConfig` - Agent configuration is invalid
- `config.ErrInvalidProjectConfig` - Project configuration is invalid

## Testing Multi-Level Configs

```go
// Create test config files
configDir := filepath.Join(storageDir, "config")
os.MkdirAll(configDir, 0755)

agentsJSON := `{"claude": {"environment": [{"name": "VAR", "value": "val"}]}}`
os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(agentsJSON), 0644)

// Run init with agent
rootCmd.SetArgs([]string{"init", sourcesDir, "--runtime", "fake", "--agent", "claude"})
rootCmd.Execute()
```

## Design Principles

- Configuration directory is NOT automatically created
- Missing configuration directory is treated as empty/default configuration
- All configurations are validated on load to catch errors early
- Configuration merging is handled by Manager, not commands
- Missing config files return empty configs (not errors)
- Invalid JSON or validation errors are reported
- All loaders follow the module design pattern
- Cross-platform compatible (uses `filepath.Join()`, `t.TempDir()`)
- Storage directory is configurable via `--storage` flag or `KDN_STORAGE` env var
- Uses nested JSON structure for clarity and extensibility
- Model types are imported from external API package for consistency

## Related Skills

- `/working-with-podman-runtime-config` - Configure runtime-specific settings (Podman image, packages, etc.)
- `/working-with-instances-manager` - Using the instances manager API

## References

- **Config Interface**: `pkg/config/config.go`
- **ConfigMerger**: `pkg/config/merger.go`
- **AgentConfigLoader**: `pkg/config/agents.go`
- **ProjectConfigLoader**: `pkg/config/projects.go`
- **Manager Integration**: `pkg/instances/manager.go`
