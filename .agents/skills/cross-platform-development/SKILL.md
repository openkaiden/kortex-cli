---
name: cross-platform-development
description: Essential patterns for cross-platform compatibility including path handling and testing practices
argument-hint: ""
---

# Cross-Platform Development

⚠️ **CRITICAL**: All path operations and tests MUST be cross-platform compatible (Linux, macOS, Windows).

## Overview

This skill covers essential patterns for writing code that works correctly across all supported platforms. Tests that pass on Linux/macOS may fail on Windows CI if they don't follow these patterns.

## Core Rules

- **Host paths**: Always use `filepath.Join()` for path construction (never hardcode "/" or "\\")
- **Container paths**: Always use `path.Join()` for paths inside containers (containers are always Unix/Linux)
- Convert relative paths to absolute with `filepath.Abs()`
- Never hardcode paths with `~` - use `os.UserHomeDir()` instead
- In tests, use `filepath.Join()` for all path assertions
- **Use `t.TempDir()` for ALL temporary directories in tests - never hardcode paths**

## Host Paths vs Container Paths

### Host Paths (Use `filepath.Join`)

Host paths run on the actual operating system (Windows, macOS, or Linux) and must use OS-specific separators:

```go
import "path/filepath"

// GOOD: Host paths use filepath.Join
configDir := filepath.Join(storageDir, ".kaiden")
runtimeDir := filepath.Join(storageDir, "runtimes", "podman")
```

### Container Paths (Use `path.Join`)

Container paths are **always Unix/Linux** regardless of the host OS. Podman containers run Linux, so paths inside containers must use forward slashes:

```go
import "path"  // Note: NOT path/filepath

// GOOD: Container paths use path.Join (always Unix)
containerPath := path.Join("/home", "agent", ".config")
mountPath := path.Join("/workspace", "sources", "pkg")

// BAD: Don't use filepath.Join for container paths
containerPath := filepath.Join("/home", "agent", ".config")  // Wrong on Windows host!
```

**Example from Podman runtime:**

```go
import (
    "path"           // For container paths
    "path/filepath"  // For host paths
)

// Host path (can be Windows, macOS, Linux)
hostConfigDir := filepath.Join(storageDir, "runtimes", "podman", "config")

// Container path (always Linux)
containerWorkspace := path.Join("/workspace", "sources")
containerHome := path.Join("/home", "agent")
```

## Common Test Failures on Windows

Tests often fail on Windows due to hardcoded Unix-style paths. These paths get normalized differently by `filepath.Abs()` on Windows vs Unix systems.

### ❌ NEVER Do This in Tests

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

### ✅ ALWAYS Do This in Tests

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

// GOOD - Use filepath.Join() for host paths
path := filepath.Join(dir, "subdir")  // ✅ Cross-platform
```

### chmod-Based Permission Tests

`os.Chmod` maps to `SetFileAttributes` on Windows, which only sets the read-only file
attribute — it does **not** restrict file creation inside a directory the way Unix ACLs do.
Tests that make a directory read-only to trigger a `WriteFile` error will pass on Linux/macOS
but silently succeed on Windows (no error returned, test fails with "expected error, got nil").

**Guard these tests with a runtime skip:**

```go
if runtime.GOOS == "windows" {
    t.Skip("chmod-based permission tests do not apply on Windows")
}
if os.Getuid() == 0 {
    t.Skip("chmod restrictions do not apply to root")
}
```

The `os.Getuid() == 0` guard is also required: root bypasses Unix permission checks, so the
same test would fail if run as root inside a container. Both guards require `"runtime"` and
`"os"` imports.

**Pattern summary:**

```go
func TestWriteSomething_WriteFileFails(t *testing.T) {
    t.Parallel()

    if runtime.GOOS == "windows" {
        t.Skip("chmod-based permission tests do not apply on Windows")
    }
    if os.Getuid() == 0 {
        t.Skip("chmod restrictions do not apply to root")
    }

    dir := t.TempDir()
    if err := os.Chmod(dir, 0500); err != nil {
        t.Fatalf("setup chmod: %v", err)
    }
    t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

    // ... test that WriteFile returns an error ...
}
```

## Production Code Patterns

### Host Path Construction

```go
import "path/filepath"

// GOOD: Cross-platform host path construction
configDir := filepath.Join(sourceDir, ".kaiden")
absPath, err := filepath.Abs(relativePath)

// BAD: Hardcoded separator
configDir := sourceDir + "/.kaiden"  // Don't do this!
```

### Container Path Construction

```go
import "path"

// GOOD: Container path construction (always Unix)
workspacePath := path.Join("/workspace", "sources")
agentHome := path.Join("/home", "agent", ".config")

// BAD: Using filepath.Join for container paths
workspacePath := filepath.Join("/workspace", "sources")  // Wrong on Windows!
```

### User Home Directory

```go
import "path/filepath"

// GOOD: User home directory (host path)
homeDir, err := os.UserHomeDir()
defaultPath := filepath.Join(homeDir, ".kdn")

// BAD: Hardcoded tilde
defaultPath := "~/.kdn"  // Don't do this!
```

### Test Assertions

```go
import "path/filepath"

// GOOD: Test assertions for host paths
expectedPath := filepath.Join(".", "relative", "path")
if result != expectedPath {
    t.Errorf("Expected %s, got %s", expectedPath, result)
}

// BAD: Hardcoded path
expectedPath := "./relative/path"  // Don't do this!
```

### Temporary Directories in Tests

```go
// GOOD: Temporary directories in tests
tempDir := t.TempDir()  // Automatic cleanup
sourcesDir := t.TempDir()

// BAD: Hardcoded temp paths
tempDir := "/tmp/test"  // Don't do this!
```

## Path Handling Best Practices

### Converting Relative to Absolute

Always convert relative paths to absolute in command `preRun`:

```go
import "path/filepath"

func (c *myCmd) preRun(cmd *cobra.Command, args []string) error {
    relativePath := args[0]

    // Convert to absolute path (host path)
    absPath, err := filepath.Abs(relativePath)
    if err != nil {
        return fmt.Errorf("failed to resolve path: %w", err)
    }

    c.path = absPath
    return nil
}
```

### Building Nested Paths

#### Host Paths

```go
import "path/filepath"

// GOOD: Multiple joins for host paths
baseDir := filepath.Join(storageDir, "runtimes")
runtimeDir := filepath.Join(baseDir, "podman")
configDir := filepath.Join(runtimeDir, "config")

// GOOD: Nested arguments for host paths
configPath := filepath.Join(storageDir, "runtimes", "podman", "config", "image.json")

// BAD: String concatenation
configPath := storageDir + "/runtimes/podman/config/image.json"  // Don't do this!
```

#### Container Paths

```go
import "path"

// GOOD: Container paths (always Unix)
workspaceSources := path.Join("/workspace", "sources", "pkg", "cmd")
agentConfig := path.Join("/home", "agent", ".config", "claude")

// BAD: Using filepath for container paths
workspaceSources := filepath.Join("/workspace", "sources", "pkg", "cmd")  // Wrong!
```

### Checking Path Existence

```go
import "path/filepath"

// GOOD: Cross-platform path checking (host paths)
configPath := filepath.Join(configDir, "workspace.json")
if _, err := os.Stat(configPath); err != nil {
    if os.IsNotExist(err) {
        // File doesn't exist
    }
}
```

## Testing Patterns

### Creating Test Directories

```go
import "path/filepath"

func TestMyFunction(t *testing.T) {
    t.Parallel()

    // Create temp directories - automatically cleaned up
    storageDir := t.TempDir()
    sourcesDir := t.TempDir()
    configDir := t.TempDir()

    // Use in tests (these are host paths)
    result, err := MyFunction(sourcesDir, configDir)
    // ...
}
```

### Testing with Invalid Paths

```go
import "path/filepath"

func TestMyFunction_InvalidPath(t *testing.T) {
    t.Parallel()

    // Create a file (not a directory)
    tempDir := t.TempDir()
    notADir := filepath.Join(tempDir, "file")
    os.WriteFile(notADir, []byte("test"), 0644)

    // Try to use file as a directory
    invalidPath := filepath.Join(notADir, "subdir")

    _, err := MyFunction(invalidPath)
    if err == nil {
        t.Fatal("Expected error for invalid path")
    }
}
```

### Path Assertions in Tests

```go
import "path/filepath"

func TestMyFunction_ReturnsPath(t *testing.T) {
    t.Parallel()

    sourceDir := t.TempDir()

    result, err := MyFunction(sourceDir)
    if err != nil {
        t.Fatalf("MyFunction() failed: %v", err)
    }

    // Build expected path using filepath.Join (host path)
    expectedPath := filepath.Join(sourceDir, ".kaiden")
    if result != expectedPath {
        t.Errorf("Expected %s, got %s", expectedPath, result)
    }
}
```

## Common Pitfalls

### Hardcoded Separators

```go
// BAD: Platform-specific
path := basedir + "/config/file.json"
path := basedir + "\\config\\file.json"

// GOOD: Cross-platform (host paths)
path := filepath.Join(basedir, "config", "file.json")

// GOOD: Container paths (always Unix)
containerPath := path.Join("/home", "agent", "config", "file.json")
```

### Home Directory Expansion

```go
import "path/filepath"

// BAD: Tilde doesn't work cross-platform
defaultPath := "~/.kdn"

// GOOD: Use os.UserHomeDir()
homeDir, err := os.UserHomeDir()
if err != nil {
    return "", err
}
defaultPath := filepath.Join(homeDir, ".kdn")
```

### Absolute Path Detection

```go
import "path/filepath"

// GOOD: Use filepath.IsAbs() for host paths
if filepath.IsAbs(path) {
    // Path is absolute
}

// BAD: String prefix checking
if strings.HasPrefix(path, "/") {  // Only works on Unix
    // Path is absolute
}
```

### Path Cleaning

```go
import "path/filepath"

// GOOD: Use filepath.Clean() to normalize host paths
cleanPath := filepath.Clean(userPath)

// Removes redundant separators and . elements
// Resolves .. elements
// Converts separators to OS-specific
```

## Summary: When to Use Which Package

| Context | Package | Example |
|---------|---------|---------|
| Host paths (files on disk) | `path/filepath` | `filepath.Join(storageDir, "config")` |
| Container paths (inside Podman) | `path` | `path.Join("/workspace", "sources")` |
| URL paths | `path` | `path.Join("/api", "v1", "users")` |

## Environment Variables

Handle environment variables in a cross-platform way:

```go
// GOOD: Use os.Getenv()
homeDir := os.Getenv("HOME")  // Unix
if homeDir == "" {
    homeDir = os.Getenv("USERPROFILE")  // Windows
}

// BETTER: Use os.UserHomeDir() which handles this
homeDir, err := os.UserHomeDir()
```

## File Permissions

Be aware of cross-platform permission differences:

```go
// Create directories with appropriate permissions
// 0755 works on Unix, Windows ignores the permission bits
err := os.MkdirAll(dirPath, 0755)

// Create files
// 0644 works on Unix, Windows ignores the permission bits
err := os.WriteFile(filePath, data, 0644)
```

## Why This Matters

**Tests that pass on Linux/macOS may fail on Windows CI if they use hardcoded Unix paths.**

Always use:
- `t.TempDir()` for temporary directories in tests
- `filepath.Join()` for host paths (files on disk)
- `path.Join()` for container paths (inside Podman containers, which are always Linux)

## Related Skills

- `/testing-commands` - Command testing patterns
- `/testing-best-practices` - General testing best practices
- `/working-with-podman-runtime-config` - Podman runtime configuration

## References

- **Go filepath package**: Standard library documentation for host paths
- **Go path package**: Standard library documentation for Unix paths (containers, URLs)
- **Cross-platform tests**: All `*_test.go` files should follow these patterns
- **Example**: `pkg/instances/manager_test.go`, `pkg/cmd/init_test.go`
- **Podman runtime**: `pkg/runtime/podman/` for container path examples
