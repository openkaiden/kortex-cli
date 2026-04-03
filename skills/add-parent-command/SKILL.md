---
name: add-parent-command
description: Add a parent/root command that has subcommands
argument-hint: <parent-command-name>
---

# Add Parent Command

This skill helps you create a parent (root) command that organizes related subcommands under a common namespace (e.g., `workspace` with subcommands `list`, `remove`, `init`).

## Prerequisites

- Parent command name (e.g., "workspace", "config", "skill")
- Understanding of what subcommands will belong under this parent
- At least one subcommand to add

## Implementation Steps

### 1. Create the Parent Command File

Create `pkg/cmd/<parent>.go` with the following structure:

```go
package cmd

import (
    "github.com/spf13/cobra"
)

func New<Parent>Cmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "<parent>",
        Short: "Short description of the parent command category",
        Long:  `Long description explaining what this command category does and what subcommands are available.`,
        Example: `# Show available subcommands
kortex-cli <parent> --help

# Execute a subcommand
kortex-cli <parent> <subcommand>`,
        Args: cobra.NoArgs,  // Parent commands typically don't accept args directly
    }

    // Add subcommands
    cmd.AddCommand(New<Parent><SubCommand1>Cmd())
    cmd.AddCommand(New<Parent><SubCommand2>Cmd())
    cmd.AddCommand(New<Parent><SubCommand3>Cmd())

    return cmd
}
```

**Example: Creating the `workspace` parent command**

```go
package cmd

import (
    "github.com/spf13/cobra"
)

func NewWorkspaceCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "workspace",
        Short: "Manage workspace instances",
        Long: `The workspace command provides subcommands for managing workspace instances.

Use these commands to initialize, list, and remove workspace configurations.`,
        Example: `# Show available workspace subcommands
kortex-cli workspace --help

# List all workspaces
kortex-cli workspace list

# Remove a workspace
kortex-cli workspace remove <id>`,
        Args: cobra.NoArgs,
    }

    // Add subcommands
    cmd.AddCommand(NewWorkspaceListCmd())
    cmd.AddCommand(NewWorkspaceRemoveCmd())
    cmd.AddCommand(NewWorkspaceInitCmd())

    return cmd
}
```

### 2. Create Subcommands

For each subcommand, create a separate file following the command patterns:
- Use `add-command-with-json` skill for subcommands with JSON output
- Use `add-command-simple` skill for subcommands without JSON output

**File naming convention:**
- `pkg/cmd/<parent>_<subcommand>.go` for each subcommand
- Factory function: `New<Parent><SubCommand>Cmd()`

**Example subcommand structure:**

```go
// File: pkg/cmd/workspace_list.go
package cmd

import (
    "github.com/spf13/cobra"
)

func NewWorkspaceListCmd() *cobra.Command {
    c := &workspaceListCmd{}

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List all workspaces",
        Long:  `List all initialized workspace instances with their details.`,
        Example: `# List all workspaces
kortex-cli workspace list

# List with JSON output
kortex-cli workspace list --output json`,
        Args:    cobra.NoArgs,
        PreRunE: c.preRun,
        RunE:    c.run,
    }

    cmd.Flags().StringVarP(&c.output, "output", "o", "", "Output format (supported: json)")

    return cmd
}
```

### 3. Register the Parent Command

In `pkg/cmd/root.go`, add to the `NewRootCmd()` function:

```go
rootCmd.AddCommand(New<Parent>Cmd())
```

**Optional: Assign to a command group**

If you want the parent command to appear in a specific group in the help output:

```go
// Define a group if it doesn't exist yet
rootCmd.AddGroup(&cobra.Group{
    ID:    "mygroup",
    Title: "My Command Group:",
})

// Create and assign parent command to the group
parentCmd := New<Parent>Cmd()
parentCmd.GroupID = "mygroup"
rootCmd.AddCommand(parentCmd)
```

Existing groups:
- `main` - Main Commands (for commonly used workspace operations)
- `workspace` - Workspace Commands (for the workspace parent command)
- Commands without a group appear under "Additional Commands"

### 4. Create Tests

Create `pkg/cmd/<parent>_test.go`:

```go
package cmd

import (
    "testing"

    "github.com/kortex-hub/kortex-cli/pkg/cmd/testutil"
)

func Test<Parent>Cmd_Structure(t *testing.T) {
    t.Parallel()

    cmd := New<Parent>Cmd()

    // Verify the command has subcommands
    if !cmd.HasSubCommands() {
        t.Fatal("Expected parent command to have subcommands")
    }

    // Verify specific subcommands exist
    expectedSubcommands := []string{"<subcommand1>", "<subcommand2>", "<subcommand3>"}
    for _, name := range expectedSubcommands {
        if _, _, err := cmd.Find([]string{name}); err != nil {
            t.Errorf("Expected subcommand '%s' to exist, but not found", name)
        }
    }
}

func Test<Parent>Cmd_NoArgs(t *testing.T) {
    t.Parallel()

    cmd := New<Parent>Cmd()

    // Verify Args validator
    if cmd.Args == nil {
        t.Error("Expected Args validator to be set")
    }

    // Verify it accepts no args
    err := cmd.Args(cmd, []string{"unexpected-arg"})
    if err == nil {
        t.Error("Expected error when passing arguments to parent command")
    }
}

func Test<Parent>Cmd_Examples(t *testing.T) {
    t.Parallel()

    cmd := New<Parent>Cmd()

    if cmd.Example == "" {
        t.Fatal("Example field should not be empty")
    }

    commands, err := testutil.ParseExampleCommands(cmd.Example)
    if err != nil {
        t.Fatalf("Failed to parse examples: %v", err)
    }

    // Parent commands typically have 2-3 examples
    expectedCount := 2
    if len(commands) != expectedCount {
        t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
    }

    rootCmd := NewRootCmd()
    err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
    if err != nil {
        t.Errorf("Example validation failed: %v", err)
    }
}
```

### 5. Run Tests

```bash
# Run tests for the parent command
go test ./pkg/cmd -run Test<Parent>

# Run all tests
make test
```

### 6. Update Documentation

Update relevant documentation to describe the parent command and its subcommands.

## Key Points

- **No Direct Execution**: Parent commands typically don't have RunE/Run - they just organize subcommands
- **Args Validation**: Always set `Args: cobra.NoArgs` since parent commands don't accept arguments directly
- **Subcommand Organization**: Use `cmd.AddCommand()` to register all subcommands
- **Help Text**: Provide clear Short and Long descriptions explaining the purpose of the command group
- **Examples**: Show how to use --help and demonstrate key subcommands
- **Testing**: Verify subcommand structure and examples

## Parent Command Patterns

### Simple Parent (No Logic)

```go
func NewParentCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "parent",
        Short: "Parent command category",
        Args:  cobra.NoArgs,
    }

    cmd.AddCommand(NewParentSubCmd1())
    cmd.AddCommand(NewParentSubCmd2())

    return cmd
}
```

### Parent with Persistent Flags

If you want flags available to all subcommands:

```go
func NewParentCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "parent",
        Short: "Parent command category",
        Args:  cobra.NoArgs,
    }

    // Persistent flags are inherited by subcommands
    cmd.PersistentFlags().Bool("verbose", false, "Verbose output for all subcommands")

    cmd.AddCommand(NewParentSubCmd1())
    cmd.AddCommand(NewParentSubCmd2())

    return cmd
}
```

### Parent with PreRun Initialization

If subcommands need common setup:

```go
func NewParentCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "parent",
        Short: "Parent command category",
        Args:  cobra.NoArgs,
        PersistentPreRun: func(cmd *cobra.Command, args []string) {
            // Common initialization for all subcommands
            // This runs before any subcommand's PreRun
        },
    }

    cmd.AddCommand(NewParentSubCmd1())
    cmd.AddCommand(NewParentSubCmd2())

    return cmd
}
```

## File Organization

For a parent command named `workspace` with subcommands `list`, `remove`, `init`:

```text
pkg/cmd/
├── workspace.go           # Parent command: NewWorkspaceCmd()
├── workspace_test.go      # Parent command tests
├── workspace_list.go      # Subcommand: NewWorkspaceListCmd()
├── workspace_list_test.go # Subcommand tests
├── workspace_remove.go    # Subcommand: NewWorkspaceRemoveCmd()
├── workspace_remove_test.go
├── workspace_init.go      # Subcommand: NewWorkspaceInitCmd()
└── workspace_init_test.go
```

## Complete Example

**File: `pkg/cmd/workspace.go`**
```go
package cmd

import (
    "github.com/spf13/cobra"
)

func NewWorkspaceCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "workspace",
        Short: "Manage workspace instances",
        Long: `The workspace command provides subcommands for managing workspace instances.

Use these commands to initialize new workspaces, list existing ones, and remove
workspaces that are no longer needed.`,
        Example: `# Show available workspace subcommands
kortex-cli workspace --help

# List all workspaces
kortex-cli workspace list

# Remove a workspace
kortex-cli workspace remove <id>

# Initialize a new workspace
kortex-cli workspace init /path/to/project`,
        Args: cobra.NoArgs,
    }

    // Add subcommands
    cmd.AddCommand(NewWorkspaceListCmd())
    cmd.AddCommand(NewWorkspaceRemoveCmd())
    cmd.AddCommand(NewWorkspaceInitCmd())

    return cmd
}
```

**File: `pkg/cmd/workspace_test.go`**
```go
package cmd

import (
    "testing"

    "github.com/kortex-hub/kortex-cli/pkg/cmd/testutil"
)

func TestWorkspaceCmd_Structure(t *testing.T) {
    t.Parallel()

    cmd := NewWorkspaceCmd()

    if !cmd.HasSubCommands() {
        t.Fatal("Expected workspace command to have subcommands")
    }

    expectedSubcommands := []string{"list", "remove", "init"}
    for _, name := range expectedSubcommands {
        if _, _, err := cmd.Find([]string{name}); err != nil {
            t.Errorf("Expected subcommand '%s' to exist, but not found", name)
        }
    }
}

func TestWorkspaceCmd_NoArgs(t *testing.T) {
    t.Parallel()

    cmd := NewWorkspaceCmd()

    if cmd.Args == nil {
        t.Error("Expected Args validator to be set")
    }

    err := cmd.Args(cmd, []string{"unexpected-arg"})
    if err == nil {
        t.Error("Expected error when passing arguments to workspace command")
    }
}

func TestWorkspaceCmd_Examples(t *testing.T) {
    t.Parallel()

    cmd := NewWorkspaceCmd()

    if cmd.Example == "" {
        t.Fatal("Example field should not be empty")
    }

    commands, err := testutil.ParseExampleCommands(cmd.Example)
    if err != nil {
        t.Fatalf("Failed to parse examples: %v", err)
    }

    expectedCount := 4
    if len(commands) != expectedCount {
        t.Errorf("Expected %d example commands, got %d", expectedCount, len(commands))
    }

    rootCmd := NewRootCmd()
    err = testutil.ValidateCommandExamples(rootCmd, cmd.Example)
    if err != nil {
        t.Errorf("Example validation failed: %v", err)
    }
}
```

## References

- `pkg/cmd/workspace.go` - Complete parent command implementation
- `pkg/cmd/workspace_test.go` - Parent command tests
- CLAUDE.md - Command Structure section
