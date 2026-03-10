# kortex-cli

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

#### Error Handling

**Workspace not found:**
```bash
kortex-cli remove invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

#### Notes

- The workspace ID is required and can be obtained using the `workspace list` or `list` command
- Removing a workspace only unregisters it from kortex-cli; it does not delete any files from the sources or configuration directories
- If the workspace ID is not found, the command will fail with a helpful error message
- Upon successful removal, the command outputs the ID of the removed workspace
