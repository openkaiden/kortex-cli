---
name: config-kdn-workspace
description: Configure your kdn workspace interactively — add environment variables, mounts, secrets, MCP servers, skills, network policies, dev container features, and port forwarding at any configuration level (workspace, global, project, or agent)
argument-hint: "what do you want to configure? (e.g. 'add my GitHub token', 'mount my .gitconfig globally', 'allow network access to api.example.com')"
---

# Configure a kdn Workspace

Use this skill to help users configure their kdn workspace. Read the goal from the argument (or ask the user what they want to set up), then guide them to the right configuration level and produce the correct JSON.

## Important: sandbox context

The agent runs inside a sandboxed workspace container. Only files from the mounted source directory are directly accessible:

- **Workspace config** (`.kaiden/workspace.json`) → available inside the workspace at `/workspace/sources/.kaiden/workspace.json`. **Can always be edited directly.**
- **Global / project config** (`~/.kdn/config/projects.json`) and **agent config** (`~/.kdn/config/agents.json`) → live on the **host machine**. They are only accessible from inside the workspace if the user has mounted `~/.kdn/config` into the workspace.

### Accessing multi-level configs from inside the workspace

If the user wants the agent to help with global, project, or agent config, suggest mounting the config directory. Add this to `.kaiden/workspace.json` (or the agent/global config on the host):

```json
{
  "mounts": [
    { "host": "$HOME/.kdn/config", "target": "$HOME/.kdn/config" }
  ]
}
```

After adding this mount and re-registering the workspace, `~/.kdn/config/` will be available inside the container.

**Never touch files outside `~/.kdn/config/`** — do not read, write, or suggest modifying anything else under `~/.kdn/` (such as instances, runtimes, or binary caches). Only `~/.kdn/config/` is in scope for this skill.

When `~/.kdn/config` is not mounted, generate the JSON snippet and tell the user exactly where to apply it on their host.

## Overview

kdn workspace configuration controls what gets injected into a workspace at runtime:

- **Environment variables** — plain values or references to secrets
- **Mounts** — host directories made available inside the container
- **Skills** — skill directories mounted into the agent
- **MCP servers** — local (stdio) or remote (SSE) tool servers for the agent
- **Network access** — allow-all or deny with an explicit host allowlist
- **Secrets** — kdn secrets injected as HTTP headers via OneCLI (distinct from Podman secrets used in environment variable entries)
- **Dev container features** — tools installed into the image at build time (⚠️ **base image is Fedora**: verify feature compatibility before recommending; many features only support Debian/Ubuntu)
- **Port forwarding** — workspace ports exposed on the host

## Important: Base Image Compatibility

⚠️ **The workspace base image is Fedora (Red Hat based).** When recommending dev container features:

1. **Check feature compatibility first** — many features are Debian/Ubuntu only and will fail
2. **Look for `dnf`/RPM support** in the feature's documentation or install script
3. **Avoid features using `apt-get`** or Debian-specific commands
4. **Prefer local features** (using `dnf`) when compatibility is uncertain

This only affects **dev container features**. Other configuration types (mounts, environment variables, secrets, etc.) are not affected.

## Step 1: Choose the right configuration level

Ask the user which scope they need. Present these choices:

| Level | File (host path) | In-workspace path | When to use |
|---|---|---|---|
| **Workspace** | `<project>/.kaiden/workspace.json` | `/workspace/sources/.kaiden/workspace.json` | Project-specific, shared with all developers, committed to git |
| **Global** | `~/.kdn/config/projects.json` (`""` key) | `~/.kdn/config/projects.json` if mounted | Applies to every project on this machine (e.g. `.gitconfig`, SSH keys) |
| **Project** | `~/.kdn/config/projects.json` (project ID key) | `~/.kdn/config/projects.json` if mounted | This project only, stays on your machine |
| **Agent** | `~/.kdn/config/agents.json` | `~/.kdn/config/agents.json` if mounted | Agent-specific settings (e.g. Claude-only or Goose-only) |

**Precedence (highest wins):** Agent > Project > Global > Workspace

If unsure: personal credentials → global or agent config; project tooling → workspace config.

## Step 2: Identify the configuration target

- **Workspace config**: edit `/workspace/sources/.kaiden/workspace.json`. Create the `.kaiden/` directory if it doesn't exist.
- **Global / project config**: edit `~/.kdn/config/projects.json` (requires `~/.kdn/config` to be mounted, or apply on the host).
  - Use `""` as the key for global settings.
  - Use the project ID for project-specific settings. Find it by running `kdn workspace list --output json` on the **host** and reading the `project` field for the workspace.
- **Agent config**: edit `~/.kdn/config/agents.json` (requires `~/.kdn/config` to be mounted, or apply on the host). Use the agent name (`claude`, `goose`, `cursor`, `opencode`) as the key.

## Step 3: Build the JSON

Use the sections below to assemble the JSON snippet and merge it into the right file.

---

### Environment variables

```json
{
  "environment": [
    { "name": "NODE_ENV", "value": "development" },
    { "name": "SECRET_VAR", "secret": "my-podman-secret" }
  ]
}
```

Rules:
- `name`: valid Unix variable name (letter/underscore first, then letters/digits/underscores)
- Use `value` for plain text
- Use `secret` to reference a **Podman secret** created with `podman secret create` — this is a runtime-specific mechanism supported only by the Podman runtime. The Podman runtime injects the secret value as the environment variable inside the container. It is **not** the same as a kdn secret created with `kdn secret create`.
- `value` and `secret` are mutually exclusive

---

### Mounts

```json
{
  "mounts": [
    { "host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true },
    { "host": "$SOURCES/../sibling-dir", "target": "$SOURCES/../sibling-dir" },
    { "host": "/absolute/path", "target": "/workspace/data", "ro": true }
  ]
}
```

Path variables (resolved relative to the **host** at workspace creation time):
- `$HOME` → host home dir / `/home/agent` inside the container
- `$SOURCES` → workspace sources dir on host / `/workspace/sources` inside the container

Rules:
- Both `host` and `target` must be absolute or start with `$SOURCES` or `$HOME`
- `ro: true` makes the mount read-only (omit or set `false` for read-write)

---

### Skills

```json
{
  "skills": [
    "/absolute/path/to/my-skill",
    "$HOME/skills/review-skill"
  ]
}
```

Each entry is a **host** directory containing a `SKILL.md`. The directory is mounted read-only inside the agent's skills directory using its basename:

| Agent | Mount target |
|---|---|
| Claude Code | `~/.claude/skills/<basename>/` |
| Goose | `~/.agents/skills/<basename>/` |
| Cursor | `~/.cursor/skills/<basename>/` |
| OpenCode | `~/.opencode/skills/<basename>/` |

Rules: paths must be absolute or start with `$HOME` (`$SOURCES` is not supported). These are host paths resolved at workspace creation time.

---

### MCP servers

```json
{
  "mcp": {
    "commands": [
      {
        "name": "filesystem",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace/sources"],
        "env": { "NODE_ENV": "production" }
      }
    ],
    "servers": [
      {
        "name": "remote-api",
        "url": "https://api.example.com/mcp",
        "headers": { "Authorization": "Bearer mytoken" }
      }
    ]
  }
}
```

Rules:
- `name` must be unique across **both** `commands` and `servers`
- `command` (for command entries) and `url` (for server entries) are required
- MCP configuration is baked into the agent settings at `kdn init` time, not at runtime

---

### Network access

```json
{
  "network": {
    "mode": "deny",
    "hosts": ["api.github.com", "registry.npmjs.org"]
  }
}
```

- `mode: "allow"` — unrestricted outbound access (do not set `hosts`)
- `mode: "deny"` — block everything except listed hosts (and hosts auto-injected from secrets/credentials)
- Omit `hosts` with `deny` for a fully isolated workspace

**Auto-injected hosts:** When secrets are configured, their required hosts are added automatically. For example, a `github` secret automatically allows `api.github.com` — no explicit `hosts` entry needed.

Network merging across levels: the **stricter** (lower-precedence) policy wins. A workspace-level `deny` cannot be overridden to `allow` by an agent config.

---

### Secrets (kdn secrets — HTTP header injection)

kdn secrets are created on the **host** with `kdn secret create`, stored in the system keychain, and injected as **HTTP headers** by OneCLI into matching outbound requests. The agent inside the workspace cannot create kdn secrets.

```bash
# Run on the HOST (not inside the workspace)
kdn service list                    # list available service types
kdn secret create my-github-token --type github --value ghp_xxxx
```

Then reference the secret by name in the `secrets` list of any config level:

```json
{
  "secrets": ["my-github-token"]
}
```

**This is distinct from the `secret` field in environment variables**, which references Podman secrets (`podman secret create`) — a Podman-only mechanism for injecting values as environment variables. kdn secrets and Podman secrets are separate systems:

| | kdn secrets (`kdn secret create`) | Podman secrets (`podman secret create`) |
|---|---|---|
| Config field | `secrets: ["name"]` (top-level list) | `environment[*].secret: "name"` |
| Delivery | HTTP header injected by OneCLI | Environment variable inside container |
| Runtime support | All runtimes | Podman only |
| Use case | Outbound API credentials | Any value a local tool needs as an env var |

---

### Dev container features

⚠️ **CRITICAL: Feature Compatibility Check Required**

The base image is **Fedora** (Red Hat based, uses `dnf`). **Many dev container features are designed for Debian/Ubuntu only** and will fail on Fedora. Before proposing any feature to the user:

1. **Check the feature's documentation** to verify it supports Red Hat/Fedora/RPM-based systems
2. **Look for `apt-get` or `apt` usage** in the feature's install script — these will NOT work on Fedora
3. **Prefer features that explicitly support multiple distros** or are distro-agnostic
4. **When in doubt, suggest a local feature** using `dnf` instead

**Common compatible features** (verified to work on Fedora):
- `ghcr.io/devcontainers/features/common-utils` — shell utilities, supports RPM
- `ghcr.io/devcontainers/features/git` — git and git-lfs, supports RPM

**Common INCOMPATIBLE features** (Debian/Ubuntu only):
- Many language features may only support `apt-get`
- Check each feature's GitHub repo before recommending

```json
{
  "features": {
    "ghcr.io/devcontainers/features/common-utils:1": {},
    "./tools/my-local-feature": {}
  }
}
```

Features are installed into the image at **build time** (`kdn init`), not at runtime. Adding or changing features requires re-registering the workspace. Use `{}` to accept all defaults.

Rules: IDs must be OCI references or relative paths (`./…`). `https://` tarball URIs are not supported. Local paths are resolved relative to `.kaiden/`.

#### Local features

**Recommended approach for Fedora compatibility:** Since many public dev container features only support Debian/Ubuntu, creating local features using `dnf` is often the most reliable way to install tools.

Local features should be placed in the `.kaiden/` directory (or a subdirectory) and referenced with a relative path starting with `./`:

**Directory structure example:**

```text
.kaiden/
  workspace.json
  features/
    custom-tools/
      devcontainer-feature.json
      install.sh
```

**Feature reference in workspace.json:**

```json
{
  "features": {
    "./features/custom-tools": {}
  }
}
```

**Feature metadata** (`.kaiden/features/custom-tools/devcontainer-feature.json`):

```json
{
  "id": "custom-tools",
  "version": "1.0.0",
  "name": "Custom Tools",
  "description": "Installs project-specific tools",
  "options": {
    "version": {
      "type": "string",
      "default": "latest",
      "description": "Version to install"
    }
  }
}
```

**Install script** (`.kaiden/features/custom-tools/install.sh`):

```bash
#!/bin/bash
set -e

# The base image is Fedora, so use dnf for package installation
dnf install -y jq wget curl

# Install other tools as needed
# Options are available as environment variables (uppercase with underscores)
echo "Installing version: ${VERSION}"
```

**Important notes for local features:**
- The base image is **Fedora**, so use `dnf` for package management (not `apt`, `yum`, or `apk`)
- Features run as **root** during image build, but the `agent` user and `/home/agent/` already exist
- Environment variables are available to install scripts:
  - `_REMOTE_USER=agent` — the container user
  - `_REMOTE_USER_HOME=/home/agent` — the user's home directory
  - Option values from `devcontainer-feature.json` (uppercased, non-alphanumeric → `_`)
- Install scripts must be executable (`chmod +x install.sh`)
- Use `set -e` to fail fast on errors

---

### Port forwarding

```json
{
  "ports": [8080, 3000]
}
```

kdn allocates a free host port for each listed workspace port at creation time. Use `kdn workspace open <name> <port>` on the host to open a forwarded port in the browser.

---

## Step 4: Apply the change

### Changes to workspace config (`/workspace/sources/.kaiden/workspace.json`)

This file is directly editable from inside the workspace. Read the current file (if it exists), merge the new JSON into it, and write it back. Create the `.kaiden/` directory and `workspace.json` if they don't exist.

The change takes effect the next time the workspace is registered. If the workspace is already registered, re-register it on the host:

```bash
# Run on the HOST
kdn workspace remove <name>
kdn init <source-dir> --runtime <runtime> --agent <agent>
```

### Changes to global / project config (`~/.kdn/config/projects.json`)

**If `~/.kdn/config` is mounted:** edit the file directly at `~/.kdn/config/projects.json`.

**If not mounted:** present the JSON snippet to the user:

> Please apply this change to `~/.kdn/config/projects.json` on your host machine:
>
> ```json
> {
>   "": {
>     "mounts": [
>       { "host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true }
>     ]
>   }
> }
> ```
>
> Merge this into the existing file (create it at `~/.kdn/config/projects.json` if it doesn't exist). The change takes effect the next time you run `kdn init` for this workspace.

For project-specific settings, use the project ID as the key. Find it on the host with:

```bash
kdn workspace list --output json   # read the "project" field
```

To make future config changes easier, consider adding this mount to `.kaiden/workspace.json` so the agent can edit global and agent configs directly:

```json
{
  "mounts": [
    { "host": "$HOME/.kdn/config", "target": "$HOME/.kdn/config" }
  ]
}
```

### Changes to agent config (`~/.kdn/config/agents.json`)

Same as above: edit directly if `~/.kdn/config` is mounted, otherwise present the JSON to the user.

```json
{
  "claude": {
    "environment": [
      { "name": "ANTHROPIC_MODEL", "value": "claude-sonnet-4-20250514" }
    ]
  }
}
```

---

## Common use cases

### Share git credentials across all projects (global)

Add to `~/.kdn/config/projects.json` under the `""` key (apply on the host or via mounted config):

```json
{
  "": {
    "mounts": [
      { "host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true }
    ]
  }
}
```

### Reuse Claude Code settings (agent config)

Add to `~/.kdn/config/agents.json` under the `"claude"` key (apply on the host or via mounted config):

```json
{
  "claude": {
    "mounts": [
      { "host": "$HOME/.claude", "target": "$HOME/.claude" },
      { "host": "$HOME/.claude.json", "target": "$HOME/.claude.json" }
    ]
  }
}
```

### Inject a GitHub token (secret + network)

Instruct the user to run on the host:

```bash
kdn secret create my-github-token --type github --value ghp_xxxx
```

Then add to `.kaiden/workspace.json` (editable from inside the workspace):

```json
{
  "secrets": ["my-github-token"],
  "network": { "mode": "deny" }
}
```

The token is injected as a `Bearer` header for `api.github.com` requests. The host is added to the allowlist automatically.

### Mount a git worktree

Add to `.kaiden/workspace.json`:

```json
{
  "mounts": [
    { "host": "$SOURCES/../main", "target": "$SOURCES/../main" }
  ]
}
```

### Allow network access to specific hosts

Add to `.kaiden/workspace.json`:

```json
{
  "network": {
    "mode": "deny",
    "hosts": ["api.example.com", "registry.npmjs.org"]
  }
}
```

### Add a dev container feature (verified Fedora-compatible)

⚠️ **Always verify feature compatibility with Fedora before recommending.** Check the feature's documentation or install script for RPM/dnf support.

If the feature is compatible, add to `.kaiden/workspace.json`:

```json
{
  "features": {
    "ghcr.io/devcontainers/features/common-utils:1": {}
  }
}
```

Then re-register on the host: `kdn workspace remove -f <name> && kdn init <dir> --runtime podman --agent <agent>`

**If unsure about compatibility, suggest a local feature instead** (see next example).

### Add a local dev container feature (custom tools)

Create the feature directory structure in `.kaiden/features/project-tools/` with a `devcontainer-feature.json` and `install.sh`. The install script should use `dnf` for packages (base image is Fedora):

```bash
#!/bin/bash
set -e
dnf install -y ripgrep fd-find
```

Then reference it in `.kaiden/workspace.json`:

```json
{
  "features": {
    "./features/project-tools": {}
  }
}
```

Re-register to apply: `kdn workspace remove -f <name> && kdn init <dir> --runtime podman --agent <agent>`

---

## Auto-configuration shortcut

For common cases (API keys from environment variables, home config file mounts, language detection), suggest running on the **host**:

```bash
kdn autoconf          # interactive — detect and prompt for each item
kdn autoconf --yes    # apply immediately to global config without prompts
```

`kdn autoconf` detects known API keys, config files, and programming languages, and writes the resulting secrets and mounts to the appropriate config file.

---

## Validation

Configuration is validated when running `kdn init` on the host. Common errors and fixes:

| Error | Fix |
|---|---|
| `has both value and secret set` | Remove one — `value` and `secret` are mutually exclusive |
| `missing host` / `missing target` | Add both `host` and `target` to every mount entry |
| `invalid variable name` | Variable names must start with a letter/underscore, no hyphens or spaces |
| `hosts must not be set when mode is allow` | Remove `hosts` when `mode` is `"allow"` |
| `duplicate MCP server name` | Names must be unique across both `commands` and `servers` |

---

## Merging behavior summary

| Field | How configs merge |
|---|---|
| `environment` | Later (higher-precedence) level wins by variable name |
| `mounts` | Deduplicated by `host`+`target`; first occurrence wins |
| `skills` | Deduplicated by path; base first |
| `mcp` | Deduplicated by `name`; higher-precedence wins |
| `network` | Stricter (lower-precedence) policy wins; deny cannot be loosened by a higher level |
| `secrets` | Deduplicated by name; base first |
| `features` | Higher-precedence level wins by feature ID |
| `ports` | Union-merged, deduplicated |
