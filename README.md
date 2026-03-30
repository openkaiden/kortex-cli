# kortex-cli

[![codecov](https://codecov.io/gh/kortex-hub/kortex-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/kortex-hub/kortex-cli)
[![Documentation](https://img.shields.io/badge/documentation-blue)](https://kortex-hub.github.io/kortex-cli/)

kortex-cli is a command-line interface for launching and managing AI agents with custom configurations. It provides a unified way to start different agents with specific settings including skills, MCP (Model Context Protocol) server connections, and LLM integrations.

**Supported Agents**

- **Claude Code** - Anthropic's official CLI for Claude
- **Goose** - AI agent for development tasks
- **Cursor** - AI-powered code editor agent

**Key Features**

- Configure agents with custom skills and capabilities
- Connect to MCP servers for extended functionality
- Integrate with various LLM providers
- Consistent interface across different agent types

## Getting Started

### Prerequisites

- Go 1.26+
- Make

### Build

```bash
make build
```

This creates the `kortex-cli` binary in the current directory.

### Run

```bash
# Display help and available commands
./kortex-cli --help

# Execute a specific command
./kortex-cli <command> [flags]
```

### Install

To install the binary to your `GOPATH/bin` for system-wide access:

```bash
make install
```

### Run Tests

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage
```

## Glossary

### Agent
An AI assistant that can perform tasks autonomously. In kortex-cli, agents are the different AI tools (Claude Code, Goose, Cursor) that can be launched and configured.

### LLM (Large Language Model)
The underlying AI model that powers the agents. Examples include Claude (by Anthropic), GPT (by OpenAI), and other language models.

### MCP (Model Context Protocol)
A standardized protocol for connecting AI agents to external data sources and tools. MCP servers provide agents with additional capabilities like database access, API integrations, or file system operations.

### Skills
Pre-configured capabilities or specialized functions that can be enabled for an agent. Skills extend what an agent can do, such as code review, testing, or specific domain knowledge.

### Workspace
A registered directory containing your project source code and its configuration. Each workspace is tracked by kortex-cli with a unique ID and name for easy management.

## Scenarios

### Managing Workspaces from a UI or Programmatically

This scenario demonstrates how to manage workspaces programmatically using JSON output, which is ideal for UIs, scripts, or automation tools. All commands support the `--output json` (or `-o json`) flag for machine-readable output.

**Step 1: Check existing workspaces**

```bash
$ kortex-cli workspace list -o json
```

```json
{
  "items": []
}
```

Exit code: `0` (success, but no workspaces registered)

**Step 2: Register a new workspace**

```bash
$ kortex-cli init /path/to/project --runtime fake --agent claude -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 3: Register with verbose output to get full details**

```bash
$ kortex-cli init /path/to/another-project --runtime fake --agent claude -o json -v
```

```json
{
  "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
  "name": "another-project",
  "agent": "claude",
  "paths": {
    "source": "/absolute/path/to/another-project",
    "configuration": "/absolute/path/to/another-project/.kortex"
  }
}
```

Exit code: `0` (success)

**Step 4: List all workspaces**

```bash
$ kortex-cli workspace list -o json
```

```json
{
  "items": [
    {
      "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea",
      "name": "project",
      "agent": "claude",
      "paths": {
        "source": "/absolute/path/to/project",
        "configuration": "/absolute/path/to/project/.kortex"
      }
    },
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
      "agent": "claude",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kortex"
      }
    }
  ]
}
```

Exit code: `0` (success)

**Step 5: Start a workspace**

```bash
$ kortex-cli workspace start 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 6: Stop a workspace**

```bash
$ kortex-cli workspace stop 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 7: Remove a workspace**

```bash
$ kortex-cli workspace remove 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 8: Verify removal**

```bash
$ kortex-cli workspace list -o json
```

```json
{
  "items": [
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
      "agent": "claude",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kortex"
      }
    }
  ]
}
```

Exit code: `0` (success)

#### Error Handling

All errors are returned in JSON format when using `--output json`, with the error written to **stdout** (not stderr) and a non-zero exit code.

**Error: Non-existent directory**

```bash
$ kortex-cli init /tmp/no-exist --runtime fake --agent claude -o json
```

```json
{
  "error": "sources directory does not exist: /tmp/no-exist"
}
```

Exit code: `1` (error)

**Error: Workspace not found**

```bash
$ kortex-cli workspace remove unknown-id -o json
```

```json
{
  "error": "workspace not found: unknown-id"
}
```

Exit code: `1` (error)

#### Best Practices for Programmatic Usage

1. **Always check the exit code** to determine success (0) or failure (non-zero)
2. **Parse stdout** for JSON output in both success and error cases
3. **Use verbose mode** with init (`-v`) when you need full workspace details immediately after creation
4. **Handle both success and error JSON structures** in your code:
   - Success responses have specific fields (e.g., `id`, `items`, `name`, `paths`)
   - Error responses always have an `error` field

**Example script pattern:**

```bash
#!/bin/bash

# Register a workspace
output=$(kortex-cli init /path/to/project --runtime fake --agent claude -o json)
exit_code=$?

if [ $exit_code -eq 0 ]; then
    workspace_id=$(echo "$output" | jq -r '.id')
    echo "Workspace created: $workspace_id"
else
    error_msg=$(echo "$output" | jq -r '.error')
    echo "Error: $error_msg"
    exit 1
fi
```

## Environment Variables

kortex-cli supports environment variables for configuring default behavior.

### `KORTEX_CLI_DEFAULT_RUNTIME`

Sets the default runtime to use when registering a workspace with the `init` command.

**Usage:**

```bash
export KORTEX_CLI_DEFAULT_RUNTIME=fake
kortex-cli init /path/to/project --agent claude
```

**Priority:**

The runtime is determined in the following order (highest to lowest priority):

1. `--runtime` flag (if specified)
2. `KORTEX_CLI_DEFAULT_RUNTIME` environment variable (if set)
3. Error if neither is set (runtime is required)

**Example:**

```bash
# Set the default runtime for the current shell session
export KORTEX_CLI_DEFAULT_RUNTIME=fake

# Register a workspace using the environment variable
kortex-cli init /path/to/project --agent claude

# Override the environment variable with the flag
kortex-cli init /path/to/another-project --agent claude --runtime podman
```

**Notes:**

- The runtime parameter is mandatory when registering workspaces
- If neither the flag nor the environment variable is set, the `init` command will fail with an error
- Supported runtime types depend on the available runtime implementations
- Setting this environment variable is useful for automation scripts or when you consistently use the same runtime

### `KORTEX_CLI_DEFAULT_AGENT`

Sets the default agent to use when registering a workspace with the `init` command.

**Usage:**

```bash
export KORTEX_CLI_DEFAULT_AGENT=claude
kortex-cli init /path/to/project --runtime fake
```

**Priority:**

The agent is determined in the following order (highest to lowest priority):

1. `--agent` flag (if specified)
2. `KORTEX_CLI_DEFAULT_AGENT` environment variable (if set)
3. Error if neither is set (agent is required)

**Example:**

```bash
# Set the default agent for the current shell session
export KORTEX_CLI_DEFAULT_AGENT=claude

# Register a workspace using the environment variable
kortex-cli init /path/to/project --runtime fake

# Override the environment variable with the flag
kortex-cli init /path/to/another-project --runtime fake --agent goose
```

**Notes:**

- The agent parameter is mandatory when registering workspaces
- If neither the flag nor the environment variable is set, the `init` command will fail with an error
- Supported agent types depend on the available agent configurations in the runtime
- Agent names must contain only alphanumeric characters or underscores (e.g., `claude`, `goose`, `my_agent`)
- Setting this environment variable is useful for automation scripts or when you consistently use the same agent

### `KORTEX_CLI_STORAGE`

Sets the default storage directory where kortex-cli stores its data files.

**Usage:**

```bash
export KORTEX_CLI_STORAGE=/custom/path/to/storage
kortex-cli init /path/to/project --runtime fake --agent claude
```

**Priority:**

The storage directory is determined in the following order (highest to lowest priority):

1. `--storage` flag (if specified)
2. `KORTEX_CLI_STORAGE` environment variable (if set)
3. Default: `$HOME/.kortex-cli`

**Example:**

```bash
# Set a custom storage directory
export KORTEX_CLI_STORAGE=/var/lib/kortex

# All commands will use this storage directory
kortex-cli init /path/to/project --runtime fake --agent claude
kortex-cli list

# Override the environment variable with the flag
kortex-cli list --storage /tmp/kortex-storage
```

## Podman Runtime

The Podman runtime provides a container-based development environment for workspaces. It creates an isolated environment with all necessary tools pre-installed and configured.

### Container Image

**Base Image:** `registry.fedoraproject.org/fedora:latest`

The Podman runtime builds a custom container image based on Fedora Linux, providing a stable and up-to-date foundation for development work.

### Installed Packages

The runtime includes a comprehensive development toolchain:

- **Core Utilities:**
  - `which` - Command location utility
  - `procps-ng` - Process management utilities
  - `wget2` - Advanced file downloader

- **Development Tools:**
  - `@development-tools` - Complete development toolchain (gcc, make, etc.)
  - `jq` - JSON processor
  - `gh` - GitHub CLI

- **Language Support:**
  - `golang` - Go programming language
  - `golangci-lint` - Go linter
  - `python3` - Python 3 interpreter
  - `python3-pip` - Python package manager

### User and Permissions

The container runs as a non-root user named `agent` with the following configuration:

- **User:** `agent`
- **UID/GID:** Matches the host user's UID and GID for seamless file permissions
- **Home Directory:** `/home/agent`

**Sudo Permissions:**

The `agent` user has limited sudo access with no password required (`NOPASSWD`) for:

- **Package Management:**
  - `/usr/bin/dnf` - Install, update, and manage packages

- **Process Management:**
  - `/bin/nice` - Run programs with modified scheduling priority
  - `/bin/kill`, `/usr/bin/kill` - Send signals to processes
  - `/usr/bin/killall` - Kill processes by name

All other sudo commands are explicitly denied for security.

### AI Agent

**Claude Code** is installed as the default AI agent using the official installation script from `claude.ai/install.sh`. This provides:

- Full Claude Code CLI capabilities
- Integrated development assistance
- Access to Claude's latest features

The agent runs within the container environment and has access to the mounted workspace sources and dependencies.

### Working Directory

The container's working directory is set to `/workspace/sources`, which is where your project source code is mounted. This ensures that the agent and all tools operate within your project context.

### Example Usage

```bash
# Register a workspace with the Podman runtime
kortex-cli init /path/to/project --runtime podman --agent claude
```

**User Experience:**

When you register a workspace with the Podman runtime, you'll see progress feedback for each operation:

```text
⠋ Creating temporary build directory
✓ Temporary build directory created
⠋ Generating Containerfile
✓ Containerfile generated
⠋ Building container image: kortex-cli-myproject
✓ Container image built
⠋ Creating container: myproject
✓ Container created
```

The `init` command will:
1. Create a temporary build directory - **with progress spinner**
2. Generate a Containerfile with the configuration above - **with progress spinner**
3. Build a custom image (tagged as `kortex-cli-<workspace-name>`) - **with progress spinner**
4. Create a container with your source code mounted - **with progress spinner**

After registration, you can start the workspace:

```bash
# Start the workspace
kortex-cli start <workspace-id>
```

**Note:** When using `--output json`, all progress spinners are hidden to avoid polluting the JSON output.

### Customizing Podman Runtime Configuration

The Podman runtime is fully configurable through JSON files. When you first use the Podman runtime, default configuration files are automatically created in your storage directory.

**Configuration Location:**

```text
$HOME/.kortex-cli/runtimes/podman/config/
├── image.json    # Base image configuration
└── claude.json   # Agent-specific configuration
```

Or if using a custom storage directory:

```text
<storage-dir>/runtimes/podman/config/
```

#### Base Image Configuration (`image.json`)

Controls the container's base image, packages, and sudo permissions.

**Structure:**

```json
{
  "version": "latest",
  "packages": [
    "which",
    "procps-ng",
    "wget2",
    "@development-tools",
    "jq",
    "gh",
    "golang",
    "golangci-lint",
    "python3",
    "python3-pip"
  ],
  "sudo": [
    "/usr/bin/dnf",
    "/bin/nice",
    "/bin/kill",
    "/usr/bin/kill",
    "/usr/bin/killall"
  ],
  "run_commands": []
}
```

**Fields:**

- `version` (required) - Fedora version tag
  - Examples: `"latest"`, `"40"`, `"41"`
  - The base registry `registry.fedoraproject.org/fedora` is hardcoded and cannot be changed

- `packages` (optional) - DNF packages to install
  - Array of package names
  - Can include package groups with `@` prefix (e.g., `"@development-tools"`)
  - Empty array is valid if no packages needed

- `sudo` (optional) - Binaries the `agent` user can run with sudo
  - Must be absolute paths (e.g., `"/usr/bin/dnf"`)
  - Creates a single `ALLOWED` command alias in sudoers
  - Empty array disables all sudo access

- `run_commands` (optional) - Custom shell commands to run during image build
  - Executed as RUN instructions in the Containerfile
  - Run before agent-specific commands
  - Useful for additional setup steps

#### Agent Configuration (`claude.json`)

Controls agent-specific packages and installation steps.

**Structure:**

```json
{
  "packages": [],
  "run_commands": [
    "curl -fsSL --proto-redir '-all,https' --tlsv1.3 https://claude.ai/install.sh | bash",
    "mkdir -p /home/agent/.config"
  ],
  "terminal_command": [
    "claude"
  ]
}
```

**Fields:**

- `packages` (optional) - Additional packages specific to this agent
  - Merged with packages from `image.json`
  - Useful for agent-specific dependencies

- `run_commands` (optional) - Commands to set up the agent
  - Executed after image configuration commands
  - Typically used for agent installation

- `terminal_command` (required) - Command to launch the agent
  - Must have at least one element
  - Can include flags: `["claude", "--verbose"]`

#### Applying Configuration Changes

Configuration changes take effect when you **register a new workspace with `init`**. The Containerfile is generated and the image is built during workspace registration, using the configuration files that exist at that time.

**To apply new configuration:**

1. Edit the configuration files:
   ```bash
   # Edit base image configuration
   nano ~/.kortex-cli/runtimes/podman/config/image.json

   # Edit agent configuration
   nano ~/.kortex-cli/runtimes/podman/config/claude.json
   ```

2. Register a new workspace (this creates the Containerfile and builds the image):
   ```bash
   kortex-cli init /path/to/project --runtime podman --agent claude
   ```

3. Start the workspace:
   ```bash
   kortex-cli start <workspace-id>
   ```

**Notes:**

- The first `init` command using Podman creates default config files automatically
- Config files are never overwritten once created - your customizations are preserved
- The Containerfile and image are built during `init`, not `start`
- Each workspace's image is built once using the configuration at registration time
- To rebuild a workspace with new config, remove and re-register it
- Validation errors in config files will cause workspace registration to fail with a descriptive message
- The generated Containerfile is automatically copied to `/home/agent/Containerfile` inside the container for reference

## Workspace Configuration

Each workspace can optionally include a configuration file that customizes the environment and mount behavior for that specific workspace. The configuration is stored in a `workspace.json` file within the workspace's configuration directory (typically `.kortex` in the sources directory).

### Configuration File Location

By default, workspace configuration is stored at:
```text
<sources-directory>/.kortex/workspace.json
```

The configuration directory (containing `workspace.json`) can be customized using the `--workspace-configuration` flag when registering a workspace with `init`. The flag accepts a directory path, not the file path itself.

### Configuration Structure

The `workspace.json` file uses a nested JSON structure:

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
  "mounts": {
    "dependencies": ["../main", "../../lib"],
    "configs": [".ssh", ".gitconfig"]
  }
}
```

### Environment Variables

Define environment variables that will be set in the workspace runtime environment.

**Structure:**
```json
{
  "environment": [
    {
      "name": "VAR_NAME",
      "value": "hardcoded-value"
    },
    {
      "name": "SECRET_VAR",
      "secret": "secret-reference"
    }
  ]
}
```

**Fields:**
- `name` (required) - Environment variable name
  - Must be a valid Unix environment variable name
  - Must start with a letter or underscore
  - Can contain letters, digits, and underscores
- `value` (optional) - Hardcoded value for the variable
  - Mutually exclusive with `secret`
  - Empty strings are allowed
- `secret` (optional) - Reference to a secret containing the value
  - Mutually exclusive with `value`
  - Cannot be empty

**Validation Rules:**
- Variable name cannot be empty
- Exactly one of `value` or `secret` must be defined
- Variable names must follow Unix conventions (e.g., `DEBUG`, `API_KEY`, `MY_VAR_123`)
- Invalid names include those starting with digits (`1INVALID`) or containing special characters (`INVALID-NAME`, `INVALID@NAME`)

### Mount Paths

Configure additional directories to mount in the workspace runtime.

**Structure:**
```json
{
  "mounts": {
    "dependencies": ["../main"],
    "configs": [".claude", ".gitconfig"]
  }
}
```

**Fields:**
- `dependencies` (optional) - Additional source directories to mount
  - Paths are relative to the workspace sources directory
  - Useful for git worktrees
- `configs` (optional) - Configuration directories to mount from the user's home directory
  - Paths are relative to `$HOME`
  - Useful for sharing Git configs, or tool configurations

**Validation Rules:**
- All paths must be relative (not absolute)
- Paths cannot be empty
- Absolute paths like `/absolute/path` are rejected

### Configuration Validation

When you register a workspace with `kortex-cli init`, the configuration is automatically validated. If `workspace.json` exists and contains invalid data, the registration will fail with a descriptive error message.

**Example - Invalid configuration (both value and secret set):**
```bash
$ kortex-cli init /path/to/project --runtime fake --agent claude
```
```text
Error: workspace configuration validation failed: invalid workspace configuration:
environment variable "API_KEY" (index 0) has both value and secret set
```

**Example - Invalid configuration (absolute path in mounts):**
```bash
$ kortex-cli init /path/to/project --runtime fake --agent claude
```
```text
Error: workspace configuration validation failed: invalid workspace configuration:
dependency mount "/absolute/path" (index 0) must be a relative path
```

### Configuration Examples

**Basic environment variables:**
```json
{
  "environment": [
    {
      "name": "NODE_ENV",
      "value": "development"
    },
    {
      "name": "DEBUG",
      "value": "true"
    }
  ]
}
```

**Using secrets:**
```json
{
  "environment": [
    {
      "name": "API_TOKEN",
      "secret": "github-api-token"
    }
  ]
}
```

**git worktree:**
```json
{
  "mounts": {
    "dependencies": [
      "../main"
    ]
  }
}
```

**Sharing user configurations:**
```json
{
  "mounts": {
    "configs": [
      ".claude",
      ".gitconfig",
      ".kube/config"
    ]
  }
}
```

**Complete configuration:**
```json
{
  "environment": [
    {
      "name": "NODE_ENV",
      "value": "development"
    },
    {
      "name": "DATABASE_URL",
      "secret": "local-db-url"
    }
  ],
  "mounts": {
    "dependencies": ["../main"],
    "configs": [".claude", ".gitconfig"]
  }
}
```

### Notes

- Configuration is **optional** - workspaces can be registered without a `workspace.json` file
- The configuration file is validated only when it exists
- Validation errors are caught early during workspace registration (`init` command)
- All validation rules are enforced to prevent runtime errors
- The configuration model is imported from the `github.com/kortex-hub/kortex-cli-api/workspace-configuration/go` package for consistency across tools

## Multi-Level Configuration

kortex-cli supports configuration at multiple levels, allowing you to customize workspace settings for different contexts. Configurations are automatically merged with proper precedence, making it easy to share common settings while still allowing project and agent-specific customization.

### Configuration Levels

**1. Workspace Configuration** (`.kortex/workspace.json`)
- Stored in your project repository
- Shared with all developers
- Used by all agents
- Committed to version control

**2. Global Project Configuration** (`~/.kortex-cli/config/projects.json` with `""` key)
- User-specific settings applied to **all projects**
- Stored on your local machine (not committed to git)
- Perfect for common settings like `.gitconfig`, SSH keys, or global environment variables
- Never shared with other developers

**3. Project-Specific Configuration** (`~/.kortex-cli/config/projects.json`)
- User-specific settings for a **specific project**
- Stored on your local machine (not committed to git)
- Overrides global settings for this project
- Identified by project ID (git repository URL or directory path)

**4. Agent-Specific Configuration** (`~/.kortex-cli/config/agents.json`)
- User-specific settings for a **specific agent** (Claude, Goose, etc.)
- Stored on your local machine (not committed to git)
- Overrides all other configurations
- Perfect for agent-specific environment variables or tools

### Configuration Precedence

When registering a workspace, configurations are merged in this order (later configs override earlier ones):

1. **Workspace** (`.kortex/workspace.json`) - Base configuration from repository
2. **Global** (projects.json `""` key) - Your global settings for all projects
3. **Project** (projects.json specific project) - Your settings for this project
4. **Agent** (agents.json specific agent) - Your settings for this agent

**Example:** If `DEBUG` is defined in workspace config as `false`, in project config as `true`, and in agent config as `verbose`, the final value will be `verbose` (from agent config).

### Storage Location

User-specific configurations are stored in the kortex-cli storage directory:

- **Default location**: `~/.kortex-cli/config/`
- **Custom location**: Set via `--storage` flag or `KORTEX_CLI_STORAGE` environment variable

The storage directory contains:
- `config/agents.json` - Agent-specific configurations
- `config/projects.json` - Project-specific and global configurations

### Agent Configuration File

**Location**: `~/.kortex-cli/config/agents.json`

**Format**:
```json
{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      }
    ],
    "mounts": {
      "configs": [".claude-config"]
    }
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

Each key is an agent name (e.g., `claude`, `goose`). The value uses the same structure as `workspace.json`.

### Project Configuration File

**Location**: `~/.kortex-cli/config/projects.json`

**Format**:
```json
{
  "": {
    "mounts": {
      "configs": [".gitconfig", ".ssh"]
    }
  },
  "github.com/kortex-hub/kortex-cli": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ],
    "mounts": {
      "dependencies": ["../kortex-common"]
    }
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
- **Empty string `""`** - Global configuration applied to **all projects**
- **Git repository URL** - Configuration for all workspaces in that repository (e.g., `github.com/user/repo`)
- **Directory path** - Configuration for a specific directory (takes precedence over repository URL)

### Use Cases

**Global Settings for All Projects:**
```json
{
  "": {
    "mounts": {
      "configs": [".gitconfig", ".ssh", ".gnupg"]
    }
  }
}
```
This mounts your git config and SSH keys in **every workspace** you create.

**Project-Specific API Keys:**
```json
{
  "github.com/company/project": {
    "environment": [
      {
        "name": "API_KEY",
        "secret": "project-api-key"
      }
    ]
  }
}
```
This adds an API key only for workspaces in the company project.

**Agent-Specific Debug Mode:**
```json
{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      }
    ]
  }
}
```
This enables debug mode only when using the Claude agent.

### Using Multi-Level Configuration

**Register workspace with agent-specific config:**
```bash
kortex-cli init --runtime fake --agent claude
```

**Register workspace with custom project:**
```bash
kortex-cli init --runtime fake --project my-custom-project --agent goose
```

**Note:** The `--agent` flag is required (or set `KORTEX_CLI_DEFAULT_AGENT` environment variable) when registering a workspace.

### Merging Behavior

**Environment Variables:**
- Variables are merged by name
- Later configurations override earlier ones
- Example: If workspace sets `DEBUG=false` and agent sets `DEBUG=true`, the final value is `DEBUG=true`

**Mount Paths:**
- Paths are deduplicated (duplicates removed)
- Order is preserved (first occurrence wins)
- Example: If workspace has `[".gitconfig", ".ssh"]` and global has `[".ssh", ".kube"]`, the result is `[".gitconfig", ".ssh", ".kube"]`

### Configuration Files Don't Exist?

All multi-level configurations are **optional**:
- If `agents.json` doesn't exist, agent-specific configuration is skipped
- If `projects.json` doesn't exist, project and global configurations are skipped
- If `workspace.json` doesn't exist, only user-specific configurations are used

The system works without any configuration files and merges only the ones that exist.

### Example: Complete Multi-Level Setup

**Workspace config** (`.kortex/workspace.json` - committed to git):
```json
{
  "environment": [
    {"name": "NODE_ENV", "value": "development"}
  ]
}
```

**Global config** (`~/.kortex-cli/config/projects.json` - your machine only):
```json
{
  "": {
    "mounts": {
      "configs": [".gitconfig", ".ssh"]
    }
  }
}
```

**Project config** (`~/.kortex-cli/config/projects.json` - your machine only):
```json
{
  "github.com/kortex-hub/kortex-cli": {
    "environment": [
      {"name": "DEBUG", "value": "true"}
    ]
  }
}
```

**Agent config** (`~/.kortex-cli/config/agents.json` - your machine only):
```json
{
  "claude": {
    "environment": [
      {"name": "CLAUDE_VERBOSE", "value": "true"}
    ]
  }
}
```

**Result when running** `kortex-cli init --runtime fake --agent claude`:
- Environment: `NODE_ENV=development`, `DEBUG=true`, `CLAUDE_VERBOSE=true`
- Mounts: `.gitconfig`, `.ssh`

## Commands

### `init` - Register a New Workspace

Registers a new workspace with kortex-cli, making it available for agent launch and configuration.

#### Usage

```bash
kortex-cli init [sources-directory] [flags]
```

#### Arguments

- `sources-directory` - Path to the directory containing your project source files (optional, defaults to current directory `.`)

#### Flags

- `--runtime, -r <type>` - Runtime to use for the workspace (required if `KORTEX_CLI_DEFAULT_RUNTIME` is not set)
- `--agent, -a <name>` - Agent to use for the workspace (required if `KORTEX_CLI_DEFAULT_AGENT` is not set)
- `--workspace-configuration <path>` - Directory for workspace configuration files (default: `<sources-directory>/.kortex`)
- `--name, -n <name>` - Human-readable name for the workspace (default: generated from sources directory)
- `--project, -p <identifier>` - Custom project identifier to override auto-detection (default: auto-detected from git repository or source directory)
- `--verbose, -v` - Show detailed output including all workspace information
- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Register the current directory:**
```bash
kortex-cli init --runtime fake --agent claude
```
Output: `a1b2c3d4e5f6...` (workspace ID)

**Register a specific directory:**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude
```

**Register with a custom name:**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude --name "my-awesome-project"
```

**Register with a custom project identifier:**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude --project "my project"
```

**Register with custom configuration location:**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude --workspace-configuration /path/to/config
```

**View detailed output:**
```bash
kortex-cli init --runtime fake --agent claude --verbose
```
Output:
```text
Registered workspace:
  ID: a1b2c3d4e5f6...
  Name: myproject
  Agent: claude
  Sources directory: /absolute/path/to/myproject
  Configuration directory: /absolute/path/to/myproject/.kortex
```

**JSON output (default - ID only):**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with verbose flag (full workspace details):**
```bash
kortex-cli init /path/to/myproject --runtime fake --agent claude --output json --verbose
```
Output:
```json
{
  "id": "a1b2c3d4e5f6...",
  "name": "myproject",
  "agent": "claude",
  "paths": {
    "source": "/absolute/path/to/myproject",
    "configuration": "/absolute/path/to/myproject/.kortex"
  }
}
```

**JSON output with short flags:**
```bash
kortex-cli init -r fake -a claude -o json -v
```

**Show runtime command output (e.g., image build logs):**
```bash
kortex-cli init --runtime podman --agent claude --show-logs
```

#### Workspace Naming

- If `--name` is not provided, the name is automatically generated from the last component of the sources directory path
- If a workspace with the same name already exists, kortex-cli automatically appends an increment (`-2`, `-3`, etc.) to ensure uniqueness

**Examples:**
```bash
# First workspace in /home/user/project
kortex-cli init /home/user/project --runtime fake --agent claude
# Name: "project"

# Second workspace with the same directory name
kortex-cli init /home/user/another-location/project --runtime fake --agent claude --name "project"
# Name: "project-2"

# Third workspace with the same name
kortex-cli init /tmp/project --runtime fake --agent claude --name "project"
# Name: "project-3"
```

#### Project Detection

When registering a workspace, kortex-cli automatically detects and stores a project identifier. This allows grouping workspaces that belong to the same project, even across different branches, forks, or subdirectories.

**The project is determined using the following rules:**

**1. Git repository with remote URL**

The project is the repository remote URL (without `.git` suffix) plus the workspace's relative path within the repository:

- **At repository root**: `https://github.com/user/repo/`
- **In subdirectory**: `https://github.com/user/repo/sub/path`

**Remote priority:**
1. `upstream` remote is checked first (useful for forks)
2. `origin` remote is used if `upstream` doesn't exist
3. If neither exists, falls back to local repository path (see below)

**Example - Fork with upstream:**
```bash
# Repository setup:
# upstream: https://github.com/kortex-hub/kortex-cli.git
# origin:   https://github.com/myuser/kortex-cli.git (fork)

# Workspace at repository root
kortex-cli init /home/user/kortex-cli --runtime fake --agent claude
# Project: https://github.com/kortex-hub/kortex-cli/

# Workspace in subdirectory
kortex-cli init /home/user/kortex-cli/pkg/git --runtime fake --agent claude
# Project: https://github.com/kortex-hub/kortex-cli/pkg/git
```

This ensures all forks and branches of the same upstream repository are grouped together.

**2. Git repository without remote**

The project is the repository root directory path plus the workspace's relative path:

- **At repository root**: `/home/user/my-local-repo`
- **In subdirectory**: `/home/user/my-local-repo/sub/path`

**Example - Local repository:**
```bash
# Workspace at repository root
kortex-cli init /home/user/local-repo --runtime fake --agent claude
# Project: /home/user/local-repo

# Workspace in subdirectory
kortex-cli init /home/user/local-repo/pkg/utils --runtime fake --agent claude
# Project: /home/user/local-repo/pkg/utils
```

**3. Non-git directory**

The project is the workspace source directory path:

**Example - Regular directory:**
```bash
kortex-cli init /tmp/workspace --runtime fake --agent claude
# Project: /tmp/workspace
```

**Benefits:**

- **Cross-branch grouping**: Workspaces in different git worktrees or branches of the same repository share the same project
- **Fork grouping**: Forks reference the upstream repository, grouping all contributors working on the same project
- **Subdirectory support**: Monorepo subdirectories are tracked with their full path for precise identification
- **Custom override**: Use `--project` flag to manually group workspaces under a custom identifier (e.g., "client-project")
- **Future filtering**: The project field enables filtering and grouping commands (e.g., list all workspaces for a specific project)

#### Notes

- **Runtime is required**: You must specify a runtime using either the `--runtime` flag or the `KORTEX_CLI_DEFAULT_RUNTIME` environment variable
- **Agent is required**: You must specify an agent using either the `--agent` flag or the `KORTEX_CLI_DEFAULT_AGENT` environment variable
- **Project auto-detection**: The project identifier is automatically detected from git repository information or source directory path. Use `--project` flag to override with a custom identifier
- All directory paths are converted to absolute paths for consistency
- The workspace ID is a unique identifier generated automatically
- Workspaces can be listed using the `workspace list` command
- The default configuration directory (`.kortex`) is created inside the sources directory unless specified otherwise
- JSON output format is useful for scripting and automation
- Without `--verbose`, JSON output returns only the workspace ID
- With `--verbose`, JSON output includes full workspace details (ID, name, agent, paths)
- Use `--show-logs` to display the full stdout and stderr from runtime commands (e.g., `podman build` output during image creation)
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace list` - List All Registered Workspaces

Lists all workspaces that have been registered with kortex-cli. Also available as the shorter alias `list`.

#### Usage

```bash
kortex-cli workspace list [flags]
kortex-cli list [flags]
```

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**List all workspaces (human-readable format):**
```bash
kortex-cli workspace list
```
Output:
```text
ID: a1b2c3d4e5f6...
  Name: myproject
  Agent: claude
  Sources: /absolute/path/to/myproject
  Configuration: /absolute/path/to/myproject/.kortex

ID: f6e5d4c3b2a1...
  Name: another-project
  Agent: goose
  Sources: /absolute/path/to/another-project
  Configuration: /absolute/path/to/another-project/.kortex
```

**Use the short alias:**
```bash
kortex-cli list
```

**List workspaces in JSON format:**
```bash
kortex-cli workspace list --output json
```
Output:
```json
{
  "items": [
    {
      "id": "a1b2c3d4e5f6...",
      "name": "myproject",
      "agent": "claude",
      "paths": {
        "source": "/absolute/path/to/myproject",
        "configuration": "/absolute/path/to/myproject/.kortex"
      }
    },
    {
      "id": "f6e5d4c3b2a1...",
      "name": "another-project",
      "agent": "goose",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kortex"
      }
    }
  ]
}
```

**List with short flag:**
```bash
kortex-cli list -o json
```

#### Notes

- When no workspaces are registered, the command displays "No workspaces registered"
- The JSON output format is useful for scripting and automation
- All paths are displayed as absolute paths for consistency
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace start` - Start a Workspace

Starts a registered workspace by its ID. Also available as the shorter alias `start`.

#### Usage

```bash
kortex-cli workspace start ID [flags]
kortex-cli start ID [flags]
```

#### Arguments

- `ID` - The unique identifier of the workspace to start (required)

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Start a workspace by ID:**
```bash
kortex-cli workspace start a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of started workspace)

**Use the short alias:**
```bash
kortex-cli start a1b2c3d4e5f6...
```

**View workspace IDs before starting:**
```bash
# First, list all workspaces to find the ID
kortex-cli list

# Then start the desired workspace
kortex-cli start a1b2c3d4e5f6...
```

**JSON output:**
```bash
kortex-cli workspace start a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kortex-cli start a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kortex-cli workspace start a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kortex-cli start invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kortex-cli start invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

#### Notes

- The workspace ID is required and can be obtained using the `workspace list` or `list` command
- Starting a workspace launches its associated runtime instance
- Upon successful start, the command outputs the ID of the started workspace
- The workspace runtime state is updated to reflect that it's running
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace stop` - Stop a Workspace

Stops a running workspace by its ID. Also available as the shorter alias `stop`.

#### Usage

```bash
kortex-cli workspace stop ID [flags]
kortex-cli stop ID [flags]
```

#### Arguments

- `ID` - The unique identifier of the workspace to stop (required)

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Stop a workspace by ID:**
```bash
kortex-cli workspace stop a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of stopped workspace)

**Use the short alias:**
```bash
kortex-cli stop a1b2c3d4e5f6...
```

**View workspace IDs before stopping:**
```bash
# First, list all workspaces to find the ID
kortex-cli list

# Then stop the desired workspace
kortex-cli stop a1b2c3d4e5f6...
```

**JSON output:**
```bash
kortex-cli workspace stop a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kortex-cli stop a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kortex-cli workspace stop a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kortex-cli stop invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kortex-cli stop invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

#### Notes

- The workspace ID is required and can be obtained using the `workspace list` or `list` command
- Stopping a workspace stops its associated runtime instance
- Upon successful stop, the command outputs the ID of the stopped workspace
- The workspace runtime state is updated to reflect that it's stopped
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace terminal` - Connect to a Running Workspace

Connects to a running workspace with an interactive terminal session. Also available as the shorter alias `terminal`.

#### Usage

```bash
kortex-cli workspace terminal ID [COMMAND...] [flags]
kortex-cli terminal ID [COMMAND...] [flags]
```

#### Arguments

- `ID` - The unique identifier of the workspace to connect to (required)
- `COMMAND...` - Optional command to execute instead of the default agent command

#### Flags

- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Connect using the default agent command:**
```bash
kortex-cli workspace terminal a1b2c3d4e5f6...
```

This starts an interactive session with the default agent (typically Claude Code) inside the running workspace container.

**Use the short alias:**
```bash
kortex-cli terminal a1b2c3d4e5f6...
```

**Run a bash shell:**
```bash
kortex-cli terminal a1b2c3d4e5f6... bash
```

**Run a command with flags (use -- to prevent kortex-cli from parsing them):**
```bash
kortex-cli terminal a1b2c3d4e5f6... -- bash -c 'echo hello'
```

The `--` separator tells kortex-cli to stop parsing flags and pass everything after it directly to the container. This is useful when your command includes flags that would otherwise be interpreted by kortex-cli.

**List workspaces and connect to a running one:**
```bash
# First, list all workspaces to find the ID
kortex-cli list

# Start a workspace if it's not running
kortex-cli start a1b2c3d4e5f6...

# Then connect with a terminal
kortex-cli terminal a1b2c3d4e5f6...
```

#### Error Handling

**Workspace not found:**
```bash
kortex-cli terminal invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not running:**
```bash
kortex-cli terminal a1b2c3d4e5f6...
```
Output:
```text
Error: instance is not running (current state: created)
```

In this case, you need to start the workspace first:
```bash
kortex-cli start a1b2c3d4e5f6...
kortex-cli terminal a1b2c3d4e5f6...
```

#### Notes

- The workspace must be in a **running state** before you can connect to it. Use `workspace start` to start a workspace first
- The workspace ID is required and can be obtained using the `workspace list` or `list` command
- By default (when no command is provided), the runtime uses the `terminal_command` from the agent's configuration file
  - For example, if the workspace was created with `--agent claude`, it will use the command defined in `claude.json` (typically `["claude"]`)
  - This ensures you connect directly to the configured agent
- You can override the default by providing a custom command (e.g., `bash`, `python`, or any executable available in the container)
- Use the `--` separator when your command includes flags to prevent kortex-cli from trying to parse them
- The terminal session is fully interactive with stdin/stdout/stderr connected to your terminal
- The command execution happens inside the workspace's container/runtime environment
- JSON output is **not supported** for this command as it's inherently interactive
- Runtime support: The terminal command requires the runtime to implement the Terminal interface. The Podman runtime supports this using `podman exec -it`

### `workspace remove` - Remove a Workspace

Removes a registered workspace by its ID. Also available as the shorter alias `remove`.

#### Usage

```bash
kortex-cli workspace remove ID [flags]
kortex-cli remove ID [flags]
```

#### Arguments

- `ID` - The unique identifier of the workspace to remove (required)

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Remove a workspace by ID:**
```bash
kortex-cli workspace remove a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of removed workspace)

**Use the short alias:**
```bash
kortex-cli remove a1b2c3d4e5f6...
```

**View workspace IDs before removing:**
```bash
# First, list all workspaces to find the ID
kortex-cli list

# Then remove the desired workspace
kortex-cli remove a1b2c3d4e5f6...
```

**JSON output:**
```bash
kortex-cli workspace remove a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kortex-cli remove a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kortex-cli workspace remove a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kortex-cli remove invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kortex-cli remove invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

#### Notes

- The workspace ID is required and can be obtained using the `workspace list` or `list` command
- Removing a workspace only unregisters it from kortex-cli; it does not delete any files from the sources or configuration directories
- If the workspace ID is not found, the command will fail with a helpful error message
- Upon successful removal, the command outputs the ID of the removed workspace
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure
