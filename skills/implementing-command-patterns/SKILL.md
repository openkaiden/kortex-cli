---
name: implementing-command-patterns
description: Advanced patterns for implementing commands including flag binding, JSON output, and interactive sessions
argument-hint: ""
---

# Implementing Command Patterns

This skill covers advanced patterns for implementing CLI commands beyond the basics. For creating simple commands, see `/add-command-simple` and `/add-command-with-json` skills.

## Command Implementation Pattern

Commands should follow a consistent structure for maintainability and testability:

### 1. Command Struct

Contains all command state:
- Input values from flags/args
- Computed/validated values
- Dependencies (e.g., manager instances)

### 2. preRun Method

Validates parameters and prepares:
- Parse and validate arguments/flags
- Access global flags (e.g., `--storage`)
- Create dependencies (managers, etc.)
- Convert paths to absolute using `filepath.Abs()`
- Store validated values in struct fields

### 3. run Method

Executes the command logic:
- Use validated values from struct fields
- Perform the actual operation
- Output results to user

**Reference:** See `pkg/cmd/init.go` for a complete implementation of this pattern.

## Flag Binding Best Practices

**IMPORTANT**: Always bind command flags directly to struct fields using the `*Var` variants (e.g., `StringVarP`, `BoolVarP`, `IntVarP`) instead of using the non-binding variants and then calling `GetString()`, `GetBool()`, etc. in `preRun`.

### Benefits

- **Cleaner code**: No need to call `cmd.Flags().GetString()` and handle errors
- **Simpler testing**: Tests can set struct fields directly instead of creating and setting flags
- **Earlier binding**: Values are available immediately when preRun is called
- **Less error-prone**: No risk of typos in flag names when retrieving values

### Pattern

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

### Avoid

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

### Testing with Bound Flags

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

## Environment Variable Fallback Pattern

Commands can support environment variables as fallbacks for flags, allowing users to set defaults without specifying flags every time.

### Rules

1. **Flags always take precedence** over environment variables
2. **Check the struct field value** - If empty/false, then check environment variable
3. **Parse environment variable** after checking if the field value is not set
4. **Document priority** in command help text and examples

### Pattern

```go
type myCmd struct {
    runtime string  // Bound to --runtime flag
    agent   string  // Bound to --agent flag
    start   bool    // Bound to --start flag
}

func (m *myCmd) preRun(cmd *cobra.Command, args []string) error {
    // String flag: check if empty, then try environment variable
    if m.runtime == "" {
        if envRuntime := os.Getenv("KORTEX_CLI_DEFAULT_RUNTIME"); envRuntime != "" {
            m.runtime = envRuntime
        } else {
            return fmt.Errorf("runtime is required: use --runtime flag or set KORTEX_CLI_DEFAULT_RUNTIME environment variable")
        }
    }

    // Boolean flag: check if false, then try environment variable
    if !m.start {
        if envStart := os.Getenv("KORTEX_CLI_INIT_AUTO_START"); envStart != "" {
            // Parse truthy values
            switch envStart {
            case "1", "true", "True", "TRUE", "yes", "Yes", "YES":
                m.start = true
            }
        }
    }

    return nil
}

func NewMyCmd() *cobra.Command {
    c := &myCmd{}

    cmd := &cobra.Command{
        Use:   "my-command",
        Short: "My command description",
        PreRunE: c.preRun,
        RunE:    c.run,
    }

    // Bind flags with helpful descriptions mentioning environment variable fallback
    cmd.Flags().StringVarP(&c.runtime, "runtime", "r", "", "Runtime to use (or set KORTEX_CLI_DEFAULT_RUNTIME)")
    cmd.Flags().StringVarP(&c.agent, "agent", "a", "", "Agent to use (or set KORTEX_CLI_DEFAULT_AGENT)")
    cmd.Flags().BoolVar(&c.start, "start", false, "Auto-start (or set KORTEX_CLI_INIT_AUTO_START)")

    return cmd
}
```

### Environment Variable Parsing

**String values:**
- Simple: just use the environment variable value directly
- Empty string means not set

**Boolean values:**
- Check if field is `false`, then parse environment variable
- Parse truthy values: `"1"`, `"true"`, `"True"`, `"TRUE"`, `"yes"`, `"Yes"`, `"YES"`
- All other values (including `"0"`, `"false"`, `"no"`, `""`) are falsy
- If flag is explicitly set to `true`, the field is already `true`, so environment variable is never checked

### Testing Environment Variables

```go
func TestMyCmd_PreRun(t *testing.T) {
    t.Run("uses environment variable when flag not set", func(t *testing.T) {
        // Note: Cannot use t.Parallel() when using t.Setenv()
        t.Setenv("KORTEX_CLI_DEFAULT_RUNTIME", "podman")

        c := &myCmd{}
        cmd := &cobra.Command{}
        cmd.Flags().String("runtime", "", "test flag")

        err := c.preRun(cmd, []string{})
        if err != nil {
            t.Fatalf("preRun() failed: %v", err)
        }

        if c.runtime != "podman" {
            t.Errorf("Expected runtime to be 'podman' from env var, got: %s", c.runtime)
        }
    })

    t.Run("flag takes precedence over environment variable", func(t *testing.T) {
        // Note: Cannot use t.Parallel() when using t.Setenv()
        t.Setenv("KORTEX_CLI_DEFAULT_RUNTIME", "fake")

        c := &myCmd{runtime: "podman"}  // Set via flag
        cmd := &cobra.Command{}
        cmd.Flags().String("runtime", "", "test flag")
        cmd.Flags().Set("runtime", "podman")

        err := c.preRun(cmd, []string{})
        if err != nil {
            t.Fatalf("preRun() failed: %v", err)
        }

        if c.runtime != "podman" {
            t.Errorf("Expected runtime to be 'podman' from flag, got: %s", c.runtime)
        }
    })

    // Table-driven test for boolean environment variable values
    t.Run("parses boolean environment variable", func(t *testing.T) {
        tests := []struct {
            name     string
            envValue string
            expected bool
        }{
            {"1 is truthy", "1", true},
            {"true is truthy", "true", true},
            {"True is truthy", "True", true},
            {"yes is truthy", "yes", true},
            {"0 is falsy", "0", false},
            {"false is falsy", "false", false},
            {"empty is falsy", "", false},
        }

        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                t.Setenv("KORTEX_CLI_INIT_AUTO_START", tt.envValue)

                c := &myCmd{}
                cmd := &cobra.Command{}
                cmd.Flags().Bool("start", false, "test flag")

                err := c.preRun(cmd, []string{})
                if err != nil {
                    t.Fatalf("preRun() failed: %v", err)
                }

                if c.start != tt.expected {
                    t.Errorf("Expected start to be %v, got %v", tt.expected, c.start)
                }
            })
        }
    })
}
```

**Reference:** See `pkg/cmd/init.go` for a complete implementation with `KORTEX_CLI_DEFAULT_RUNTIME`, `KORTEX_CLI_DEFAULT_AGENT`, and `KORTEX_CLI_INIT_AUTO_START`.

## JSON Output Support Pattern

When adding JSON output support to commands, follow this pattern to ensure consistent error handling and output formatting.

### Rules

1. **Check output flag FIRST in preRun** - Validate the output format before any other validation
2. **Set SilenceErrors early** - Prevent Cobra's default error output when in JSON mode
3. **Use outputErrorIfJSON for ALL errors** - In preRun, run, and any helper methods (like outputJSON)

### Pattern

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

### Why This Pattern

- **Consistent error format**: All errors are JSON when `--output=json` is set
- **No Cobra pollution**: SilenceErrors prevents "Error: ..." prefix in JSON output
- **Early detection**: Output flag is validated before expensive operations
- **Helper methods work**: outputErrorIfJSON works in any method that has access to cmd and output flag

### Helper Function

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

## The --show-logs Flag Pattern

Commands that trigger runtime CLI execution (e.g., `podman build`, `podman start`) should expose a `--show-logs` flag to let users see the raw stdout/stderr from those commands.

### Rules

1. Add `showLogs bool` to the command struct bound to `--show-logs`
2. In `preRun`, reject the combination of `--show-logs` and `--output json`
3. In `run`, create a `logger.Logger` from the flag value and inject it into context
4. (Optional) For progress feedback, also create a `steplogger.StepLogger` as shown in `/working-with-steplogger`
### Pattern

```go
import (
    "github.com/kortex-hub/kortex-cli/pkg/logger"
    "github.com/kortex-hub/kortex-cli/pkg/steplogger"
)

type myCmd struct {
    output   string
    showLogs bool
    manager  instances.Manager
}

func (m *myCmd) preRun(cmd *cobra.Command, args []string) error {
    if m.output != "" && m.output != "json" {
        return fmt.Errorf("unsupported output format: %s (supported: json)", m.output)
    }
    if m.showLogs && m.output == "json" {
        return fmt.Errorf("--show-logs cannot be used with --output json")
    }
    // ... rest of preRun
}

func (m *myCmd) run(cmd *cobra.Command, args []string) error {
    var l logger.Logger
    if m.showLogs {
        l = logger.NewTextLogger(cmd.OutOrStdout(), cmd.ErrOrStderr())
    } else {
        l = logger.NewNoOpLogger()
    }
    ctx := logger.WithLogger(cmd.Context(), l)

    return m.manager.DoSomething(ctx)
}

func NewMyCmd() *cobra.Command {
    c := &myCmd{}
    cmd := &cobra.Command{ /* ... */ }

    cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")
    cmd.Flags().BoolVar(&c.showLogs, "show-logs", false, "Show stdout and stderr from runtime commands")

    cmd.RegisterFlagCompletionFunc("output", newOutputFlagCompletion([]string{"json"}))
    return cmd
}
```

**Note:** Register all `RegisterFlagCompletionFunc` calls after all flag definitions.

**Reference:** See `pkg/cmd/workspace_start.go`, `pkg/cmd/workspace_stop.go`, `pkg/cmd/workspace_remove.go`, and `pkg/cmd/init.go`.

## Interactive Commands (No JSON Output)

Some commands are inherently interactive and do not support JSON output. These commands connect stdin/stdout/stderr directly to a user's terminal.

### Example: Terminal Command

The `terminal` command provides an interactive session with a running workspace instance:

```go
type workspaceTerminalCmd struct {
    manager  instances.Manager
    nameOrID string
    command  []string
}

func (w *workspaceTerminalCmd) preRun(cmd *cobra.Command, args []string) error {
    w.nameOrID = args[0]

    // Extract command from args[1:] if provided
    if len(args) > 1 {
        w.command = args[1:]
    } else {
        // Default command (configurable from runtime)
        w.command = []string{"claude"}
    }

    // Standard setup: storage flag, manager, runtime registration
    storageDir, err := cmd.Flags().GetString("storage")
    if err != nil {
        return fmt.Errorf("failed to read --storage flag: %w", err)
    }

    absStorageDir, err := filepath.Abs(storageDir)
    if err != nil {
        return fmt.Errorf("failed to resolve storage path: %w", err)
    }

    manager, err := instances.NewManager(absStorageDir)
    if err != nil {
        return fmt.Errorf("failed to create manager: %w", err)
    }

    if err := runtimesetup.RegisterAll(manager); err != nil {
        return fmt.Errorf("failed to register runtimes: %w", err)
    }

    w.manager = manager
    return nil
}

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

    // Connect to terminal - this is a blocking interactive call
    err = w.manager.Terminal(cmd.Context(), instanceID, w.command)
    if err != nil {
        return err
    }
    return nil
}
```

### Key Differences from JSON-Supporting Commands

- **No `--output` flag** - Interactive commands don't need this
- **No JSON output helpers** - All output goes directly to terminal
- **Simpler error handling** - Just return errors normally (no `outputErrorIfJSON`)
- **Blocking execution** - The command runs until the user exits the interactive session
- **Command arguments** - Accept commands to run inside the instance: `terminal ID bash` or `terminal ID -- bash -c 'echo hello'`

### Example Command Registration

```go
func NewWorkspaceTerminalCmd() *cobra.Command {
    c := &workspaceTerminalCmd{}

    cmd := &cobra.Command{
        Use:     "terminal NAME|ID [COMMAND...]",
        Short:   "Connect to a running workspace with an interactive terminal",
        Args:    cobra.MinimumNArgs(1),
        ValidArgsFunction: completeRunningWorkspaceID,  // Shows running workspace IDs and names
        PreRunE: c.preRun,
        RunE:    c.run,
    }

    // No flags needed - just uses global --storage flag
    return cmd
}
```

**Reference:** See `pkg/cmd/workspace_terminal.go` for the complete implementation.

## Related Skills

- `/add-command-simple` - Creating simple commands
- `/add-command-with-json` - Creating commands with JSON output
- `/testing-commands` - Testing command implementations
- `/working-with-steplogger` - Adding progress feedback to commands

## References

- **Command Pattern Example**: `pkg/cmd/init.go`
- **Flag Binding Examples**: `pkg/cmd/init.go`, `pkg/cmd/workspace_remove.go`, `pkg/cmd/workspace_list.go`
- **JSON Output Examples**: `pkg/cmd/init.go`, `pkg/cmd/workspace_remove.go`, `pkg/cmd/workspace_list.go`
- **Interactive Command Example**: `pkg/cmd/workspace_terminal.go`
- **Helper Functions**: `pkg/cmd/conversion.go`
