# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

kdn is a command-line interface for launching and managing AI agents (Claude Code, Goose, Cursor, OpenCode) with custom configurations. It provides a unified way to start different agents with specific settings including skills, MCP server connections, and LLM integrations.

## Build and Test Commands

All build and test commands are available through the Makefile. Run `make help` to see all available commands.

### Build
```bash
make build
```

### Execute
After building, the `kdn` binary will be created in the current directory:

```bash
# Display help and available commands
./kdn --help

# Execute a specific command
./kdn <command> [flags]
```

### Run Tests
```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage
```

For more granular testing (specific packages or tests), use Go directly:
```bash
# Run tests in a specific package
go test ./pkg/cmd

# Run a specific test
go test -run TestName ./pkg/cmd
```

### Format Code
```bash
# Format all Go files in the project
make fmt

# Check if code is formatted (without modifying files)
make check-fmt
```

Code should be formatted before committing. Run `make fmt` to ensure consistent style across the codebase.

### Integration Tests
```bash
# Run integration tests (requires Podman)
make test-integration
```

### Additional Commands
```bash
# Run go vet
make vet

# Run all CI checks (format check, vet, tests)
make ci-checks

# Clean build artifacts
make clean

# Install binary to GOPATH/bin
make install
```

## Architecture

### Command Structure (Cobra-based)
- Entry point: `cmd/kdn/main.go` → calls `cmd.NewRootCmd().Execute()` and handles errors with `os.Exit(1)`
- Root command: `pkg/cmd/root.go` exports `NewRootCmd()` which creates and configures the root command
- Subcommands: Each command is in `pkg/cmd/<command>.go` with a `New<Command>Cmd()` factory function
- Commands use a factory pattern: each command exports a `New<Command>Cmd()` function that returns `*cobra.Command`
- Command registration: `NewRootCmd()` calls `rootCmd.AddCommand(New<Command>Cmd())` for each subcommand
- No global variables or `init()` functions - all configuration is explicit through factory functions

### Global Flags
Global flags are defined as persistent flags in `pkg/cmd/root.go` and are available to all commands.

#### Accessing the --storage Flag

The `--storage` flag specifies the directory where kdn stores all its files. The default path is computed at runtime using `os.UserHomeDir()` and `filepath.Join()` to ensure cross-platform compatibility (Linux, macOS, Windows). The default is `$HOME/.kdn` with a fallback to `.kdn` in the current directory if the home directory cannot be determined.

**Environment Variable**: The `KDN_STORAGE` environment variable can be used to set the storage directory path. The flag `--storage` will override the environment variable if both are specified.

**Priority order** (highest to lowest):
1. `--storage` flag (if specified)
2. `KDN_STORAGE` environment variable (if set)
3. Default: `$HOME/.kdn`

To access this value in any command:

```go
func NewExampleCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "example",
        Short: "An example command",
        Run: func(cmd *cobra.Command, args []string) {
            storagePath, _ := cmd.Flags().GetString("storage")
            // Use storagePath...
        },
    }
}
```

**Important**: Never hardcode paths with `~` as it's not cross-platform. Always use `os.UserHomeDir()` and `filepath.Join()` for path construction.

### Module Design Pattern

All modules (packages outside of `cmd/`) MUST follow the interface-based design pattern to ensure proper encapsulation, testability, and API safety.

**Required Pattern:**
1. **Public types are interfaces** - All public types must be declared as interfaces
2. **Implementations are unexported** - Concrete struct implementations must be unexported (lowercase names)
3. **Compile-time interface checks** - Add unnamed variable declarations to verify interface implementation at compile time
4. **Factory functions** - Provide `New*()` functions that return the interface type

**Benefits:**
- Prevents direct struct instantiation (compile-time enforcement)
- Forces usage of factory functions for proper validation and initialization
- Enables easy mocking in tests
- Clear API boundaries
- Better encapsulation

**This pattern is MANDATORY for all new modules in `pkg/`.**

### JSON Storage Structure

When designing JSON storage structures for persistent data, use **nested objects with subfields** instead of flat structures with naming conventions.

**Preferred Pattern (nested structure):**
```json
{
  "id": "dc610bffa75f21b5b043f98aff12b157fb16fae6c0ac3139c28f85d6defbe017",
  "paths": {
    "source": "/Users/user/project",
    "configuration": "/Users/user/project/.kaiden"
  }
}
```

**Benefits:**
- **Better organization** - Related fields are grouped together
- **Clarity** - Field relationships are explicit through nesting
- **Extensibility** - Easy to add new subfields without polluting the top level
- **No naming conflicts** - Avoids debates about snake_case vs camelCase
- **Self-documenting** - Structure communicates intent

### Runtime System

The runtime system provides a pluggable architecture for managing workspaces on different container/VM platforms (Podman, MicroVM, Kubernetes, etc.).

**Key Components:**
- **Runtime Interface** (`pkg/runtime/runtime.go`): Contract all runtimes must implement
- **Registry** (`pkg/runtime/registry.go`): Manages runtime registration and discovery
- **Runtime Implementations** (`pkg/runtime/<runtime-name>/`): Platform-specific packages (e.g., `fake`, `podman`)
- **Centralized Registration** (`pkg/runtimesetup/register.go`): Automatically registers all available runtimes

**Optional Interfaces:**
- **StorageAware**: Enables runtimes to persist data in a dedicated storage directory
- **AgentLister**: Enables runtimes to report which agents they support
- **Terminal**: Enables interactive terminal sessions with instances (auto-starts if needed)

**For detailed runtime implementation guidance, use:** `/working-with-runtime-system`

**To add a new runtime, use:** `/add-runtime`

### Secret Service System

The secret service system provides a pluggable architecture for managing secret service definitions that describe how secrets are applied to workspace requests.

**Key Components:**
- **SecretService Interface** (`pkg/secretservice/secretservice.go`): Contract all secret services must implement (`Name()`, `HostPattern()`, `Path()`, `EnvVars()`, `HeaderName()`, `HeaderTemplate()`)
- **Registry** (`pkg/secretservice/registry.go`): Manages secret service registration and discovery
- **Centralized Registration** (`pkg/secretservicesetup/register.go`): Automatically registers all available secret services

### StepLogger System

The StepLogger system provides user-facing progress feedback during runtime operations with spinners and completion messages.

**Key Points:**
- Commands inject StepLogger into context based on output mode (text with spinners vs JSON silent)
- Runtime methods retrieve logger from context and report progress steps
- Automatic behavior: animated spinners in text mode, silent in JSON mode

**For detailed StepLogger integration guidance, use:** `/working-with-steplogger`

### Logger System

The Logger system (`pkg/logger`) routes stdout and stderr from runtime CLI commands (e.g., `podman build`) to the user. It is controlled by the `--show-logs` flag.

**Key Points:**
- Commands inject a `logger.Logger` into context based on the `--show-logs` flag
- Runtime methods retrieve it from context and pass its writers to CLI command execution
- When `--show-logs` is set, output is written to the command's stdout/stderr; otherwise it is discarded
- `--show-logs` cannot be combined with `--output json` (enforced in `preRun`)

**Interface** (`pkg/logger/logger.go`):
```go
type Logger interface {
    Stdout() io.Writer
    Stderr() io.Writer
}
```

**Context integration** (`pkg/logger/context.go`): `WithLogger()` / `FromContext()` — mirrors the StepLogger pattern.

### Config System

The config system manages workspace configuration for **injecting environment variables, mounting directories, providing skills, configuring MCP servers, managing secrets and controlling network access** into workspaces (different from runtime-specific configuration).

**Multi-Level Configuration:**
- **Workspace-level** (`.kaiden/workspace.json`) - Project configuration, set via `--workspace-configuration` flag
- **Project-specific** (`~/.kdn/config/projects.json`) - User's custom config for specific projects
- **Global** (empty string `""` key in `projects.json`) - Settings applied to all projects
- **Agent-specific** (`~/.kdn/config/agents.json`) - Per-agent overrides

**Configuration Precedence:** Agent > Project > Global > Workspace (highest to lowest)

### Agent Default Settings Files

A separate mechanism (distinct from env/mount config) allows default dotfiles to be baked into the workspace image:

- **Location:** `~/.kdn/config/<agent>/` (e.g., `~/.kdn/config/claude/`)
- Files are read by `manager.readAgentSettings()` into a `map[string][]byte` and passed to the runtime via `runtime.CreateParams.AgentSettings`
- After reading, the manager calls `agent.SkipOnboarding()`, `agent.SetModel()` (if a model is set), and `agent.SetMCPServers()` (if MCP is configured) to further modify the settings map
- The Podman runtime writes these files into the build context as `agent-settings/` and adds `COPY --chown=agent:agent agent-settings/. /home/agent/` to the Containerfile
- Result: every file under `config/<agent>/` lands at the corresponding path under `/home/agent/` inside the image

**For detailed guidance, use:** `/working-with-config-system`

**For detailed configuration guidance, use:** `/working-with-config-system`

### Podman Runtime Configuration

The Podman runtime supports runtime-specific configuration for **building and configuring containers** (base image, packages, sudo permissions, agent setup).

**Configuration Files:**
- `<storage-dir>/runtimes/podman/config/image.json` - Base image configuration
- `<storage-dir>/runtimes/podman/config/claude.json` - Claude agent configuration
- `<storage-dir>/runtimes/podman/config/goose.json` - Goose agent configuration
- `<storage-dir>/runtimes/podman/config/opencode.json` - OpenCode agent configuration

**For Podman runtime configuration details, use:** `/working-with-podman-runtime-config`

### Skills System

Skills are reusable capabilities that can be discovered and executed by AI agents:

- **Location**: `.agents/skills/<skill-name>/SKILL.md`
- **Claude support**: `.claude/skills` is a symlink to `../.agents/skills`, so Claude Code discovers skills automatically
- **Format**: Each SKILL.md contains:
  - YAML frontmatter with `name`, `description`, `argument-hint`
  - Detailed instructions for execution
  - Usage examples

Skills can be provided to workspaces via the `skills` field in `workspace.json` (or any other config level). Each entry is the path to a single skill directory on the host. kdn mounts it read-only into the agent's skills directory inside the container using the directory's basename as the skill name:

| Agent | Container skills directory |
|-------|--------------------------|
| Claude Code | `$HOME/.claude/skills/` |
| Goose | `$HOME/.agents/skills/` |
| Cursor | `$HOME/.cursor/skills/` |
| OpenCode | `$HOME/.opencode/skills/` |

The `Agent` interface (`pkg/agent/agent.go`) exposes `SkillsDir() string` which returns the container path (using the `$HOME` variable) where skill directories should be mounted. The manager calls this during `Add()` to convert `WorkspaceConfig.Skills` entries into `workspace.Mount` entries before passing the config to the runtime.

### Adding a New Skill
1. Create directory: `.agents/skills/<skill-name>/`
2. Create SKILL.md with frontmatter and instructions
3. No symlink step needed — `.claude/skills` already symlinks to `.agents/skills/`

### Adding a New Command

**Available Skills:**
- `/add-command-simple` - For commands without JSON output support
- `/add-command-with-json` - For commands with JSON output support
- `/add-alias-command` - For alias commands that delegate to existing commands
- `/add-parent-command` - For parent commands with subcommands

**All commands MUST:**
- Define the `Args` field for argument validation (`cobra.NoArgs`, `cobra.ExactArgs(n)`, etc.)
- Include an `Example` field with usage examples
- Have a corresponding `Test<Command>Cmd_Examples` test function to validate examples

**For advanced command patterns, use:** `/implementing-command-patterns`

**For testing commands, use:** `/testing-commands`

### Working with the Instances Manager

The instances manager provides the API for managing workspace instances:

```go
// Create manager
manager, err := instances.NewManager(storageDir)

// Register runtimes
runtimesetup.RegisterAll(manager)

// Register agents
agentsetup.RegisterAll(manager)

// Register secret services
secretservicesetup.RegisterAll(manager)

// Add instance
manager.Add(ctx, instances.AddOptions{...})

// List, Get, Delete instances
manager.List()
manager.Get(id)
manager.Delete(id)

// Start, Stop instances
manager.Start(ctx, id)
manager.Stop(ctx, id)

// Interactive terminal
manager.Terminal(ctx, id, []string{"bash"})
```

**Workspace Name Sanitization:** The manager automatically sanitizes workspace names — whether auto-generated from the source directory basename or provided via `--name`. Names are lowercased and any run of invalid characters (spaces, `@`, etc.) is collapsed into a single hyphen. This ensures compatibility with runtimes like Podman that require lowercase image names.

**For detailed manager API and project detection, use:** `/working-with-instances-manager`

### Cross-Platform Path Handling

⚠️ **CRITICAL**: All path operations and tests MUST be cross-platform compatible (Linux, macOS, Windows).

**Core Rules:**
- **Host paths** (files on disk): Use `filepath.Join()` - works on Windows, macOS, Linux
- **Container paths** (inside Podman): Use `path.Join()` - containers are always Unix/Linux
- Convert relative paths to absolute with `filepath.Abs()`
- Never hardcode paths with `~` - use `os.UserHomeDir()` instead
- **Use `t.TempDir()` for ALL temporary directories in tests**

**Example:**
```go
import (
    "path"           // For container paths
    "path/filepath"  // For host paths
)

// Host path (cross-platform)
configDir := filepath.Join(storageDir, ".kaiden")

// Container path (always Unix)
workspacePath := path.Join("/workspace", "sources")
```

**For detailed cross-platform patterns, use:** `/cross-platform-development`

## Documentation Standards

### Markdown Best Practices

All markdown files (*.md) in this repository must follow these standards:

**Fenced Code Blocks:**
- **ALWAYS** include a language tag in fenced code blocks
- Use the appropriate language identifier (`bash`, `go`, `json`, `yaml`, `text`, etc.)
- For output examples or plain text content, use `text` as the language tag
- This ensures markdown linters (markdownlint MD040) pass and improves syntax highlighting

**Common Language Tags:**
- `bash` - Shell commands and scripts
- `go` - Go source code
- `json` - JSON data structures
- `yaml` - YAML configuration files
- `text` - Plain text output, error messages, or generic content
- `markdown` - Markdown examples

## Copyright Headers

All source files must include Apache License 2.0 copyright headers with Red Hat copyright. Use the `/copyright-headers` skill to add or update headers automatically. The current year is 2026.

## Dependencies

- Cobra (github.com/spf13/cobra): CLI framework
- Go 1.26+

## Testing

Tests follow Go conventions with `*_test.go` files alongside source files. Tests use the standard `testing` package and should cover command initialization, execution, and error cases.

### Parallel Test Execution

**All tests MUST call `t.Parallel()` as the first line of the test function.**

**Exception:** Tests using `t.Setenv()` cannot use `t.Parallel()` on the parent test function.

**For general testing best practices, use:** `/testing-best-practices`

**For command testing patterns, use:** `/testing-commands`

**For cross-platform testing, use:** `/cross-platform-development`

**Before submitting a PR (code, tests, docs checklist), use:** `/complete-pr`

## GitHub Actions

GitHub Actions workflows are stored in `.github/workflows/`. All workflows must use commit SHA1 hashes instead of version tags for security reasons (to prevent supply chain attacks from tag manipulation).

Example:
```yaml
- uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
```

Always include the version as a comment for readability.
