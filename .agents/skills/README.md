# Agent Skills

This directory contains reusable skills that can be discovered and executed by any AI agent.

## Structure

Each skill is contained in its own subdirectory with a `SKILL.md` file that defines:
- The skill name and description
- Input parameters and argument hints
- Detailed instructions for execution
- Usage examples

## Available Skills

### Command Development
- **add-command-simple**: Add a simple CLI command without JSON output support
- **add-command-with-json**: Add a new CLI command with JSON output support
- **add-alias-command**: Add an alias command that delegates to an existing command
- **add-parent-command**: Add a parent/root command that has subcommands
- **implementing-command-patterns**: Advanced patterns for implementing commands including flag binding, JSON output, and interactive sessions

### Runtime Development
- **add-runtime**: Add a new runtime implementation to the kdn runtime system
- **working-with-runtime-system**: Guide to understanding and working with the kdn runtime system architecture

### Configuration & Integration
- **working-with-config-system**: Guide to workspace configuration for environment variables and mount points at multiple levels
- **working-with-podman-runtime-config**: Guide to configuring the Podman runtime including image setup, agent configuration, and containerfile generation
- **working-with-steplogger**: Complete guide to integrating StepLogger for user progress feedback in commands and runtimes
- **working-with-instances-manager**: Guide to using the instances manager API for workspace management and project detection
- **working-with-secrets**: Guide to the secrets abstraction including the Store, SecretService registry, and how to add new named secret types

### Testing
- **testing-commands**: Comprehensive guide to testing CLI commands with unit tests, E2E tests, and best practices
- **testing-best-practices**: Testing best practices including parallel execution, fake objects, and factory injection patterns

### Development Standards
- **cross-platform-development**: Essential patterns for cross-platform compatibility including path handling and testing practices
- **copyright-headers**: Add or update Apache License 2.0 copyright headers to source files

### Tools
- **commit**: Generate conventional commit messages based on staged changes
- **complete-pr**: Checklist and guidance for ensuring a PR is complete with code, tests, and documentation

## Usage

Agents can discover skills by scanning this directory for `SKILL.md` files. Each skill's metadata and instructions are contained in its respective file.
