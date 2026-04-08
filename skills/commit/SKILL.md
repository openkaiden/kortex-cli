---
name: commit
description: Conventional Commit Message Generator
argument-hint: "[optional context]"
---

# Conventional Commit Message Generator

## Overview

You're a senior software engineer with a strong focus on clean Git history and clear communication through commit messages.
You understand that commit messages are documentation that helps future developers (including yourself) understand why changes were made.

Your task is to help create well-structured conventional commit messages for the current changes.

## Pre-fetched Context

### Recent Commits (for style reference)

```bash
git log --oneline -10 2>/dev/null || echo "No commits yet"
```

### Co-Authored-By Usage

```bash
git log --format="%b" -20 2>/dev/null | grep -i "Co-Authored-By" | head -3 || echo "No Co-Authored-By found"
```

### Git Status

```bash
git status --short 2>/dev/null || echo "Not a git repository"
```

### Changes Summary

```bash
git diff HEAD --stat 2>/dev/null || git diff --stat 2>/dev/null || echo "No changes"
```

### Full Diff

```bash
git diff HEAD 2>/dev/null || git diff 2>/dev/null || echo "No changes to show"
```

## Conventional Commit Format

```text
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

## Commit Types

| Type       | Description                                               | Semantic Version Impact |
|------------|-----------------------------------------------------------|-------------------------|
| `feat`     | A new feature                                             | MINOR                   |
| `fix`      | A bug fix or performance improvement                      | PATCH                   |
| `refactor` | A code change that neither fixes a bug nor adds a feature | PATCH                   |
| `test`     | Adding missing tests or correcting existing tests         | PATCH                   |
| `chore`    | Maintenance tasks (with scope for specifics)              | PATCH                   |
| `revert`   | Reverts a previous commit                                 | PATCH                   |

## Chore Scopes

Use `chore(<scope>)` for maintenance tasks:

| Scope   | Description                                                             |
|---------|-------------------------------------------------------------------------|
| `docs`  | Documentation only changes (can also use `docs:` directly)              |
| `style` | Code style changes (white-space, formatting, missing semi-colons, etc.) |
| `build` | Changes that affect the build system or external dependencies           |
| `ci`    | Changes to CI configuration files and scripts (can also use `ci:` directly) |
| `deps`  | Dependency updates                                                      |

## Breaking Changes

For breaking changes, add an exclamation mark after the type/scope and include a BREAKING CHANGE footer:

```text
feat(api)!: remove deprecated endpoints

BREAKING CHANGE: The /v1/users endpoint has been removed. Use /v2/users instead.
```

## Guidelines

1. **Description (subject line)**:
   - Use imperative mood ("add" not "added" or "adds")
   - Don't capitalize the first letter
   - No period at the end
   - Keep it under 50 characters if possible (max 72)
   - Be specific and meaningful

2. **Scope** (optional but recommended):
   - Use lowercase
   - Use a noun describing the section of the codebase
   - Examples: `cli`, `cmd`, `instances`, `config`, `api`, `agent`

3. **Body** (recommended for non-trivial changes):
   - Always include for changes touching multiple files or concepts
   - Summarize the nature of the change (new feature, bug fix, refactor, etc.)
   - Focus on **why** the change was made, not how (the code shows how)
   - 1-3 concise sentences
   - Wrap at 72 characters
   - Use blank line to separate from subject
   - Can use bullet points

4. **Footer** (optional):
   - Reference issues: `Fixes #123`, `Closes #456`, `Refs #789`
   - Breaking changes: `BREAKING CHANGE: description`
   - Co-authors: `Co-Authored-By: Name <email>`

## Process

Using the pre-fetched context above:

1. **Analyze changes**: Review the git status and diff to understand what's being committed.

2. **Summarize**: Identify the nature of the change (new feature, enhancement, bug fix, refactoring, etc.) and the motivation behind it. This summary becomes the commit body.

3. **Scoped changes check**: If you have been working on specific files during this session (active context), ask the user:
   - Do you want to commit **all changes** in the working tree?
   - Or only the **scoped changes** related to the current work?

   This prevents accidentally committing unrelated changes that happen to be in the working tree.

4. **Suggest commit message**: Based on the project's commit style and the changes, suggest a commit message following the conventional commit format.

5. **Co-Authored-By**: If the project's commit history shows usage of `Co-Authored-By`, include this trailer with the current agent information:
   - **If agent metadata is available**: Use the agent name and noreply email from the runtime metadata (e.g., available through agent context or environment variables)
   - **If agent metadata is unavailable**: Prompt the user to confirm or enter the co-author information

   Format:
   ```text
   Co-Authored-By: <agent name> <agent noreply email>
   ```

   Example with populated values:
   ```text
   Co-Authored-By: Claude Code (Claude Sonnet 4.5) <noreply@anthropic.com>
   ```

6. **Commit**: Once approved, stage and commit with sign-off.

   **IMPORTANT: Always use SEPARATE Bash tool calls for staging and committing (never chain with `&&`).** This ensures proper confirmation before the commit is created.

   First, stage the changes:
   ```bash
   # Stage all changes
   git add -A

   # Or stage only specific files for scoped commits
   git add <file1> <file2>
   ```

   Then, in a **separate** Bash tool call, create the commit:
   ```bash
   # Simple commit with sign-off
   git commit --signoff -m "<message>"

   # For multi-line messages with body
   git commit --signoff -m "<subject>" -m "<body>"

   # Or using heredoc for complex messages (including Co-Authored-By if applicable)
   # NOTE: Replace <agent name> and <agent noreply email> with values from runtime
   # agent metadata or prompt the user if metadata is unavailable
   git commit --signoff -m "$(cat <<'EOF'
   <type>(<scope>): <description>

   <body>

   Co-Authored-By: <agent name> <agent noreply email>
   EOF
   )"
   ```

**Important:**
- The `--signoff` flag is **ALWAYS required** - it adds a `Signed-off-by` trailer with the name and email from git config.
- Never omit `--signoff` under any circumstances.
- **Never** chain `git add` and `git commit` in the same command — they must be separate tool calls so the user can review staged changes and confirm the commit.

## Examples

**Simple feature:**

```text
feat(cli): add workspace list command

Allows users to view all registered workspaces with their paths
and configuration, making it easier to manage multiple projects.
```

**Bug fix with scope:**

```text
fix(instances): pass absolute storage path to manager

The manager was receiving relative paths which caused issues when
the working directory changed, leading to incorrect file operations.
```

**Documentation update:**

```text
docs: add installation instructions for Windows

The project lacked Windows-specific setup guidance, which was the
most requested topic in issues.
```

**CI configuration change:**

```text
ci: add caching to speed up GitHub Actions workflow

CI builds were taking 12+ minutes due to repeated dependency
downloads. Adding Go module cache reduces this to ~4 minutes.
```

**Performance improvement:**

```text
fix(parser): reduce memory allocation in tokenizer loop

The tokenizer was allocating a new buffer per token, causing GC
pressure on large inputs. Reusing a shared buffer cuts memory
usage by ~60%.
```

**Refactoring with body:**

```text
refactor(cmd): do not declare commands as global variables

Commands were declared as global variables which made testing
difficult and created initialization order dependencies. This
change uses factory functions instead.
```

**Feature with issue reference:**

```text
feat(cmd): add workspace remove command

Allows users to remove workspaces from the registry when they
are no longer needed, keeping the workspace list clean.

Closes #55
```

**Test addition:**

```text
test: add tests for KDN_STORAGE support

Ensures the environment variable is properly respected and has
the correct priority order relative to the --storage flag.
```

## Additional Context

If you provide additional context as an argument, it will be taken into account when crafting the commit message.

**Usage:**

```bash
/commit [optional context about the changes]
```
