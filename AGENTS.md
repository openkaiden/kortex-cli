# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

kortex-cli is a command-line interface for launching and managing AI agents (Claude Code, Goose, Cursor) with custom configurations. It provides a unified way to start different agents with specific settings including skills, MCP server connections, and LLM integrations.

## Build and Test Commands

All build and test commands are available through the Makefile. Run `make help` to see all available commands.

### Build
```bash
make build
```

### Execute
After building, the `kortex-cli` binary will be created in the current directory:

```bash
# Display help and available commands
./kortex-cli --help

# Execute a specific command
./kortex-cli <command> [flags]
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
- Entry point: `cmd/kortex-cli/main.go` → calls `cmd.NewRootCmd().Execute()` and handles errors with `os.Exit(1)`
- Root command: `pkg/cmd/root.go` exports `NewRootCmd()` which creates and configures the root command
- Subcommands: Each command is in `pkg/cmd/<command>.go` with a `New<Command>Cmd()` factory function
- Commands use a factory pattern: each command exports a `New<Command>Cmd()` function that returns `*cobra.Command`
- Command registration: `NewRootCmd()` calls `rootCmd.AddCommand(New<Command>Cmd())` for each subcommand
- No global variables or `init()` functions - all configuration is explicit through factory functions

### Global Flags
Global flags are defined as persistent flags in `pkg/cmd/root.go` and are available to all commands.

#### Accessing the --storage Flag
The `--storage` flag specifies the directory where kortex-cli stores all its files. The default path is computed at runtime using `os.UserHomeDir()` and `filepath.Join()` to ensure cross-platform compatibility (Linux, macOS, Windows). The default is `$HOME/.kortex-cli` with a fallback to `.kortex-cli` in the current directory if the home directory cannot be determined.

**Environment Variable**: The `KORTEX_CLI_STORAGE` environment variable can be used to set the storage directory path. The flag `--storage` will override the environment variable if both are specified.

**Priority order** (highest to lowest):
1. `--storage` flag (if specified)
2. `KORTEX_CLI_STORAGE` environment variable (if set)
3. Default: `$HOME/.kortex-cli`

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
    "configuration": "/Users/user/project/.kortex"
  }
}
```

**Avoid (flat structure with snake_case or camelCase):**
```json
{
  "id": "...",
  "source_dir": "/Users/user/project",      // Don't use snake_case
  "config_dir": "/Users/user/project/.kortex"
}
```

```json
{
  "id": "...",
  "sourceDir": "/Users/user/project",       // Don't use camelCase
  "configDir": "/Users/user/project/.kortex"
}
```

**Benefits:**
- **Better organization** - Related fields are grouped together
- **Clarity** - Field relationships are explicit through nesting
- **Extensibility** - Easy to add new subfields without polluting the top level
- **No naming conflicts** - Avoids debates about snake_case vs camelCase
- **Self-documenting** - Structure communicates intent

**Implementation:**
- Create nested structs with `json` tags
- Use lowercase field names in JSON (Go convention for exported fields + json tags)
- Group related fields under descriptive parent keys

**Example:**
```go
type InstancePaths struct {
    Source        string `json:"source"`
    Configuration string `json:"configuration"`
}

type InstanceData struct {
    ID    string        `json:"id"`
    Paths InstancePaths `json:"paths"`
}
```

### Runtime System

The runtime system provides a pluggable architecture for managing workspaces on different container/VM platforms (Podman, MicroVM, Kubernetes, etc.).

**Key Components:**
- **Runtime Interface** (`pkg/runtime/runtime.go`): Contract all runtimes must implement
- **Registry** (`pkg/runtime/registry.go`): Manages runtime registration and discovery
- **Runtime Implementations** (`pkg/runtime/<runtime-name>/`): Platform-specific packages (e.g., `fake`)
- **Centralized Registration** (`pkg/runtimesetup/register.go`): Automatically registers all available runtimes

**Adding a New Runtime:**

Use the `/add-runtime` skill which provides step-by-step instructions for creating a new runtime implementation. The `fake` runtime in `pkg/runtime/fake/` serves as a reference implementation.

**Runtime Registration in Commands:**

Commands use `runtimesetup.RegisterAll()` to automatically register all available runtimes:

```go
import "github.com/kortex-hub/kortex-cli/pkg/runtimesetup"

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

### Config System

The config system manages workspace configuration stored in the `.kortex` directory. It provides an interface for reading and validating workspace settings including environment variables and mount points.

**Key Components:**
- **Config Interface** (`pkg/config/config.go`): Interface for managing configuration directories
- **WorkspaceConfiguration Model**: Imported from `github.com/kortex-hub/kortex-cli-api/workspace-configuration/go`
- **Configuration File**: `workspace.json` within the `.kortex` directory

**Configuration Structure:**

The `workspace.json` file follows the nested JSON structure pattern:

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
    "dependencies": ["../main"],
    "configs": [".ssh", ".gitconfig"]
  }
}
```

**Model Fields:**
- `environment` - Environment variables with either hardcoded `value` or `secret` reference (optional)
  - `name` - Variable name (must be valid Unix environment variable name)
  - `value` - Hardcoded value (mutually exclusive with `secret`, empty strings allowed)
  - `secret` - Secret reference (mutually exclusive with `value`, cannot be empty)
- `mounts.dependencies` - Additional source directories to mount (optional)
  - Paths must be relative (not absolute)
  - Paths cannot be empty
  - Relative to workspace sources directory
- `mounts.configs` - Configuration directories to mount (optional)
  - Paths must be relative (not absolute)
  - Paths cannot be empty
  - Relative to `$HOME`

**Using the Config Interface:**

```go
import (
    "github.com/kortex-hub/kortex-cli/pkg/config"
    workspace "github.com/kortex-hub/kortex-cli-api/workspace-configuration/go"
)

// Create a config manager for a workspace
cfg, err := config.NewConfig("/path/to/workspace/.kortex")
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
    if workspaceCfg.Mounts.Dependencies != nil {
        // Use dependency paths
    }
    if workspaceCfg.Mounts.Configs != nil {
        // Use config paths
    }
}
```

**Configuration Validation:**

The `Load()` method automatically validates the configuration and returns `ErrInvalidConfig` if any of these rules are violated:

**Environment Variables:**
- Name cannot be empty
- Name must be a valid Unix environment variable name (starts with letter or underscore, followed by letters, digits, or underscores)
- Exactly one of `value` or `secret` must be defined
- Secret references cannot be empty strings
- Empty values are allowed (valid use case: set env var to empty string)

**Mount Paths:**
- Dependency paths cannot be empty
- Dependency paths must be relative (not absolute)
- Config paths cannot be empty
- Config paths must be relative (not absolute)

**Error Handling:**
- `config.ErrInvalidPath` - Configuration path is empty or invalid
- `config.ErrConfigNotFound` - The `workspace.json` file is not found
- `config.ErrInvalidConfig` - Configuration validation failed (includes detailed error message)

**Design Principles:**
- Configuration directory is NOT automatically created
- Missing configuration directory is treated as empty/default configuration
- All configurations are validated on load to catch errors early
- Follows the module design pattern with interface-based API
- Uses nested JSON structure for clarity and extensibility
- Model types are imported from external API package for consistency

### Skills System
Skills are reusable capabilities that can be discovered and executed by AI agents:
- **Location**: `skills/<skill-name>/SKILL.md`
- **Claude support**: Skills are symlinked in `.claude/skills/` for Claude Code
- **Format**: Each SKILL.md contains:
  - YAML frontmatter with `name`, `description`, `argument-hint`
  - Detailed instructions for execution
  - Usage examples

### Adding a New Skill
1. Create directory: `skills/<skill-name>/`
2. Create SKILL.md with frontmatter and instructions
3. Symlink in `.claude/skills/`: `ln -s ../../skills/<skill-name> .claude/skills/<skill-name>`

### Adding a New Command
1. Create `pkg/cmd/<command>.go` with a `New<Command>Cmd()` function that returns `*cobra.Command`
2. In the `New<Command>Cmd()` function:
   - Create and configure the `cobra.Command`
   - **IMPORTANT**: Always define the `Args` field to specify argument validation
   - **IMPORTANT**: Always add an `Example` field with usage examples
   - Set up any flags or subcommands
   - Return the configured command
3. Register the command in `pkg/cmd/root.go` by adding `rootCmd.AddCommand(New<Command>Cmd())` in the `NewRootCmd()` function
4. Create corresponding test file `pkg/cmd/<command>_test.go`
5. In tests, create command instances using `NewRootCmd()` or `New<Command>Cmd()` as needed
6. **IMPORTANT**: Add a `Test<Command>Cmd_Examples` function to validate the examples

**Command Argument Validation:**

All commands **MUST** declare the `Args` field to specify argument validation behavior. Common options:
- `cobra.NoArgs` - Command accepts no arguments (most common for parent commands and no-arg commands)
- `cobra.ExactArgs(n)` - Command requires exactly n arguments
- `cobra.MinimumNArgs(n)` - Command requires at least n arguments
- `cobra.MaximumNArgs(n)` - Command accepts up to n arguments
- `cobra.RangeArgs(min, max)` - Command accepts between min and max arguments

Example:
```go
// pkg/cmd/example.go
func NewExampleCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "example",
        Short: "An example command",
        Example: `# Run the example command
kortex-cli example`,
        Args:  cobra.NoArgs,  // Always declare Args field
        Run: func(cmd *cobra.Command, args []string) {
            // Command logic here
        },
    }
}

// In pkg/cmd/root.go, add to NewRootCmd():
rootCmd.AddCommand(NewExampleCmd())
```

**Command Examples:**

All commands **MUST** include an `Example` field with usage examples. Examples improve help documentation and are automatically validated to ensure they stay accurate as the code evolves.

**Example Format:**
```go
func NewExampleCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "example [arg]",
        Short: "An example command",
        Example: `# Basic usage with comment
kortex-cli example

# With argument
kortex-cli example value

# With flag
kortex-cli example --flag value`,
        Args: cobra.MaximumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            // Command logic here
        },
    }
}
```

**Example Guidelines:**
- Use comments (lines starting with `#`) to describe what each example does
- Show the most common use cases (typically 3-5 examples)
- Include examples for all important flags
- Examples must use the actual binary name (`kortex-cli`)
- All commands and flags in examples must exist
- Keep examples concise and realistic

**Validating Examples:**

Every command with an `Example` field **MUST** have a corresponding validation test:

```go
func Test<Command>Cmd_Examples(t *testing.T) {
    t.Parallel()

    // Get the command
    cmd := New<Command>Cmd()

    // Verify Example field is not empty
    if cmd.Example == "" {
        t.Fatal("Example field should not be empty")
    }

    // Parse the examples
    commands, err := testutil.ParseExampleCommands(cmd.Example)
    if err != nil {
        t.Fatalf("Failed to parse examples: %v", err)
    }

    // Verify we have the expected number of examples
    expectedCount := 3  // Adjust based on your examples
    if len(commands) != expectedCount {
        t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
    }

    // Validate all examples against the root command
    rootCmd := NewRootCmd()
    err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
    if err != nil {
        t.Errorf("Example validation failed: %v", err)
    }
}
```

**What the validator checks:**
- Binary name is `kortex-cli`
- All commands exist in the command tree
- All flags (both long and short) are valid for the command
- No invalid subcommands are used

**Reference:** See `pkg/cmd/init.go` and `pkg/cmd/init_test.go` for complete examples.

**Alias Commands:**

Alias commands are shortcuts that delegate to existing commands (e.g., `list` as an alias for `workspace list`). For alias commands:

1. **Inherit the Example field** from the original command
2. **Adapt the examples** to show the alias syntax instead of the full command
3. **Do NOT create validation tests** for aliases (they use the same validation as the original command)

Use the `AdaptExampleForAlias()` helper function (from `pkg/cmd/helpers.go`) to automatically replace the command name in examples while preserving comments:

```go
func NewListCmd() *cobra.Command {
    // Create the workspace list command
    workspaceListCmd := NewWorkspaceListCmd()

    // Create an alias command that delegates to workspace list
    cmd := &cobra.Command{
        Use:     "list",
        Short:   workspaceListCmd.Short,
        Long:    workspaceListCmd.Long,
        Example: AdaptExampleForAlias(workspaceListCmd.Example, "workspace list", "list"),
        Args:    workspaceListCmd.Args,
        PreRunE: workspaceListCmd.PreRunE,
        RunE:    workspaceListCmd.RunE,
    }

    // Copy flags from workspace list command
    cmd.Flags().AddFlagSet(workspaceListCmd.Flags())

    return cmd
}
```

The `AdaptExampleForAlias()` function:
- Replaces the original command with the alias **only in command lines** (lines starting with `kortex-cli`)
- **Preserves comments unchanged** (lines starting with `#`)
- Maintains formatting and indentation

**Example transformation:**
```bash
# Original (from workspace list):
# List all workspaces
kortex-cli workspace list

# List in JSON format
kortex-cli workspace list --output json

# After AdaptExampleForAlias(..., "workspace list", "list"):
# List all workspaces
kortex-cli list

# List in JSON format
kortex-cli list --output json
```

**Reference:** See `pkg/cmd/list.go` and `pkg/cmd/remove.go` for complete alias examples.

### Command Implementation Pattern

Commands should follow a consistent structure for maintainability and testability:

1. **Command Struct** - Contains all command state:
   - Input values from flags/args
   - Computed/validated values
   - Dependencies (e.g., manager instances)

2. **preRun Method** - Validates parameters and prepares:
   - Parse and validate arguments/flags
   - Access global flags (e.g., `--storage`)
   - Create dependencies (managers, etc.)
   - Convert paths to absolute using `filepath.Abs()`
   - Store validated values in struct fields

3. **run Method** - Executes the command logic:
   - Use validated values from struct fields
   - Perform the actual operation
   - Output results to user

**Reference:** See `pkg/cmd/init.go` for a complete implementation of this pattern.

### Flag Binding Best Practices

**IMPORTANT**: Always bind command flags directly to struct fields using the `*Var` variants (e.g., `StringVarP`, `BoolVarP`, `IntVarP`) instead of using the non-binding variants and then calling `GetString()`, `GetBool()`, etc. in `preRun`.

**Benefits:**
- **Cleaner code**: No need to call `cmd.Flags().GetString()` and handle errors
- **Simpler testing**: Tests can set struct fields directly instead of creating and setting flags
- **Earlier binding**: Values are available immediately when preRun is called
- **Less error-prone**: No risk of typos in flag names when retrieving values

**Pattern:**

```go
// Command struct with fields for all flags
type myCmd struct {
    verbose bool
    output  string
    count   int
    manager instances.Manager
}

// Bind flags to struct fields in the command factory
func NewMyCmd() *cobra.Command {
    c := &myCmd{}

    cmd := &cobra.Command{
        Use:     "my-command",
        Short:   "My command description",
        Args:    cobra.NoArgs,
        PreRunE: c.preRun,
        RunE:    c.run,
    }

    // GOOD: Bind flags directly to struct fields
    cmd.Flags().BoolVarP(&c.verbose, "verbose", "v", false, "Show detailed output")
    cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
    cmd.Flags().IntVarP(&c.count, "count", "c", 10, "Number of items to process")

    return cmd
}

// Use the bound values directly in preRun
func (m *myCmd) preRun(cmd *cobra.Command, args []string) error {
    // Values are already available from struct fields
    if m.output != "" && m.output != "json" {
        return fmt.Errorf("unsupported output format: %s", m.output)
    }

    // No need to call cmd.Flags().GetString("output")
    return nil
}
```

**Avoid:**

```go
// BAD: Don't define flags without binding
cmd.Flags().StringP("output", "o", "", "Output format")

// BAD: Don't retrieve flag values in preRun
func (m *myCmd) preRun(cmd *cobra.Command, args []string) error {
    output, err := cmd.Flags().GetString("output")  // Avoid this pattern
    if err != nil {
        return err
    }
    m.output = output
    // ...
}
```

**Testing with Bound Flags:**

```go
func TestMyCmd_PreRun(t *testing.T) {
    t.Run("validates output format", func(t *testing.T) {
        // Set struct fields directly - no need to set up flags
        c := &myCmd{
            output: "xml",  // Invalid format
        }
        cmd := &cobra.Command{}

        err := c.preRun(cmd, []string{})
        if err == nil {
            t.Fatal("Expected error for invalid output format")
        }
    })
}
```

**Reference:** See `pkg/cmd/init.go`, `pkg/cmd/workspace_remove.go`, and `pkg/cmd/workspace_list.go` for examples of proper flag binding.

### JSON Output Support Pattern

When adding JSON output support to commands, follow this pattern to ensure consistent error handling and output formatting:

**Rules:**

1. **Check output flag FIRST in preRun** - Validate the output format before any other validation
2. **Set SilenceErrors and SilenceUsage early** - Prevent Cobra's default error output when in JSON mode
3. **Use outputErrorIfJSON for ALL errors** - In preRun, run, and any helper methods (like outputJSON)

**Pattern:**

```go
type myCmd struct {
    output  string  // Bound to --output flag
    manager instances.Manager
}

func (m *myCmd) preRun(cmd *cobra.Command, args []string) error {
    // 1. FIRST: Validate output format
    if m.output != "" && m.output != "json" {
        return fmt.Errorf("unsupported output format: %s (supported: json)", m.output)
    }

    // 2. EARLY: Silence Cobra's error output in JSON mode
    if m.output == "json" {
        cmd.SilenceErrors = true
        cmd.SilenceUsage = true
    }

    // 3. ALL subsequent errors use outputErrorIfJSON
    storageDir, err := cmd.Flags().GetString("storage")
    if err != nil {
        return outputErrorIfJSON(cmd, m.output, fmt.Errorf("failed to read --storage flag: %w", err))
    }

    manager, err := instances.NewManager(storageDir)
    if err != nil {
        return outputErrorIfJSON(cmd, m.output, fmt.Errorf("failed to create manager: %w", err))
    }
    m.manager = manager

    return nil
}

func (m *myCmd) run(cmd *cobra.Command, args []string) error {
    // ALL errors in run use outputErrorIfJSON
    data, err := m.manager.GetData()
    if err != nil {
        return outputErrorIfJSON(cmd, m.output, fmt.Errorf("failed to get data: %w", err))
    }

    if m.output == "json" {
        return m.outputJSON(cmd, data)
    }

    // Text output
    cmd.Println(data)
    return nil
}

func (m *myCmd) outputJSON(cmd *cobra.Command, data interface{}) error {
    jsonData, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        // Even unlikely errors in helper methods use outputErrorIfJSON
        return outputErrorIfJSON(cmd, m.output, fmt.Errorf("failed to marshal to JSON: %w", err))
    }

    fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
    return nil
}
```

**Why this pattern:**
- **Consistent error format**: All errors are JSON when `--output=json` is set
- **No Cobra pollution**: SilenceErrors prevents "Error: ..." prefix in JSON output
- **Early detection**: Output flag is validated before expensive operations
- **Helper methods work**: outputErrorIfJSON works in any method that has access to cmd and output flag

**Helper function:**

The `outputErrorIfJSON` helper in `pkg/cmd/conversion.go` handles the formatting:

```go
func outputErrorIfJSON(cmd interface{ OutOrStdout() io.Writer }, output string, err error) error {
    if output == "json" {
        jsonErr, _ := formatErrorJSON(err)
        fmt.Fprintln(cmd.OutOrStdout(), jsonErr)  // Errors go to stdout in JSON mode
    }
    return err  // Still return the error for proper exit codes
}
```

**Reference:** See `pkg/cmd/init.go`, `pkg/cmd/workspace_remove.go`, and `pkg/cmd/workspace_list.go` for complete implementations.

### Testing Pattern for Commands

Commands should have two types of tests following the pattern in `pkg/cmd/init_test.go`:

1. **Unit Tests** - Test the `preRun` method directly by calling it on the command struct:
   - **IMPORTANT**: Create an instance of the command struct (e.g., `c := &initCmd{}`)
   - **IMPORTANT**: Create a mock `*cobra.Command` and set up required flags
   - **IMPORTANT**: Call `c.preRun(cmd, args)` directly - DO NOT call `rootCmd.Execute()`
   - Use `t.Run()` for subtests within a parent test function
   - Test with different argument/flag combinations
   - Verify struct fields are set correctly after `preRun()` executes
   - Use `t.TempDir()` for temporary directories (automatic cleanup)

   **Example:**
   ```go
   func TestMyCmd_PreRun(t *testing.T) {
       t.Run("sets fields correctly", func(t *testing.T) {
           t.Parallel()

           storageDir := t.TempDir()

           c := &myCmd{}  // Create command struct instance
           cmd := &cobra.Command{}  // Create mock cobra command
           cmd.Flags().String("storage", storageDir, "test storage flag")

           args := []string{"arg1"}

           err := c.preRun(cmd, args)  // Call preRun directly
           if err != nil {
               t.Fatalf("preRun() failed: %v", err)
           }

           // Assert on struct fields
           if c.manager == nil {
               t.Error("Expected manager to be created")
           }
       })
   }
   ```

2. **E2E Tests** - Test the full command execution including Cobra wiring:
   - Execute via `rootCmd.Execute()` to test the complete flow
   - Use real temp directories with `t.TempDir()`
   - Verify output messages
   - Verify persistence (check storage/database)
   - Verify all field values from `manager.List()` or similar
   - Test multiple scenarios (default args, custom args, edge cases)
   - Test Cobra's argument validation (e.g., required args, arg counts)

   **Example:**
   ```go
   func TestMyCmd_E2E(t *testing.T) {
       t.Run("executes successfully", func(t *testing.T) {
           t.Parallel()

           storageDir := t.TempDir()

           rootCmd := NewRootCmd()  // Use full command construction
           rootCmd.SetArgs([]string{"mycommand", "arg1", "--storage", storageDir})

           err := rootCmd.Execute()  // Execute the full command
           if err != nil {
               t.Fatalf("Execute() failed: %v", err)
           }

           // Verify results in storage
           manager, _ := instances.NewManager(storageDir)
           instancesList, _ := manager.List()
           // ... assert on results
       })
   }
   ```

**Reference:** See `pkg/cmd/init_test.go`, `pkg/cmd/workspace_list_test.go`, and `pkg/cmd/workspace_remove_test.go` for complete examples of both `preRun` unit tests (in `Test*Cmd_PreRun`) and E2E tests (in `Test*Cmd_E2E`).

### Working with the Instances Manager

When commands need to interact with workspaces:

```go
// In preRun - create manager from storage flag
storageDir, _ := cmd.Flags().GetString("storage")
manager, err := instances.NewManager(storageDir)
if err != nil {
    return fmt.Errorf("failed to create manager: %w", err)
}

// In run - use manager to add instances
instance, err := instances.NewInstance(sourceDir, configDir)
if err != nil {
    return fmt.Errorf("failed to create instance: %w", err)
}

addedInstance, err := manager.Add(instance)
if err != nil {
    return fmt.Errorf("failed to add instance: %w", err)
}

// List instances
instancesList, err := manager.List()
if err != nil {
    return fmt.Errorf("failed to list instances: %w", err)
}

// Get specific instance
instance, err := manager.Get(id)
if err != nil {
    return fmt.Errorf("instance not found: %w", err)
}

// Delete instance
err := manager.Delete(id)
if err != nil {
    return fmt.Errorf("failed to delete instance: %w", err)
}
```

### Cross-Platform Path Handling

⚠️ **CRITICAL**: All path operations and tests MUST be cross-platform compatible (Linux, macOS, Windows).

**Rules:**
- Always use `filepath.Join()` for path construction (never hardcode "/" or "\\")
- Convert relative paths to absolute with `filepath.Abs()`
- Never hardcode paths with `~` - use `os.UserHomeDir()` instead
- In tests, use `filepath.Join()` for all path assertions
- **Use `t.TempDir()` for ALL temporary directories in tests - never hardcode paths**

#### Common Test Failures on Windows

Tests often fail on Windows due to hardcoded Unix-style paths. These paths get normalized differently by `filepath.Abs()` on Windows vs Unix systems.

**❌ NEVER do this in tests:**
```go
// BAD - Will fail on Windows because filepath.Abs() normalizes differently
instance, err := instances.NewInstance(instances.NewInstanceParams{
    SourceDir: "/path/to/source",      // ❌ Hardcoded Unix path
    ConfigDir: "/path/to/config",      // ❌ Hardcoded Unix path
})

// BAD - Will fail on Windows
invalidPath := "/this/path/does/not/exist"  // ❌ Unix-style absolute path

// BAD - Platform-specific separator
path := dir + "/subdir"  // ❌ Hardcoded forward slash
```

**✅ ALWAYS do this in tests:**
```go
// GOOD - Cross-platform, works everywhere
sourceDir := t.TempDir()
configDir := t.TempDir()
instance, err := instances.NewInstance(instances.NewInstanceParams{
    SourceDir: sourceDir,  // ✅ Real temp directory
    ConfigDir: configDir,  // ✅ Real temp directory
})

// GOOD - Create invalid path cross-platform way
tempDir := t.TempDir()
notADir := filepath.Join(tempDir, "file")
os.WriteFile(notADir, []byte("test"), 0644)
invalidPath := filepath.Join(notADir, "subdir")  // ✅ Will fail MkdirAll on all platforms

// GOOD - Use filepath.Join()
path := filepath.Join(dir, "subdir")  // ✅ Cross-platform
```

#### Examples for Production Code

```go
// GOOD: Cross-platform path construction
configDir := filepath.Join(sourceDir, ".kortex")
absPath, err := filepath.Abs(relativePath)

// BAD: Hardcoded separator
configDir := sourceDir + "/.kortex"  // Don't do this!

// GOOD: User home directory
homeDir, err := os.UserHomeDir()
defaultPath := filepath.Join(homeDir, ".kortex-cli")

// BAD: Hardcoded tilde
defaultPath := "~/.kortex-cli"  // Don't do this!

// GOOD: Test assertions
expectedPath := filepath.Join(".", "relative", "path")
if result != expectedPath {
    t.Errorf("Expected %s, got %s", expectedPath, result)
}

// GOOD: Temporary directories in tests
tempDir := t.TempDir()  // Automatic cleanup
sourcesDir := t.TempDir()
```

**Why this matters:** Tests that pass on Linux/macOS may fail on Windows CI if they use hardcoded Unix paths. Always use `t.TempDir()` and `filepath.Join()` to ensure tests work on all platforms.

## Documentation Standards

### Markdown Best Practices

All markdown files (*.md) in this repository must follow these standards:

**Fenced Code Blocks:**
- **ALWAYS** include a language tag in fenced code blocks
- Use the appropriate language identifier (`bash`, `go`, `json`, `yaml`, `text`, etc.)
- For output examples or plain text content, use `text` as the language tag
- This ensures markdown linters (markdownlint MD040) pass and improves syntax highlighting

**Examples:**

````markdown
<!-- CORRECT: Language tag specified -->
```bash
make build
```

```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

<!-- INCORRECT: Missing language tag -->
```
make build
```
````

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
- Go 1.25+

## Testing

Tests follow Go conventions with `*_test.go` files alongside source files. Tests use the standard `testing` package and should cover command initialization, execution, and error cases.

### Parallel Test Execution

**All tests MUST call `t.Parallel()` as the first line of the test function.**

This ensures faster test execution and better resource utilization. Every test function should start with:

```go
func TestExample(t *testing.T) {
    t.Parallel()

    // Test code here...
}
```

**Exception: Tests using `t.Setenv()`**

Tests that use `t.Setenv()` to set environment variables **cannot use `t.Parallel()`** on the parent test function. The Go testing framework enforces this restriction because environment variable changes affect the entire process.

```go
// CORRECT: No t.Parallel() when using t.Setenv()
func TestWithEnvVariable(t *testing.T) {
    t.Run("subtest with env var", func(t *testing.T) {
        t.Setenv("MY_VAR", "value")
        // Test code here...
    })
}

// INCORRECT: Will panic at runtime
func TestWithEnvVariable(t *testing.T) {
    t.Parallel() // ❌ WRONG - cannot use with t.Setenv()

    t.Run("subtest with env var", func(t *testing.T) {
        t.Setenv("MY_VAR", "value")
        // Test code here...
    })
}
```

**Reference:** See `pkg/cmd/root_test.go:TestRootCmd_StorageEnvVariable()` for an example of testing with environment variables.

### Testing with Fake Objects

When testing code that uses interfaces (following the Module Design Pattern), **use fake implementations instead of real implementations or mocks**.

**Pattern:**
1. Create unexported fake structs that implement the interface
2. Use factory injection to provide fakes to the code under test
3. Control fake behavior through constructor parameters or fields

**Benefits:**
- **No external dependencies** - Fakes are simple structs with no framework requirements
- **Full control** - Control exact behavior through fields/parameters
- **Type-safe** - Compile-time verification that fakes implement interfaces
- **Easy to understand** - Fakes are just plain Go code
- **Flexible** - Can create different factories for different test scenarios

**Reference:** See `pkg/instances/manager_test.go` for a complete implementation of this pattern with factory injection.

## GitHub Actions

GitHub Actions workflows are stored in `.github/workflows/`. All workflows must use commit SHA1 hashes instead of version tags for security reasons (to prevent supply chain attacks from tag manipulation).

Example:
```yaml
- uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
```

Always include the version as a comment for readability.
