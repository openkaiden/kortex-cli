# kortex-cli

[![codecov](https://codecov.io/gh/kortex-hub/kortex-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/kortex-hub/kortex-cli)

## Introduction

kortex-cli is a command-line interface for launching and managing AI agents with custom configurations. It provides a unified way to start different agents with specific settings including skills, MCP (Model Context Protocol) server connections, and LLM integrations.

### Supported Agents

- **Claude Code** - Anthropic's official CLI for Claude
- **Goose** - AI agent for development tasks
- **Cursor** - AI-powered code editor agent

### Key Features

- Configure agents with custom skills and capabilities
- Connect to MCP servers for extended functionality
- Integrate with various LLM providers
- Consistent interface across different agent types

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
$ kortex-cli init /path/to/project -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 3: Register with verbose output to get full details**

```bash
$ kortex-cli init /path/to/another-project -o json -v
```

```json
{
  "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
  "name": "another-project",
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
      "paths": {
        "source": "/absolute/path/to/project",
        "configuration": "/absolute/path/to/project/.kortex"
      }
    },
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kortex"
      }
    }
  ]
}
```

Exit code: `0` (success)

**Step 5: Remove a workspace**

```bash
$ kortex-cli workspace remove 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 6: Verify removal**

```bash
$ kortex-cli workspace list -o json
```

```json
{
  "items": [
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
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
$ kortex-cli init /tmp/no-exist -o json
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
output=$(kortex-cli init /path/to/project -o json)
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

- `--workspace-configuration <path>` - Directory for workspace configuration files (default: `<sources-directory>/.kortex`)
- `--name, -n <name>` - Human-readable name for the workspace (default: generated from sources directory)
- `--verbose, -v` - Show detailed output including all workspace information
- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kortex-cli data (default: `$HOME/.kortex-cli`)

#### Examples

**Register the current directory:**
```bash
kortex-cli init
```
Output: `a1b2c3d4e5f6...` (workspace ID)

**Register a specific directory:**
```bash
kortex-cli init /path/to/myproject
```

**Register with a custom name:**
```bash
kortex-cli init /path/to/myproject --name "my-awesome-project"
```

**Register with custom configuration location:**
```bash
kortex-cli init /path/to/myproject --workspace-configuration /path/to/config
```

**View detailed output:**
```bash
kortex-cli init --verbose
```
Output:
```text
Registered workspace:
  ID: a1b2c3d4e5f6...
  Name: myproject
  Sources directory: /absolute/path/to/myproject
  Configuration directory: /absolute/path/to/myproject/.kortex
```

**JSON output (default - ID only):**
```bash
kortex-cli init /path/to/myproject --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with verbose flag (full workspace details):**
```bash
kortex-cli init /path/to/myproject --output json --verbose
```
Output:
```json
{
  "id": "a1b2c3d4e5f6...",
  "name": "myproject",
  "paths": {
    "source": "/absolute/path/to/myproject",
    "configuration": "/absolute/path/to/myproject/.kortex"
  }
}
```

**JSON output with short flags:**
```bash
kortex-cli init -o json -v
```

#### Workspace Naming

- If `--name` is not provided, the name is automatically generated from the last component of the sources directory path
- If a workspace with the same name already exists, kortex-cli automatically appends an increment (`-2`, `-3`, etc.) to ensure uniqueness

**Examples:**
```bash
# First workspace in /home/user/project
kortex-cli init /home/user/project
# Name: "project"

# Second workspace with the same directory name
kortex-cli init /home/user/another-location/project --name "project"
# Name: "project-2"

# Third workspace with the same name
kortex-cli init /tmp/project --name "project"
# Name: "project-3"
```

#### Notes

- All directory paths are converted to absolute paths for consistency
- The workspace ID is a unique identifier generated automatically
- Workspaces can be listed using the `workspace list` command
- The default configuration directory (`.kortex`) is created inside the sources directory unless specified otherwise
- JSON output format is useful for scripting and automation
- Without `--verbose`, JSON output returns only the workspace ID
- With `--verbose`, JSON output includes full workspace details (ID, name, paths)
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
  Sources: /absolute/path/to/myproject
  Configuration: /absolute/path/to/myproject/.kortex

ID: f6e5d4c3b2a1...
  Name: another-project
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
      "paths": {
        "source": "/absolute/path/to/myproject",
        "configuration": "/absolute/path/to/myproject/.kortex"
      }
    },
    {
      "id": "f6e5d4c3b2a1...",
      "name": "another-project",
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
