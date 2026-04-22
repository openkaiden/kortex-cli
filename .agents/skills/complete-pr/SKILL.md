---
name: complete-pr
description: Checklist and guidance for ensuring a PR is complete with code, tests, and documentation
argument-hint: ""
---

# Complete PR Checklist

This skill helps ensure a pull request is complete before submission: code changes, tests covering those changes, and all relevant documentation updates.

## Overview

A complete PR must include:

1. **Code** — the implementation itself
2. **Tests** — covering the new or changed code
3. **Documentation** — keeping AGENTS.md, skills, and README.md in sync

Work through each section below and confirm everything applies to the PR at hand.

## 1. Code

- [ ] Implementation is complete and matches the stated goal of the PR
- [ ] No dead code, commented-out blocks, or debug leftovers
- [ ] No speculative abstractions — only what the task requires
- [ ] No new security vulnerabilities (injection, path traversal, hardcoded secrets…)
- [ ] Copyright headers present on all new source files (use `/copyright-headers`)
- [ ] Code is formatted (`make fmt`) and passes vet (`make vet`)

## 2. Tests

All changes need test coverage. Use `/testing-commands` and `/testing-best-practices` for patterns.

### New commands or functions

- [ ] Unit test for the `preRun` / validation logic
- [ ] E2E test exercising the full command execution path
- [ ] `Test<Command>Cmd_Examples` test validating the `Example` field
- [ ] Error path tests (invalid args, missing flags, bad state)

### Changed behaviour

- [ ] Existing tests updated to reflect the new behaviour
- [ ] Regression test added if the PR fixes a bug

### General requirements

- [ ] Every test function calls `t.Parallel()` as its first line (unless it uses `t.Setenv()`)
- [ ] Temporary directories created with `t.TempDir()` — never `os.MkdirTemp` or hardcoded paths
- [ ] Tests pass: `make test`

## 3. Documentation

Documentation must stay in sync with the code. Review each area below and update what applies.

### AGENTS.md (CLAUDE.md)

Update when the PR introduces or changes something an AI agent working in this repo needs to know:

- [ ] New module or package added → document its design pattern and purpose
- [ ] New global flag or persistent flag added → document access pattern and priority order
- [ ] Architecture change (new system, new interface, new registration pattern) → add/update the relevant section
- [ ] New command-line convention → add to the appropriate section
- [ ] New configuration file or storage location → document the path and format
- [ ] Existing guidance is now wrong or incomplete → correct it

### Skills (`skills/`)

Skills document reusable capabilities for AI agents. Update when:

- [ ] A new repeatable workflow is introduced → create a new skill (`skills/<name>/SKILL.md`)
- [ ] An existing skill references code that has changed (function names, file paths, flag names) → update the skill
- [ ] Step counts, patterns, or examples in a skill are now outdated → refresh them
- [ ] New skill created → no symlink needed; `.claude/skills` is already a symlink to `../.agents/skills`

### README.md

Update the README when the PR changes something a user or operator of the tool will observe:

- [ ] New command or subcommand → add to the command reference
- [ ] New flag on an existing command → document flag, default, and effect
- [ ] Changed default behaviour → update the relevant description
- [ ] New installation step or dependency → add to the setup section
- [ ] Removed or renamed command/flag → remove stale references

## Process

```bash
# 1. Confirm all tests pass
make test

# 2. Confirm formatting and vet
make ci-checks

# 3. Review the diff for documentation gaps
git diff main...HEAD -- '*.go' | grep -E "^(\+func |\+type |\+const |// |//)" | head -40

# 4. Check what doc files changed alongside the code
git diff --name-only main...HEAD
```

Go through the checklist, open the items that apply, and complete them before marking the PR ready for review.

## Related Skills

- `/commit` — craft a conventional commit message for the changes
- `/testing-commands` — patterns for command unit and E2E tests
- `/testing-best-practices` — parallel execution, fakes, factory injection
- `/copyright-headers` — add or update Apache 2.0 headers
- `/add-command-with-json` — full template for a new command (includes tests and doc steps)
