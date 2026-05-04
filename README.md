# kdn

[![codecov](https://codecov.io/gh/openkaiden/kdn/branch/main/graph/badge.svg)](https://codecov.io/gh/openkaiden/kdn)
[![Documentation](https://img.shields.io/badge/documentation-blue)](https://openkaiden.github.io/kdn/)

kdn is a command-line interface for launching and managing AI agents in isolated, reproducible workspaces. It creates runtime-based environments (containers, VMs, or other backends) where agents run with your project source code mounted, automatically configured and ready to use — no manual onboarding or setup required.

The architecture is built around pluggable runtimes. The first supported runtime is **Podman**, which creates container-based workspaces using a custom Fedora image. Additional runtimes (e.g., MicroVM, Kubernetes) can be added to support other execution environments.

**Supported Agents**

- **Claude Code** - Anthropic's official CLI for Claude
- **Goose** - AI agent for development tasks
- **Cursor** - AI-powered code editor agent
- **OpenCode** - Open-source AI coding agent

**Key Features**

- Isolated workspaces per project, each running in its own runtime instance
- Pluggable runtime system — Podman is the default, with support for adding other runtimes
- Automatic agent configuration (onboarding flags, trusted directories) on workspace creation
- Multi-level configuration: workspace, global, project-specific, and agent-specific settings
- Inject environment variables and mount directories into workspaces at multiple scopes
- Control network access with allow/deny policies per workspace
- Connect to MCP servers and integrate with various LLM providers (including Vertex AI)
- Consistent CLI interface across different agent types and runtimes

## Getting Started

### Prerequisites

- Go 1.26+
- Make

### Build

```bash
make build
```

This creates the `kdn` binary in the current directory.

### Run

```bash
# Display help and available commands
./kdn --help

# Execute a specific command
./kdn <command> [flags]
```

### Install

To install the binary to your `GOPATH/bin` for system-wide access:

```bash
make install
```

### Run Tests

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage
```

## Glossary

### Agent
An AI assistant that can perform tasks autonomously. In kdn, agents are the different AI tools (Claude Code, Goose, Cursor, OpenCode) that can be launched and configured.

### LLM (Large Language Model)
The underlying AI model that powers the agents. Examples include Claude (by Anthropic), GPT (by OpenAI), and other language models.

### MCP (Model Context Protocol)
A standardized protocol for connecting AI agents to external data sources and tools. MCP servers provide agents with additional capabilities like database access, API integrations, or file system operations.

### Runtime
The environment where workspaces run. kdn supports multiple runtimes (e.g., Podman containers), allowing workspaces to be hosted on different backends depending on your needs.

### Service
A secret service definition that describes how to inject credentials into workspace HTTP requests. Each service specifies a host pattern to match, the HTTP header to set, and which environment variables hold the credential value.

### Skills
Pre-configured capabilities or specialized functions that can be enabled for an agent. Skills extend what an agent can do, such as code review, testing, or specific domain knowledge.

### Workspace
A registered directory containing your project source code and its configuration. Each workspace is tracked by kdn with a unique ID and a human-readable name. Workspaces can be accessed using either their ID or name in all commands (start, stop, remove, terminal).

## Scenarios

### Claude with a Model from Vertex AI

This scenario demonstrates how to configure Claude Code to use a model hosted on Google Cloud Vertex AI instead of the default Anthropic API. This is useful when you need to use Claude through your Google Cloud organization's billing or compliance setup.

**Prerequisites:**

- A Google Cloud project with the Vertex AI API enabled and Claude models available
- Google Cloud credentials configured on your host machine (via `gcloud auth application-default login`)

**Step 1: Configure Claude agent settings**

Create or edit `~/.kdn/config/agents.json` to add the required environment variables and mount your Google Cloud credentials into the workspace:

```json
{
  "claude": {
    "environment": [
      {
        "name": "CLAUDE_CODE_USE_VERTEX",
        "value": "1"
      },
      {
        "name": "ANTHROPIC_VERTEX_PROJECT_ID",
        "value": "my-gcp-project-id"
      },
      {
        "name": "CLOUD_ML_REGION",
        "value": "my-region"
      }
    ],
    "mounts": [
      {"host": "$HOME/.config/gcloud", "target": "$HOME/.config/gcloud", "ro": true}
    ]
  }
}
```

**Fields:**

- `CLAUDE_CODE_USE_VERTEX` - Set to `1` to instruct Claude Code to use Vertex AI instead of the Anthropic API
- `ANTHROPIC_VERTEX_PROJECT_ID` - Your Google Cloud project ID where Vertex AI is configured
- `CLOUD_ML_REGION` - The region where Claude is available on Vertex AI
- `$HOME/.config/gcloud` mounted read-only - Provides the workspace access to your application default credentials

**Step 2: Register and start the workspace**

```bash
# Register a workspace with the Podman runtime and Claude agent
kdn init /path/to/project --runtime podman --agent claude

# Start the workspace (using name or ID)
kdn start my-project

# Connect to the workspace — Claude Code will use Vertex AI automatically
kdn terminal my-project
```

When Claude Code starts, it detects `ANTHROPIC_VERTEX_PROJECT_ID` and `CLOUD_ML_REGION` and routes all requests to Vertex AI using the mounted application default credentials.

**Sharing local Claude settings (optional)**

To reuse your host Claude Code settings (preferences, custom instructions, etc.) inside the workspace, add `~/.claude` and `~/.claude.json` to the mounts:

```json
{
  "claude": {
    "environment": [
      {
        "name": "CLAUDE_CODE_USE_VERTEX",
        "value": "1"
      },
      {
        "name": "ANTHROPIC_VERTEX_PROJECT_ID",
        "value": "my-gcp-project-id"
      },
      {
        "name": "CLOUD_ML_REGION",
        "value": "my-region"
      }
    ],
    "mounts": [
      {"host": "$HOME/.config/gcloud", "target": "$HOME/.config/gcloud", "ro": true},
      {"host": "$HOME/.claude", "target": "$HOME/.claude"},
      {"host": "$HOME/.claude.json", "target": "$HOME/.claude.json"}
    ]
  }
}
```

`~/.claude` contains your Claude Code configuration directory (skills, settings) and `~/.claude.json` stores your account and preferences. These are mounted read-write so that changes made inside the workspace (e.g., updated preferences) are persisted back to your host.

**Notes:**

- Run `gcloud auth application-default login` on your host machine before starting the workspace to ensure valid credentials are available
- The `$HOME/.config/gcloud` mount is read-only to prevent the workspace from modifying your host credentials
- No `ANTHROPIC_API_KEY` is needed when using Vertex AI — credentials are provided via the mounted gcloud configuration
- To pin a specific Claude model, use `--model` flag during `init` (e.g., `--model claude-sonnet-4-20250514`), which takes precedence over any model in default settings, or add an `ANTHROPIC_MODEL` environment variable (e.g., `"claude-opus-4-5"`)

### Starting Claude with Default Settings

This scenario demonstrates how to pre-configure Claude Code's settings so that when it starts inside a workspace, it skips the interactive onboarding flow and uses your preferred defaults. kdn automatically handles the onboarding flags, and you can optionally customize other settings like theme preferences.

**Automatic Onboarding Skip**

When you register a workspace with the Claude agent, kdn automatically:
- Sets `hasCompletedOnboarding: true` to skip the first-run wizard
- Sets `hasTrustDialogAccepted: true` for the workspace sources directory (the exact path is determined by the runtime)

This happens automatically for every Claude workspace — no manual configuration required.

**Optional: Customize Theme and Other Settings**

If you want to customize Claude's theme or other preferences, create default settings:

**Step 1: Create the agent settings directory**

```bash
mkdir -p ~/.kdn/config/claude
```

**Step 2: Write the default Claude settings file**

```bash
cat > ~/.kdn/config/claude/.claude.json << 'EOF'
{
  "theme": "dark-daltonized"
}
EOF
```

**Fields:**

- `theme` - The UI theme for Claude Code (e.g., `"dark"`, `"light"`, `"dark-daltonized"`)

You don't need to set `hasCompletedOnboarding` or `hasTrustDialogAccepted` — kdn adds these automatically when creating the workspace.

**Step 3: Register and start the workspace**

```bash
# Register a workspace — the settings file is embedded in the container image
kdn init /path/to/project --runtime podman --agent claude

# Start the workspace (using name or ID)
kdn start my-project

# Connect — Claude Code starts directly without onboarding
kdn terminal my-project
```

When `init` runs, kdn:
1. Reads all files from `~/.kdn/config/claude/` (e.g., your theme preferences)
2. Automatically adds `hasCompletedOnboarding: true` and marks the workspace sources directory as trusted (the path is determined by the runtime)
3. Copies the final merged settings into the container image at `/home/agent/.claude.json`

Claude Code finds this file on startup and skips onboarding.

**Notes:**

- **Onboarding is skipped automatically** — even if you don't create any settings files, kdn ensures Claude starts without prompts
- The settings are baked into the container image at `init` time, not mounted at runtime — changes to the files on the host require re-registering the workspace to take effect
- Any file placed under `~/.kdn/config/claude/` is copied into the container home directory, preserving the directory structure (e.g., `~/.kdn/config/claude/.some-tool/config` becomes `/home/agent/.some-tool/config` inside the container)
- This approach keeps your workspace self-contained — other developers using the same project are not affected, and your local `~/.claude` directory is not exposed inside the container
- To apply changes to the settings, remove and re-register the workspace: `kdn remove <workspace-id>` then `kdn init` again

### Using Goose Agent with a Model from Vertex AI

This scenario demonstrates how to configure the Goose agent in a kdn workspace using Vertex AI as the backend, covering credential injection, sharing your local gcloud configuration, and pre-configuring the default model.

#### Authenticating with Vertex AI

Goose can use Google Cloud Vertex AI as its backend. Authentication relies on Application Default Credentials (ADC) provided by the `gcloud` CLI. Mount your local `~/.config/gcloud` directory to make your host credentials available inside the workspace, and set the `GCP_PROJECT_ID`, `GCP_LOCATION`, and `GOOSE_PROVIDER` environment variables to tell Goose which project and region to use.

Create or edit `~/.kdn/config/agents.json`:

```json
{
  "goose": {
    "environment": [
      {
        "name": "GOOSE_PROVIDER",
        "value": "gcp_vertex_ai"
      },
      {
        "name": "GCP_PROJECT_ID",
        "value": "my-gcp-project"
      },
      {
        "name": "GCP_LOCATION",
        "value": "my-region"
      }
    ],
    "mounts": [
      {"host": "$HOME/.config/gcloud", "target": "$HOME/.config/gcloud", "ro": true}
    ]
  }
}
```

The `~/.config/gcloud` directory contains your Application Default Credentials and active account configuration. It is mounted read-only so that credentials are available inside the workspace while the host configuration remains unmodified.

Then register and start the workspace:

```bash
# Register a workspace with the Podman runtime and Goose agent
kdn init /path/to/project --runtime podman --agent goose

# Start the workspace
kdn start my-project

# Connect — Goose starts with Vertex AI configured
kdn terminal my-project
```

#### Sharing Local Goose Settings

To reuse your host Goose settings (model preferences, provider configuration, etc.) inside the workspace, mount the `~/.config/goose` directory.

Edit `~/.kdn/config/agents.json` to add the mount alongside the Vertex AI configuration:

```json
{
  "goose": {
    "environment": [
      {
        "name": "GOOSE_PROVIDER",
        "value": "gcp_vertex_ai"
      },
      {
        "name": "GCP_PROJECT_ID",
        "value": "my-gcp-project"
      },
      {
        "name": "GCP_LOCATION",
        "value": "my-region"
      }
    ],
    "mounts": [
      {"host": "$HOME/.config/gcloud", "target": "$HOME/.config/gcloud", "ro": true},
      {"host": "$HOME/.config/goose", "target": "$HOME/.config/goose"}
    ]
  }
}
```

The `~/.config/goose` directory contains your Goose configuration (settings, model preferences, etc.). It is mounted read-write so that changes made inside the workspace are persisted back to your host.

#### Using Default Settings

If you want to pre-configure Goose with default settings without exposing your local `~/.config/goose` directory inside the container, create default settings files that are baked into the container image at workspace registration time. This is an alternative to mounting your local Goose settings — use one approach or the other, not both.

**Automatic Onboarding Skip**

When you register a workspace with the Goose agent, kdn automatically sets `GOOSE_TELEMETRY_ENABLED` to `false` in the Goose config file if it is not already defined, so Goose skips its telemetry prompt on first launch.

**Step 1: Create the agent settings directory**

```bash
mkdir -p ~/.kdn/config/goose/.config/goose
```

**Step 2: Write the default Goose settings file**

As an example, you can configure the model and enable telemetry:

```bash
cat > ~/.kdn/config/goose/.config/goose/config.yaml << 'EOF'
GOOSE_MODEL: "claude-sonnet-4-6"
GOOSE_TELEMETRY_ENABLED: true
EOF
```

**Fields:**

- `GOOSE_MODEL` - The model identifier Goose uses for its AI interactions. Alternatively, use `--model` flag during `init` to set this (the flag takes precedence over this setting)
- `GOOSE_TELEMETRY_ENABLED` - Whether Goose sends usage telemetry; set to `true` to opt in, or omit to have kdn default it to `false`

**Step 3: Register and start the workspace**

```bash
# Register a workspace — the settings file is embedded in the container image
kdn init /path/to/project --runtime podman --agent goose

# Start the workspace
kdn start my-project

# Connect — Goose starts with the configured provider and model
kdn terminal my-project
```

When `init` runs, kdn:
1. Reads all files from `~/.kdn/config/goose/` (e.g., your provider and model settings)
2. Automatically sets `GOOSE_TELEMETRY_ENABLED: false` in `.config/goose/config.yaml` if the key is not already defined
3. Copies the final settings into the container image at `/home/agent/.config/goose/config.yaml`

Goose finds this file on startup and uses the pre-configured settings without prompting.

**Notes:**

- **Telemetry is disabled automatically** — even if you don't create any settings files, kdn ensures Goose starts without the telemetry prompt
- If you prefer to enable telemetry, set `GOOSE_TELEMETRY_ENABLED: true` in `~/.kdn/config/goose/.config/goose/config.yaml`
- The settings are baked into the container image at `init` time, not mounted at runtime — changes to the files on the host require re-registering the workspace to take effect
- Any file placed under `~/.kdn/config/goose/` is copied into the container home directory, preserving the directory structure (e.g., `~/.kdn/config/goose/.config/goose/config.yaml` becomes `/home/agent/.config/goose/config.yaml` inside the container)
- This approach keeps your workspace self-contained — other developers using the same project are not affected, and your local `~/.config/goose` directory is not exposed inside the container
- To apply changes to the settings, remove and re-register the workspace: `kdn remove <workspace-id>` then `kdn init` again

### Using Cursor CLI Agent

This scenario demonstrates how to configure the Cursor agent in a kdn workspace, covering API key injection, sharing your local Cursor settings, and pre-configuring the default model.

#### Defining the Cursor API Key via a Secret

Cursor requires a `CURSOR_API_KEY` environment variable to authenticate with the Cursor service. Rather than embedding the key as plain text, use the secret mechanism to keep credentials out of your configuration files.

**Step 1: Create the secret**

For the **Podman runtime**, create the secret once on your host machine using `podman secret create`:

```bash
echo "$CURSOR_API_KEY" | podman secret create cursor-api-key -
```

**Step 2: Reference the secret in agent configuration**

Create or edit `~/.kdn/config/agents.json` to inject the secret as an environment variable for the `cursor` agent:

```json
{
  "cursor": {
    "environment": [
      {
        "name": "CURSOR_API_KEY",
        "secret": "cursor-api-key"
      }
    ]
  }
}
```

**Step 3: Register and start the workspace**

```bash
# Register a workspace with the Podman runtime and Cursor agent
kdn init /path/to/project --runtime podman --agent cursor

# Start the workspace
kdn start my-project

# Connect — Cursor starts with the API key available
kdn terminal my-project
```

The secret name (`cursor-api-key`) must match the `secret` field value in your configuration. At workspace creation time, kdn passes the secret to Podman, which injects it as the `CURSOR_API_KEY` environment variable inside the container.

#### Sharing Local Cursor Settings

To reuse your host Cursor settings (preferences, keybindings, extensions configuration, etc.) inside the workspace, mount the `~/.cursor` directory.

Edit `~/.kdn/config/agents.json` to add the mount:

```json
{
  "cursor": {
    "environment": [
      {
        "name": "CURSOR_API_KEY",
        "secret": "cursor-api-key"
      }
    ],
    "mounts": [
      {"host": "$HOME/.cursor", "target": "$HOME/.cursor"}
    ]
  }
}
```

The `~/.cursor` directory contains your Cursor configuration (settings, model preferences, etc.). It is mounted read-write so that changes made inside the workspace are persisted back to your host.

#### Using Default Settings

If you want to pre-configure Cursor with default settings without exposing your local `~/.cursor` directory inside the container, create default settings files that are baked into the container image at workspace registration time. This is an alternative to mounting your local Cursor settings — use one approach or the other, not both.

**Automatic Onboarding Skip**

When you register a workspace with the Cursor agent, kdn automatically creates a `.workspace-trusted` file in the Cursor projects directory for the workspace sources path, so Cursor skips its workspace trust dialog on first launch.

**Step 1: Configure the agent environment**

Create or edit `~/.kdn/config/agents.json` to inject the API key. No mount is needed since settings are baked in:

```json
{
  "cursor": {
    "environment": [
      {
        "name": "CURSOR_API_KEY",
        "secret": "cursor-api-key"
      }
    ]
  }
}
```

**Step 2: Create the agent settings directory**

```bash
mkdir -p ~/.kdn/config/cursor/.cursor
```

**Step 3: Write the default Cursor settings file (optional)**

You can optionally pre-configure Cursor with additional settings by creating a `cli-config.json` file:

```bash
cat > ~/.kdn/config/cursor/.cursor/cli-config.json << 'EOF'
{
  "model": {
    "modelId": "my-preferred-model",
    "displayModelId": "my-preferred-model",
    "displayName": "My Preferred Model",
    "displayNameShort": "My Model",
    "maxMode": false
  },
  "hasChangedDefaultModel": true
}
EOF
```

**Fields:**

- `model.modelId` - The model identifier used internally by Cursor
- `model.displayName` / `model.displayNameShort` - Human-readable model names shown in the UI
- `model.maxMode` - Whether to enable max mode for this model
- `hasChangedDefaultModel` - Tells Cursor that the model selection is intentional and should not prompt the user to choose a model

**Note:** Using the `--model` flag during `init` is the preferred way to configure the model, as it automatically sets all model fields correctly.

**Step 4: Register and start the workspace**

```bash
# Register a workspace with a specific model using the --model flag (recommended)
kdn init /path/to/project --runtime podman --agent cursor --model my-model-id

# Or register without --model to use settings from cli-config.json
kdn init /path/to/project --runtime podman --agent cursor

# Start the workspace
kdn start my-project

# Connect — Cursor starts with the configured model
kdn terminal my-project
```

When `init` runs, kdn:
1. Reads all files from `~/.kdn/config/cursor/` (e.g., your settings)
2. If `--model` is specified, updates `cli-config.json` with the model configuration (takes precedence over any existing model in settings files)
3. Automatically creates the workspace trust file so Cursor skips its trust dialog
4. Copies the final settings into the container image at `/home/agent/.cursor/cli-config.json`

Cursor finds this file on startup and uses the pre-configured model without prompting.

**Notes:**

- **Model configuration**: Use `--model` flag during `init` to set the model (e.g., `--model my-model-id`). This takes precedence over any model defined in settings files
- The settings are baked into the container image at `init` time, not mounted at runtime — changes to the files on the host require re-registering the workspace to take effect
- Any file placed under `~/.kdn/config/cursor/` is copied into the container home directory, preserving the directory structure (e.g., `~/.kdn/config/cursor/.cursor/cli-config.json` becomes `/home/agent/.cursor/cli-config.json` inside the container)
- To apply changes to the settings, remove and re-register the workspace: `kdn remove <workspace-id>` then `kdn init` again
- This approach keeps your workspace self-contained — other developers using the same project are not affected, and your local `~/.cursor` directory is not exposed inside the container
- Do not combine this approach with the `~/.cursor` mount from the previous section — the mounted directory would override the baked-in defaults at runtime

### Using OpenCode with a Local Model

OpenCode supports using locally-running models via providers like Ollama or RamaLama. This scenario demonstrates how to configure a kdn workspace to use a local model running on your host machine.

**Prerequisites:**

- A local model server running on your host (e.g., [Ollama](https://ollama.com) or [RamaLama](https://ramalama.ai))
- The model you want to use downloaded to your local server

**Step 1: Start a local model server on your host**

For example, with Ollama:

```bash
# Pull a model
ollama pull gemma3:12b

# Ollama runs as a service on port 11434 by default
```

Or with RamaLama:

```bash
# Serve a model (runs on port 8080 by default)
ramalama serve granite3.3:8b
```

**Step 2: Register the workspace with a local model**

Use the `--model` flag with the `provider::model` format. kdn knows the default endpoints for `ollama` and `ramalama` and automatically configures them to be reachable from inside the container:

```bash
# Use Ollama with a specific model (default endpoint: host.containers.internal:11434/v1 for Podman)
kdn init /path/to/project --runtime podman --agent opencode --model ollama::gemma3:12b

# Use RamaLama with a specific model (default endpoint: host.containers.internal:8080/v1 for Podman)
kdn init /path/to/project --runtime podman --agent opencode --model ramalama::granite3.3:8b
```

**Using a custom endpoint**

If your model server runs on a non-default port or a remote host, specify the full endpoint as the third component:

```bash
# Custom port on localhost (localhost is auto-converted to host.containers.internal for Podman)
kdn init /path/to/project --runtime podman --agent opencode --model ollama::gemma3:12b::http://localhost:8080/v1

# Remote host
kdn init /path/to/project --runtime podman --agent opencode --model ollama::gemma3:12b::http://192.168.1.50:11434/v1
```

When using the Podman runtime, localhost aliases (`localhost`, `127.0.0.1`, `0.0.0.0`, `::1`) in the base URL are automatically converted to `host.containers.internal` so the model server is reachable from inside the container.

**Using a custom OpenAI-compatible provider**

For any OpenAI-compatible model server not in the known provider list, use the three-part format with an explicit base URL:

```bash
kdn init /path/to/project --runtime podman --agent opencode --model myprovider::mymodel::http://localhost:9090/v1
```

**What kdn configures**

When you specify a local model provider, kdn writes an `opencode.json` configuration file baked into the container image. For `ollama::gemma3:12b` with the Podman runtime, it produces:

```json
{
  "model": "ollama/gemma3:12b",
  "provider": {
    "ollama": {
      "name": "ollama",
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://host.containers.internal:11434/v1"
      },
      "models": {
        "gemma3:12b": {
          "_launch": true,
          "name": "gemma3:12b"
        }
      }
    }
  }
}
```

**Step 3: Start and connect to the workspace**

```bash
# Start the workspace
kdn start my-project

# Connect — OpenCode starts using the local model automatically
kdn terminal my-project
```

**Notes:**

- The model server must be running on your host before connecting to the workspace
- The `provider::model` format stores the model as `provider/model` in the configuration (e.g., `ollama/gemma3:12b`)
- Known providers (`ollama`, `ramalama`) have preconfigured default base URLs; for other OpenAI-compatible providers, use the full `provider::model::baseURL` format
- When using the Podman runtime, the default base URLs for known providers point to `host.containers.internal`, which is the standard way to reach the host from a Podman container
- The settings are baked into the container image at `init` time — changes require re-registering the workspace: `kdn remove <workspace-id>` then `kdn init` again

### Sharing a GitHub Token

This scenario demonstrates how to make a GitHub token available inside workspaces using the multi-level configuration system — either globally for all projects or scoped to a specific project.

kdn has a built-in `github` secret service. The token is stored once with `kdn secret create` and referenced by name in any configuration level. At workspace creation time, kdn provisions the token into OneCLI, which injects it as a `Bearer` Authorization header for requests to `api.github.com`. It also sets `GH_TOKEN` and `GITHUB_TOKEN` as placeholder environment variables so that `gh` CLI and other GitHub-aware tools detect that credentials are configured.

**Step 1: Create the secret**

```bash
kdn secret create my-github-token --type github --value ghp_mytoken
```

The token is stored securely in the system keychain. The config files only hold the name.

**For all projects**

Edit `~/.kdn/config/projects.json` and add the secret name and your git configuration under the global `""` key:

```json
{
  "": {
    "secrets": ["my-github-token"],
    "mounts": [
      {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig", "ro": true}
    ]
  }
}
```

The `$HOME/.gitconfig` mount makes your git identity (name, email, aliases, etc.) available to git commands run by the agent.

**For a specific project**

Use the project identifier as the key instead. The identifier is the git remote URL (without `.git`) as detected by kdn during `init`:

```json
{
  "https://github.com/my-org/my-repo/": {
    "secrets": ["my-github-token"]
  }
}
```

This injects the token only when working on workspaces that belong to `https://github.com/my-org/my-repo/`, leaving other projects unaffected.

**Both at once**

If you need different tokens for different projects, create a secret for each and reference them per entry:

```bash
kdn secret create my-github-token-default --type github --value ghp_default
kdn secret create my-github-token-private --type github --value ghp_private
```

```json
{
  "": {
    "secrets": ["my-github-token-default"]
  },
  "https://github.com/my-org/my-private-repo/": {
    "secrets": ["my-github-token-private"]
  }
}
```

**Notes:**

- The token value never appears in configuration files — only the secret name does
- `gh` CLI and git will see `GH_TOKEN`/`GITHUB_TOKEN` set to a placeholder value, signalling that credentials are available; OneCLI injects the real token as a `Bearer` header on actual requests to `api.github.com`
- The project identifier used as the key must match what kdn detected during `init` — run `kdn list -o json` to see the project field for each registered workspace
- Configuration changes in `projects.json` take effect the next time you run `kdn init` for that workspace; already-registered workspaces need to be removed and re-registered

### Protecting an OpenShift Token

> **Note:** This is a temporary workaround. Native support for OpenShift token injection will be added to the `kdn init` command in a future release.

This scenario demonstrates how to connect to an OpenShift cluster from inside a workspace without embedding your real token in the kubeconfig file. The token is stored securely with `kdn secret create` and injected by OneCLI as a `Bearer` Authorization header on every request to the OpenShift API server.

**Prerequisites:**

- The `oc` CLI installed on your host machine
- An OpenShift cluster reachable from your host

**Step 1: Log in to the cluster with a dedicated kubeconfig**

Use the `--kubeconfig` flag to write the credentials to a separate file that will be shared with the workspace, rather than modifying your default `~/.kube/config`:

```bash
oc login --kubeconfig=$HOME/kubeconfig-agent \
  --token=sha256~<real-token> \
  --server=https://my-openshift-server:6443
```

**Step 2: Replace the real token with a placeholder**

Edit `$HOME/kubeconfig-agent` and replace `sha256~<real-token>` with `sha256-placeholder`. The file is now safe to mount read-only into workspaces because it contains no real credentials.

**Step 3: Create the kdn secret**

Store the real token once in the system keychain:

```bash
kdn secret create oc \
  --type other \
  --header Authorization \
  --headerTemplate 'Bearer ${value}' \
  --host my-openshift-server \
  --value sha256~<real-token>
```

This tells kdn to instruct OneCLI to inject `Authorization: Bearer sha256~<real-token>` on every outbound request to `my-openshift-server`, replacing the placeholder in the kubeconfig.

**Step 4: Declare the secret and mount the kubeconfig**

Add the following to your `.kaiden/workspace.json`:

```json
{
  "secrets": ["oc"],
  "mounts": [
    {
      "host": "$HOME/kubeconfig-agent",
      "target": "$HOME/.kube/config",
      "ro": true
    }
  ]
}
```

The `secrets` entry tells kdn to provision the `oc` secret into OneCLI when the workspace starts. The mount makes the placeholder kubeconfig available at the standard `~/.kube/config` path inside the container.

**Step 5: Register and start the workspace**

```bash
# Register a workspace
kdn init /path/to/project --runtime podman --agent claude

# Start the workspace
kdn start my-project

# Connect — oc and kubectl commands reach the cluster via OneCLI
kdn terminal my-project
```

**Notes:**

- The real token never appears in `workspace.json` or any config file — only the secret name does
- OneCLI intercepts outbound HTTPS requests to `my-openshift-server` and injects the real `Authorization` header, so `oc` and `kubectl` work transparently inside the container
- The placeholder value (`sha256-placeholder`) in the kubeconfig must differ from the real token so that the kubeconfig file is safe to share or commit; OneCLI replaces it at the network level

### Working with Git Worktrees

This scenario demonstrates how to run multiple agents in parallel, each working on a different branch of the same repository. Git worktrees allow each branch to live in its own directory, so each agent gets its own isolated workspace.

**Step 1: Clone the repository**

```bash
git clone https://github.com/my-org/my-repo.git /path/to/my-project/main
```

**Step 2: Create a worktree for each feature branch**

```bash
cd /path/to/my-project/main

git worktree add ../feature-a feature-a
git worktree add ../feature-b feature-b
```

This results in the following layout:

```text
/path/to/my-project/
├── main/       ← main branch (original clone)
├── feature-a/  ← feature-a branch (worktree)
└── feature-b/  ← feature-b branch (worktree)
```

**Step 3: Configure the main branch mount in your local project config**

If you want the agents to have access to the main branch (e.g., to compare changes), add the mount in `~/.kdn/config/projects.json` under the project identifier. This keeps the configuration on your machine only — not all developers of the project may use worktrees, so it does not belong in the repository's `.kaiden/workspace.json`.

```json
{
  "https://github.com/my-org/my-repo/": {
    "mounts": [
      {"host": "$SOURCES/../main", "target": "$SOURCES/../main"}
    ]
  }
}
```

`$SOURCES` expands to the workspace sources directory (e.g., `/path/to/my-project/feature-a`), so `$SOURCES/../main` resolves to `/path/to/my-project/main` on both the host and inside the container.

**Step 4: Register a workspace for each worktree**

```bash
kdn init /path/to/my-project/feature-a --runtime podman --agent claude
kdn init /path/to/my-project/feature-b --runtime podman --agent claude
```

**Step 5: Start and connect to each workspace independently**

```bash
# Start both workspaces (using names or IDs)
kdn start feature-a
kdn start feature-b

# Connect to each agent in separate terminals
kdn terminal feature-a
kdn terminal feature-b
```

Each agent runs independently in its own container, operating on its own branch without interfering with the other.

**Notes:**

- Each worktree shares the same `.git` directory, so agents can run git commands that are branch-aware
- Workspaces for different worktrees of the same repository share the same project identifier (derived from the git remote URL), so the mount defined in `projects.json` automatically applies to all of them

### Managing Workspaces from a UI or Programmatically

This scenario demonstrates how to manage workspaces programmatically using JSON output, which is ideal for UIs, scripts, or automation tools. All commands support the `--output json` (or `-o json`) flag for machine-readable output.

**Step 1: Check existing workspaces**

```bash
$ kdn workspace list -o json
```

```json
{
  "items": []
}
```

Exit code: `0` (success, but no workspaces registered)

**Step 2: Register a new workspace**

```bash
$ kdn init /path/to/project --runtime podman --agent claude -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 3: Register with verbose output to get full details**

```bash
$ kdn init /path/to/another-project --runtime podman --agent claude --model claude-sonnet-4-20250514 -o json -v
```

```json
{
  "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
  "name": "another-project",
  "agent": "claude",
  "model": "claude-sonnet-4-20250514",
  "project": "/absolute/path/to/another-project",
  "state": "stopped",
  "paths": {
    "source": "/absolute/path/to/another-project",
    "configuration": "/absolute/path/to/another-project/.kaiden"
  },
  "timestamps": {
    "created": 1752912000000
  }
}
```

Exit code: `0` (success)

**Step 3a: Register and start immediately with auto-start flag**

```bash
$ kdn init /path/to/third-project --runtime podman --agent claude -o json --start
```

```json
{
  "id": "3c4d5e6f7a8b9098765432109876543210987654321098765432109876543210b"
}
```

Exit code: `0` (success, workspace is running)

**Step 4: List all workspaces**

```bash
$ kdn workspace list -o json
```

```json
{
  "items": [
    {
      "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea",
      "name": "project",
      "agent": "claude",
      "project": "/absolute/path/to/project",
      "state": "running",
      "paths": {
        "source": "/absolute/path/to/project",
        "configuration": "/absolute/path/to/project/.kaiden"
      },
      "timestamps": {
        "created": 1752912000000,
        "started": 1752912300000
      }
    },
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
      "agent": "claude",
      "model": "claude-sonnet-4-20250514",
      "project": "/absolute/path/to/another-project",
      "state": "stopped",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kaiden"
      },
      "timestamps": {
        "created": 1752912000000
      }
    }
  ]
}
```

Exit code: `0` (success)

**Step 5: Start a workspace**

```bash
$ kdn workspace start 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 6: Stop a workspace**

```bash
$ kdn workspace stop 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 7: Remove a workspace**

```bash
$ kdn workspace remove 2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea -o json
```

```json
{
  "id": "2c5f16046476be368fcada501ac6cdc6bbd34ea80eb9ceb635530c0af64681ea"
}
```

Exit code: `0` (success)

**Step 8: Verify removal**

```bash
$ kdn workspace list -o json
```

```json
{
  "items": [
    {
      "id": "f6e5d4c3b2a1098765432109876543210987654321098765432109876543210a",
      "name": "another-project",
      "agent": "claude",
      "model": "claude-sonnet-4-20250514",
      "project": "/absolute/path/to/another-project",
      "state": "stopped",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kaiden"
      },
      "timestamps": {
        "created": 1752912000000
      }
    }
  ]
}
```

Exit code: `0` (success)

#### Error Handling

All errors are returned in JSON format when using `--output json`, with the error written to **stdout** (not stderr) and a non-zero exit code.

**Error: Non-existent directory**

```bash
$ kdn init /tmp/no-exist --runtime podman --agent claude -o json
```

```json
{
  "error": "sources directory does not exist: /tmp/no-exist"
}
```

Exit code: `1` (error)

**Error: Workspace not found**

```bash
$ kdn workspace remove unknown-id -o json
```

```json
{
  "error": "workspace not found: unknown-id"
}
```

Exit code: `1` (error)

#### Best Practices for Programmatic Usage

1. **Always check the exit code** to determine success (0) or failure (non-zero)
2. **Parse stdout** for JSON output in both success and error cases
3. **Use verbose mode** with init (`-v`) when you need full workspace details immediately after creation
4. **Handle both success and error JSON structures** in your code:
   - Success responses have specific fields (e.g., `id`, `items`, `name`, `paths`)
   - Error responses always have an `error` field

**Example script pattern:**

```bash
#!/bin/bash

# Register a workspace
output=$(kdn init /path/to/project --runtime podman --agent claude -o json)
exit_code=$?

if [ $exit_code -eq 0 ]; then
    workspace_id=$(echo "$output" | jq -r '.id')
    echo "Workspace created: $workspace_id"
else
    error_msg=$(echo "$output" | jq -r '.error')
    echo "Error: $error_msg"
    exit 1
fi
```

## Environment Variables

kdn supports environment variables for configuring default behavior.

### `KDN_DEFAULT_RUNTIME`

Sets the default runtime to use when registering a workspace with the `init` command.

**Usage:**

```bash
export KDN_DEFAULT_RUNTIME=fake
kdn init /path/to/project --agent claude
```

**Priority:**

The runtime is determined in the following order (highest to lowest priority):

1. `--runtime` flag (if specified)
2. `KDN_DEFAULT_RUNTIME` environment variable (if set)
3. Error if neither is set (runtime is required)

**Example:**

```bash
# Set the default runtime for the current shell session
export KDN_DEFAULT_RUNTIME=fake

# Register a workspace using the environment variable
kdn init /path/to/project --agent claude

# Override the environment variable with the flag
kdn init /path/to/another-project --agent claude --runtime podman
```

**Notes:**

- The runtime parameter is mandatory when registering workspaces
- If neither the flag nor the environment variable is set, the `init` command will fail with an error
- Supported runtime types depend on the available runtime implementations
- Setting this environment variable is useful for automation scripts or when you consistently use the same runtime

### `KDN_DEFAULT_AGENT`

Sets the default agent to use when registering a workspace with the `init` command.

**Usage:**

```bash
export KDN_DEFAULT_AGENT=claude
kdn init /path/to/project --runtime podman
```

**Priority:**

The agent is determined in the following order (highest to lowest priority):

1. `--agent` flag (if specified)
2. `KDN_DEFAULT_AGENT` environment variable (if set)
3. Error if neither is set (agent is required)

**Example:**

```bash
# Set the default agent for the current shell session
export KDN_DEFAULT_AGENT=claude

# Register a workspace using the environment variable
kdn init /path/to/project --runtime podman

# Override the environment variable with the flag
kdn init /path/to/another-project --runtime podman --agent goose
```

**Notes:**

- The agent parameter is mandatory when registering workspaces
- If neither the flag nor the environment variable is set, the `init` command will fail with an error
- Supported agent types depend on the available agent configurations in the runtime
- Agent names must contain only alphanumeric characters or underscores (e.g., `claude`, `goose`, `my_agent`)
- Setting this environment variable is useful for automation scripts or when you consistently use the same agent

### `KDN_STORAGE`

Sets the default storage directory where kdn stores its data files.

**Usage:**

```bash
export KDN_STORAGE=/custom/path/to/storage
kdn init /path/to/project --runtime podman --agent claude
```

**Priority:**

The storage directory is determined in the following order (highest to lowest priority):

1. `--storage` flag (if specified)
2. `KDN_STORAGE` environment variable (if set)
3. Default: `$HOME/.kdn`

**Example:**

```bash
# Set a custom storage directory
export KDN_STORAGE=/var/lib/kaiden

# All commands will use this storage directory
kdn init /path/to/project --runtime podman --agent claude
kdn list

# Override the environment variable with the flag
kdn list --storage /tmp/kaiden-storage
```

### `KDN_INIT_AUTO_START`

Automatically starts a workspace after registration when using the `init` command.

**Usage:**

```bash
export KDN_INIT_AUTO_START=1
kdn init /path/to/project --runtime podman --agent claude
```

**Priority:**

The auto-start behavior is determined in the following order (highest to lowest priority):

1. `--start` flag (if specified)
2. `KDN_INIT_AUTO_START` environment variable (if set to a truthy value)
3. Default: workspace is not started automatically

**Supported Values:**

The environment variable accepts the following truthy values (case-insensitive):
- `1`
- `true`, `True`, `TRUE`
- `yes`, `Yes`, `YES`

Any other value (including `0`, `false`, `no`, or empty string) will not trigger auto-start.

**Example:**

```bash
# Set auto-start for the current shell session
export KDN_INIT_AUTO_START=1

# Register and start a workspace automatically
kdn init /path/to/project --runtime podman --agent claude
# Workspace is now running

# Override the environment variable with the flag
export KDN_INIT_AUTO_START=0
kdn init /path/to/another-project --runtime podman --agent claude --start
# Workspace is started despite env var being 0
```

**Notes:**

- Auto-starting combines the `init` and `start` commands into a single operation
- Useful for automation scripts where you want workspaces ready to use immediately
- If the workspace fails to start, the registration still succeeds, but an error is returned
- The `--start` flag always takes precedence over the environment variable

### `KDN_AUTOCOMPLETE_IGNORE_IDS`

Hides workspace IDs from shell autocompletion, so only names are suggested.

By default, commands like `kdn start`, `kdn stop`, and `kdn remove` autocomplete both workspace IDs and names. If only one workspace exists, the shell cannot complete the argument immediately because there are two candidates (ID and name). Setting `KDN_AUTOCOMPLETE_IGNORE_IDS` removes IDs from the suggestions, allowing instant completion when a single workspace is registered.

**Usage:**

```bash
export KDN_AUTOCOMPLETE_IGNORE_IDS=1
kdn start <TAB>   # suggests names only
```

**Supported Values:**

The environment variable accepts the following truthy values (case-insensitive):
- `1`
- `true`, `True`, `TRUE`
- `yes`, `Yes`, `YES`

Any other value (including `0`, `false`, `no`, or empty string) keeps the default behaviour of suggesting both IDs and names.

**Example:**

```bash
# Show only names during tab-completion
export KDN_AUTOCOMPLETE_IGNORE_IDS=1
kdn start <TAB>      # completes to the workspace name immediately if only one exists
kdn stop <TAB>
kdn remove <TAB>

# Restore default behaviour (show IDs and names)
unset KDN_AUTOCOMPLETE_IGNORE_IDS
```

## Podman Runtime

The Podman runtime provides a container-based development environment for workspaces. It creates an isolated environment with all necessary tools pre-installed and configured.

### Container Image

**Base Image:** `registry.fedoraproject.org/fedora:latest`

The Podman runtime builds a custom container image based on Fedora Linux, providing a stable and up-to-date foundation for development work.

### Installed Packages

The runtime includes a comprehensive development toolchain:

- **Core Utilities:**
  - `which` - Command location utility
  - `procps-ng` - Process management utilities
  - `wget2` - Advanced file downloader

- **Development Tools:**
  - `@development-tools` - Complete development toolchain (gcc, make, etc.)
  - `jq` - JSON processor
  - `gh` - GitHub CLI

- **Language Support:**
  - `golang` - Go programming language
  - `golangci-lint` - Go linter
  - `python3` - Python 3 interpreter
  - `python3-pip` - Python package manager

### User and Permissions

The container runs as a non-root user named `agent` with the following configuration:

- **User:** `agent`
- **UID/GID:** Matches the host user's UID and GID for seamless file permissions
- **Home Directory:** `/home/agent`

**Sudo Permissions:**

The `agent` user has limited sudo access with no password required (`NOPASSWD`) for:

- **Package Management:**
  - `/usr/bin/dnf` - Install, update, and manage packages

- **Process Management:**
  - `/bin/nice` - Run programs with modified scheduling priority
  - `/bin/kill`, `/usr/bin/kill` - Send signals to processes
  - `/usr/bin/killall` - Kill processes by name

All other sudo commands are explicitly denied for security.

### AI Agents

The Podman runtime includes default configurations for the following AI agents:

**Claude Code** - Installed using the official installation script from `claude.ai/install.sh`:
- Full Claude Code CLI capabilities
- Integrated development assistance
- Access to Claude's latest features

**Goose** - Installed using the official installer from `github.com/block/goose`:
- AI-powered development agent
- Task automation and code assistance
- Configurable development workflows

**Cursor** - Installed using the official installer from `cursor.com/install`:
- AI-powered code editor agent
- Configurable development workflows

**OpenCode** - Installed using the official installer from `opencode.ai/install`:
- Open-source AI coding agent
- The installer places the binary in `~/.opencode/bin/`, which is symlinked into `~/.local/bin/` for PATH access

The agent runs within the container environment and has access to the mounted workspace sources and dependencies.

### Working Directory

The container's working directory is set to `/workspace/sources`, which is where your project source code is mounted. This ensures that the agent and all tools operate within your project context.

### Example Usage

```bash
# Register a workspace with the Podman runtime
kdn init /path/to/project --runtime podman --agent claude
```

**User Experience:**

When you register a workspace with the Podman runtime, you'll see progress feedback for each operation:

```text
⠋ Creating temporary build directory
✓ Temporary build directory created
⠋ Generating Containerfile
✓ Containerfile generated
⠋ Building container image: kdn-myproject
✓ Container image built
⠋ Creating container: myproject
✓ Container created
```

The `init` command will:
1. Create a temporary build directory - **with progress spinner**
2. Generate a Containerfile with the configuration above - **with progress spinner**
3. Build a custom image (tagged as `kdn-<workspace-name>`) - **with progress spinner**
4. Create a container with your source code mounted - **with progress spinner**

After registration, you can start the workspace:

```bash
# Start the workspace
kdn start <workspace-id>
```

**Note:** When using `--output json`, all progress spinners are hidden to avoid polluting the JSON output.

### Customizing Podman Runtime Configuration

The Podman runtime is fully configurable through JSON files. When you first use the Podman runtime, default configuration files are automatically created in your storage directory.

**Configuration Location:**

```text
$HOME/.kdn/runtimes/podman/config/
├── image.json      # Base image configuration
├── claude.json     # Claude agent configuration
├── goose.json      # Goose agent configuration
└── opencode.json   # OpenCode agent configuration
```

Or if using a custom storage directory:

```text
<storage-dir>/runtimes/podman/config/
```

#### Base Image Configuration (`image.json`)

Controls the container's base image, packages, and sudo permissions.

**Structure:**

```json
{
  "version": "latest",
  "packages": [
    "which",
    "procps-ng",
    "wget2",
    "@development-tools",
    "jq",
    "gh",
    "golang",
    "golangci-lint",
    "python3",
    "python3-pip"
  ],
  "sudo": [
    "/usr/bin/dnf",
    "/bin/nice",
    "/bin/kill",
    "/usr/bin/kill",
    "/usr/bin/killall"
  ],
  "run_commands": []
}
```

**Fields:**

- `version` (required) - Fedora version tag
  - Examples: `"latest"`, `"40"`, `"41"`
  - The base registry `registry.fedoraproject.org/fedora` is hardcoded and cannot be changed

- `packages` (optional) - DNF packages to install
  - Array of package names
  - Can include package groups with `@` prefix (e.g., `"@development-tools"`)
  - Empty array is valid if no packages needed

- `sudo` (optional) - Binaries the `agent` user can run with sudo
  - Must be absolute paths (e.g., `"/usr/bin/dnf"`)
  - Creates a single `ALLOWED` command alias in sudoers
  - Empty array disables all sudo access

- `run_commands` (optional) - Custom shell commands to run during image build
  - Executed as RUN instructions in the Containerfile
  - Run before agent-specific commands
  - Useful for additional setup steps

#### Agent Configuration

Controls agent-specific packages and installation steps. The Podman runtime provides default configurations for Claude Code (`claude.json`), Goose (`goose.json`), Cursor (`cursor.json`), and OpenCode (`opencode.json`).

**Structure (claude.json):**

```json
{
  "packages": [],
  "run_commands": [
    "curl -fsSL --proto-redir '-all,https' --tlsv1.3 https://claude.ai/install.sh | bash",
    "mkdir -p /home/agent/.config"
  ],
  "terminal_command": [
    "claude"
  ]
}
```

**Structure (goose.json):**

```json
{
  "packages": [],
  "run_commands": [
    "cd /tmp && curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | CONFIGURE=false bash"
  ],
  "terminal_command": [
    "goose"
  ]
}
```

**Structure (opencode.json):**

```json
{
  "packages": [],
  "run_commands": [
    "cd /tmp && curl -fsSL https://opencode.ai/install | bash",
    "mkdir -p /home/agent/.local/bin && ln -sf /home/agent/.opencode/bin/opencode /home/agent/.local/bin/opencode",
    "mkdir -p /home/agent/.config/opencode"
  ],
  "terminal_command": [
    "opencode"
  ]
}
```

**Fields:**

- `packages` (optional) - Additional packages specific to this agent
  - Merged with packages from `image.json`
  - Useful for agent-specific dependencies

- `run_commands` (optional) - Commands to set up the agent
  - Executed after image configuration commands
  - Typically used for agent installation

- `terminal_command` (required) - Command to launch the agent
  - Must have at least one element
  - Can include flags: `["claude", "--verbose"]`

#### Applying Configuration Changes

Configuration changes take effect when you **register a new workspace with `init`**. The Containerfile is generated and the image is built during workspace registration, using the configuration files that exist at that time.

**To apply new configuration:**

1. Edit the configuration files:
   ```bash
   # Edit base image configuration
   nano ~/.kdn/runtimes/podman/config/image.json

   # Edit agent configuration (use the agent you want)
   nano ~/.kdn/runtimes/podman/config/claude.json
   # or
   nano ~/.kdn/runtimes/podman/config/goose.json
   ```

2. Register a new workspace (this creates the Containerfile and builds the image):
   ```bash
   # Using Claude agent
   kdn init /path/to/project --runtime podman --agent claude
   # or using Goose agent
   kdn init /path/to/project --runtime podman --agent goose
   ```

3. Start the workspace:
   ```bash
   kdn start <workspace-id>
   ```

**Notes:**

- The first `init` command using Podman creates default config files automatically
- Config files are never overwritten once created - your customizations are preserved
- The Containerfile and image are built during `init`, not `start`
- Each workspace's image is built once using the configuration at registration time
- To rebuild a workspace with new config, remove and re-register it
- Validation errors in config files will cause workspace registration to fail with a descriptive message
- The generated Containerfile is automatically copied to `/home/agent/Containerfile` inside the container for reference

## Workspace Configuration

Each workspace can optionally include a configuration file that customizes the environment, mount, and skills behavior for that specific workspace. The configuration is stored in a `workspace.json` file within the workspace's configuration directory (typically `.kaiden` in the sources directory).

### Configuration File Location

By default, workspace configuration is stored at:
```text
<sources-directory>/.kaiden/workspace.json
```

The configuration directory (containing `workspace.json`) can be customized using the `--workspace-configuration` flag when registering a workspace with `init`. The flag accepts a directory path, not the file path itself.

### Configuration Structure

The `workspace.json` file uses a nested JSON structure:

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
  "mounts": [
    {"host": "$SOURCES/../main", "target": "$SOURCES/../main"},
    {"host": "$HOME/.ssh", "target": "$HOME/.ssh"},
    {"host": "/absolute/path/to/data", "target": "/workspace/data"}
  ],
  "skills": [
    "/absolute/path/to/commit-skill",
    "$HOME/review-skill"
  ],
  "mcp": {
    "commands": [
      {
        "name": "my-local-tool",
        "command": "python3",
        "args": ["/workspace/sources/scripts/mcp_server.py"],
        "env": {"DEBUG": "true"}
      }
    ],
    "servers": [
      {
        "name": "remote-api",
        "url": "https://api.example.com/mcp",
        "headers": {"Authorization": "Bearer token123"}
      }
    ]
  },
  "network": {
    "mode": "deny",
    "hosts": ["api.github.com"]
  },
  "secrets": ["my-github-token", "my-api-key"],
  "features": {
    "ghcr.io/devcontainers/features/go:1": {"version": "1.23"},
    "./tools/my-feature": {}
  }
}
```

### Environment Variables

Define environment variables that will be set in the workspace runtime environment.

**Structure:**
```json
{
  "environment": [
    {
      "name": "VAR_NAME",
      "value": "hardcoded-value"
    },
    {
      "name": "SECRET_VAR",
      "secret": "secret-reference"
    }
  ]
}
```

**Fields:**
- `name` (required) - Environment variable name
  - Must be a valid Unix environment variable name
  - Must start with a letter or underscore
  - Can contain letters, digits, and underscores
- `value` (optional) - Hardcoded value for the variable
  - Mutually exclusive with `secret`
  - Empty strings are allowed
- `secret` (optional) - Reference to a runtime secret (e.g., a Podman secret) containing the value; the runtime injects it as an environment variable inside the workspace
  - Mutually exclusive with `value`
  - Cannot be empty
  - Use this when a local tool inside the workspace needs the credential via an environment variable
  - For credentials used in outbound network requests, use the `secrets` list field and `kdn secret create` instead — those are injected as HTTP headers by OneCLI

**Validation Rules:**
- Variable name cannot be empty
- Exactly one of `value` or `secret` must be defined
- Variable names must follow Unix conventions (e.g., `DEBUG`, `API_KEY`, `MY_VAR_123`)
- Invalid names include those starting with digits (`1INVALID`) or containing special characters (`INVALID-NAME`, `INVALID@NAME`)

### Mount Paths

Configure additional directories to mount in the workspace runtime.

**Structure:**
```json
{
  "mounts": [
    {"host": "$SOURCES/../main", "target": "$SOURCES/../main"},
    {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"},
    {"host": "/absolute/path/to/data", "target": "/workspace/data", "ro": true}
  ]
}
```

**Fields:**
- `host` (required) - Path on the host filesystem to mount
- `target` (required) - Path inside the container where the host path is mounted
- `ro` (optional) - Mount as read-only (default: `false`)

**Path Variables:**

Both `host` and `target` support the following variables:
- `$SOURCES` - Expands to the workspace sources directory on the host, or `/workspace/sources` in the container
- `$HOME` - Expands to the user's home directory on the host, or `/home/agent` in the container

Paths can also be absolute (e.g., `/absolute/path`).

**Validation Rules:**
- `host` and `target` cannot be empty
- Each path must be absolute or start with `$SOURCES` or `$HOME`
- `$SOURCES`-based container targets must not escape above `/workspace`
- `$HOME`-based container targets must not escape above `/home/agent`

### Skills

Configure skill directories to make available to the agent inside the workspace.

Each entry is a path to a directory on the host that contains a single skill — a `SKILL.md` file and any related files. The directory is mounted read-only inside the agent's skills directory using the directory's basename as the skill name, allowing the agent to discover and use it.

**Structure:**
```json
{
  "skills": [
    "/absolute/path/to/commit-skill",
    "$HOME/review-skill"
  ]
}
```

**Fields:**
- Each entry is a path to a host directory containing a single skill (`SKILL.md` and related files)

**Path Variables:**

Skills paths support the following variables:
- `$HOME` - Expands to the user's home directory on the host

Paths can also be absolute (e.g., `/absolute/path/to/commit-skill`).

**Mount targets per agent:**

Each skill directory is mounted read-only under the agent's skills directory inside the container. The subdirectory name matches the basename of the host path:

| Agent | Mount target |
|-------|-------------|
| Claude Code | `~/.claude/skills/<basename>/` |
| Goose | `~/.agents/skills/<basename>/` |
| Cursor | `~/.cursor/skills/<basename>/` |
| OpenCode | `~/.opencode/skills/<basename>/` |

For example, a skills path of `/home/user/commit-skill` is mounted at `~/.claude/skills/commit-skill/` for Claude Code, making the skill discoverable by the agent.

**Validation Rules:**
- Each path cannot be empty
- Each path must be an absolute path or start with `$HOME`
- `$SOURCES`-based paths are not supported for skills

### MCP Servers

Configure MCP (Model Context Protocol) servers to give the agent access to external tools and data sources. Two types are supported:

- **Commands** — local MCP servers launched by the agent inside the workspace using stdio transport
- **Servers** — remote MCP servers accessed over SSE (Server-Sent Events)

**Structure:**
```json
{
  "mcp": {
    "commands": [
      {
        "name": "my-tool",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace/sources"],
        "env": {"NODE_ENV": "production"}
      }
    ],
    "servers": [
      {
        "name": "remote-api",
        "url": "https://api.example.com/mcp",
        "headers": {"Authorization": "Bearer token123"}
      }
    ]
  }
}
```

**Command fields** (`commands[*]`):
- `name` (required) - Unique name for this MCP server
- `command` (required) - Executable to run (e.g., `npx`, `python3`, `node`)
- `args` (optional) - Arguments to pass to the command
- `env` (optional) - Environment variables to set for the process

**Server fields** (`servers[*]`):
- `name` (required) - Unique name for this MCP server
- `url` (required) - SSE endpoint URL of the remote MCP server
- `headers` (optional) - HTTP headers to include in requests (e.g., `Authorization`)

**Agent support:**

MCP server configuration is applied to agents that support it at workspace registration time. For **Claude Code**, both command-based and URL-based MCP servers are written to `~/.claude.json` under the top-level `mcpServers` key (user scope), so they are available across all projects inside the workspace.

**Validation Rules:**
- `name` cannot be empty and must be unique across **both** `commands` and `servers` combined — a command and a server cannot share the same name, since all entries map to the same flat `mcpServers` key in the agent settings
- `command` cannot be empty for command entries
- `url` cannot be empty for server entries

### Network Access

Control network access for the workspace. By default, network access is denied (deny mode). You can allow all network access or restrict it to specific hosts.

**Structure:**
```json
{
  "network": {
    "mode": "deny",
    "hosts": ["example.com", "api.github.com"]
  }
}
```

**Fields:**
- `mode` (optional) - Network access mode
  - `"allow"` - Permits all network access (no restrictions)
  - `"deny"` - Blocks all outbound network access from the workspace agent, except for the hosts listed in `hosts` and the hosts associated with configured secrets
- `hosts` (optional) - List of hostnames to allow when in deny mode
  - Only meaningful when mode is `"deny"`
  - Each entry must be a non-empty string
  - Omitting `hosts` (or leaving it empty) is valid: the workspace is fully isolated, with no outbound access permitted unless secrets contribute hosts

**Automatic secret host injection:** When `mode` is `"deny"` and secrets are configured, kdn automatically adds the hosts associated with those secrets to the allowed list. You do not need to list them explicitly under `hosts`. For example, a `github` secret automatically allows `api.github.com` without any `hosts` entry.

**Validation Rules:**
- If `mode` is set, it must be either `"allow"` or `"deny"`
- If `mode` is `"allow"`, `hosts` must not be set (they are meaningless in allow mode)
- Host entries cannot be empty strings

### Secrets

Configure secrets to inject into the workspace. Each entry is the name of a secret previously created with `kdn secret create`. At workspace creation time, kdn looks up the secret value from the system keychain and provisions it into the workspace via OneCLI, which injects it as an HTTP header into matching outbound requests. This is distinct from the `secret` field in environment variables, which references runtime secrets by name for environment variable injection.

When `network.mode` is `"deny"`, the hosts associated with each secret are automatically added to the allowed list — you do not need to duplicate them under `network.hosts`.

**Structure:**
```json
{
  "secrets": ["my-github-token", "my-api-key"]
}
```

**Fields:**
- Each entry is a secret name (string) referencing a secret stored with `kdn secret create`

**Validation Rules:**
- Secret names cannot be empty
- Duplicate names within the list are rejected

### Dev Container Features

Install [Dev Container Features](https://containers.dev/implementors/features/) into the workspace image at build time. Features are reusable environment components that add languages, runtimes, and tools to your workspace.

**Structure:**
```json
{
  "features": {
    "<feature-id>": {},
    "<feature-id>": {"<option>": "<value>"}
  }
}
```

Each key is a feature ID — either an OCI reference (`ghcr.io/org/repo/feature:tag`) or a relative path to a local directory (`./my-feature`). Each value is a map of options that override the feature's defaults; use an empty object `{}` to accept all defaults.

**Fields:**
- Feature ID (required) — OCI reference or relative path to a local directory
- Options (required, can be empty) — key/value pairs that customise the feature

**Validation Rules:**
- Feature IDs must be OCI references or relative paths (`./…`); `https://` tarball URIs are not supported
- Local paths are resolved relative to the workspace configuration directory (e.g. `.kaiden/`)

**Example — install Go and Node.js:**
```json
{
  "features": {
    "ghcr.io/devcontainers/features/go:1": {"version": "1.23"},
    "ghcr.io/devcontainers/features/node:1": {"version": "20"}
  }
}
```

**Example — use a local feature:**
```json
{
  "features": {
    "./tools/my-feature": {}
  }
}
```

### Configuration Validation

When you register a workspace with `kdn init`, the configuration is automatically validated. If `workspace.json` exists and contains invalid data, the registration will fail with a descriptive error message.

**Example - Invalid configuration (both value and secret set):**
```bash
$ kdn init /path/to/project --runtime podman --agent claude
```
```text
Error: workspace configuration validation failed: invalid workspace configuration:
environment variable "API_KEY" (index 0) has both value and secret set
```

**Example - Invalid configuration (missing host in mount):**
```bash
$ kdn init /path/to/project --runtime podman --agent claude
```
```text
Error: workspace configuration validation failed: invalid workspace configuration:
mount at index 0 is missing host
```

### Configuration Examples

**Basic environment variables:**
```json
{
  "environment": [
    {
      "name": "NODE_ENV",
      "value": "development"
    },
    {
      "name": "DEBUG",
      "value": "true"
    }
  ]
}
```

**Using secrets:**
```json
{
  "environment": [
    {
      "name": "API_TOKEN",
      "secret": "github-api-token"
    }
  ]
}
```

**git worktree:**
```json
{
  "mounts": [
    {"host": "$SOURCES/../main", "target": "$SOURCES/../main"}
  ]
}
```

**Sharing user configurations:**
```json
{
  "mounts": [
    {"host": "$HOME/.claude", "target": "$HOME/.claude"},
    {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"},
    {"host": "$HOME/.kube/config", "target": "$HOME/.kube/config", "ro": true}
  ]
}
```

**MCP command server (local tool):**
```json
{
  "mcp": {
    "commands": [
      {
        "name": "filesystem",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace/sources"]
      }
    ]
  }
}
```

**MCP remote server with authentication:**
```json
{
  "mcp": {
    "servers": [
      {
        "name": "company-api",
        "url": "https://mcp.company.com/sse",
        "headers": {"Authorization": "Bearer mytoken"}
      }
    ]
  }
}
```

**Network access - allow all:**
```json
{
  "network": {
    "mode": "allow"
  }
}
```

**Network access - deny with exceptions:**
```json
{
  "network": {
    "mode": "deny",
    "hosts": ["api.github.com", "registry.npmjs.org"]
  }
}
```

**Network access - fully isolated (deny, no hosts):**
```json
{
  "network": {
    "mode": "deny"
  }
}
```

**Network access - deny with secrets (hosts inferred automatically):**
```json
{
  "network": {
    "mode": "deny"
  },
  "secrets": ["my-github-token"]
}
```

The `my-github-token` secret (type `github`) automatically allows `api.github.com` without any `hosts` entry.

**Secrets:**
```json
{
  "secrets": ["my-github-token", "my-internal-api"]
}
```

**Complete configuration:**
```json
{
  "environment": [
    {
      "name": "NODE_ENV",
      "value": "development"
    },
    {
      "name": "DATABASE_URL",
      "secret": "local-db-url"
    }
  ],
  "mounts": [
    {"host": "$SOURCES/../main", "target": "$SOURCES/../main"},
    {"host": "$HOME/.claude", "target": "$HOME/.claude"},
    {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"}
  ],
  "mcp": {
    "commands": [
      {
        "name": "filesystem",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace/sources"]
      }
    ],
    "servers": [
      {
        "name": "remote-api",
        "url": "https://api.example.com/mcp"
      }
    ]
  },
  "network": {
    "mode": "deny"
  },
  "secrets": ["my-github-token"]
}
```

The `my-github-token` secret (type `github`) automatically allows `api.github.com`, so no explicit `hosts` entry is needed.

### Notes

- Configuration is **optional** - workspaces can be registered without a `workspace.json` file
- The configuration file is validated only when it exists
- Validation errors are caught early during workspace registration (`init` command)
- All validation rules are enforced to prevent runtime errors
- The configuration model is imported from the `github.com/openkaiden/kdn-api/workspace-configuration/go` package for consistency across tools

## Multi-Level Configuration

kdn supports configuration at multiple levels, allowing you to customize workspace settings for different contexts. Configurations are automatically merged with proper precedence, making it easy to share common settings while still allowing project and agent-specific customization.

### Configuration Levels

**1. Workspace Configuration** (`.kaiden/workspace.json`)
- Stored in your project repository
- Shared with all developers
- Used by all agents
- Committed to version control

**2. Global Project Configuration** (`~/.kdn/config/projects.json` with `""` key)
- User-specific settings applied to **all projects**
- Stored on your local machine (not committed to git)
- Perfect for common settings like `.gitconfig`, SSH keys, or global environment variables
- Never shared with other developers

**3. Project-Specific Configuration** (`~/.kdn/config/projects.json`)
- User-specific settings for a **specific project**
- Stored on your local machine (not committed to git)
- Overrides global settings for this project
- Identified by project ID (git repository URL or directory path)

**4. Agent-Specific Configuration** (`~/.kdn/config/agents.json`)
- User-specific settings for a **specific agent** (Claude, Goose, etc.)
- Stored on your local machine (not committed to git)
- Overrides all other configurations
- Perfect for agent-specific environment variables or tools

### Configuration Precedence

When registering a workspace, configurations are merged in this order (later configs override earlier ones):

1. **Workspace** (`.kaiden/workspace.json`) - Base configuration from repository
2. **Global** (projects.json `""` key) - Your global settings for all projects
3. **Project** (projects.json specific project) - Your settings for this project
4. **Agent** (agents.json specific agent) - Your settings for this agent

**Example:** If `DEBUG` is defined in workspace config as `false`, in project config as `true`, and in agent config as `verbose`, the final value will be `verbose` (from agent config).

### Storage Location

User-specific configurations are stored in the kdn storage directory:

- **Default location**: `~/.kdn/config/`
- **Custom location**: Set via `--storage` flag or `KDN_STORAGE` environment variable

The storage directory contains:
- `config/agents.json` - Agent-specific environment variables and mounts
- `config/projects.json` - Project-specific and global environment variables and mounts
- `config/<agent>/` - Agent default settings files (e.g., `config/claude/.claude.json`)

### Agent Configuration File

**Location**: `~/.kdn/config/agents.json`

**Format**:
```json
{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      }
    ],
    "mounts": [
      {"host": "$HOME/.claude-config", "target": "$HOME/.claude-config"}
    ]
  },
  "goose": {
    "environment": [
      {
        "name": "GOOSE_MODE",
        "value": "verbose"
      }
    ]
  }
}
```

Each key is an agent name (e.g., `claude`, `goose`). The value uses the same structure as `workspace.json`.

### Project Configuration File

**Location**: `~/.kdn/config/projects.json`

**Format**:
```json
{
  "": {
    "mounts": [
      {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"},
      {"host": "$HOME/.ssh", "target": "$HOME/.ssh"}
    ]
  },
  "https://github.com/openkaiden/kdn/": {
    "environment": [
      {
        "name": "PROJECT_VAR",
        "value": "project-value"
      }
    ],
    "mounts": [
      {"host": "$SOURCES/../kaiden-common", "target": "$SOURCES/../kaiden-common"}
    ]
  },
  "/home/user/my/project": {
    "environment": [
      {
        "name": "LOCAL_DEV",
        "value": "true"
      }
    ]
  }
}
```

**Special Keys:**
- **Empty string `""`** - Global configuration applied to **all projects**
- **Git repository URL** - Configuration for all workspaces in that repository (e.g., `github.com/user/repo`)
- **Directory path** - Configuration for a specific directory (takes precedence over repository URL)

### Use Cases

**Global Settings for All Projects:**
```json
{
  "": {
    "mounts": [
      {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"},
      {"host": "$HOME/.ssh", "target": "$HOME/.ssh"},
      {"host": "$HOME/.gnupg", "target": "$HOME/.gnupg"}
    ]
  }
}
```
This mounts your git config and SSH keys in **every workspace** you create.

**Project-Specific API Keys:**
```json
{
  "github.com/company/project": {
    "environment": [
      {
        "name": "API_KEY",
        "secret": "project-api-key"
      }
    ]
  }
}
```
This adds an API key only for workspaces in the company project.

**Agent-Specific Debug Mode:**
```json
{
  "claude": {
    "environment": [
      {
        "name": "DEBUG",
        "value": "true"
      }
    ]
  }
}
```
This enables debug mode only when using the Claude agent.

### Using Multi-Level Configuration

**Register workspace with agent-specific config:**
```bash
kdn init --runtime podman --agent claude
```

**Register workspace with custom project:**
```bash
kdn init --runtime podman --project my-custom-project --agent goose
```

**Note:** The `--agent` flag is required (or set `KDN_DEFAULT_AGENT` environment variable) when registering a workspace.

### Merging Behavior

**Environment Variables:**
- Variables are merged by name
- Later configurations override earlier ones
- Example: If workspace sets `DEBUG=false` and agent sets `DEBUG=true`, the final value is `DEBUG=true`

**Mount Paths:**
- Mounts are deduplicated by `host`+`target` pair (duplicates removed)
- Order is preserved (first occurrence wins)
- Example: If workspace has mounts for `.gitconfig` and `.ssh`, and global adds `.ssh` and `.kube`, the result contains `.gitconfig`, `.ssh`, and `.kube`

**MCP Servers:**
- Commands and servers are each merged by `name`
- Later configurations override earlier ones with the same name
- Example: If workspace defines an MCP command named `filesystem` and agent config also defines `filesystem`, the agent config's version is used

**Network:**
- The base (lower-precedence) network policy is dominant
- If base has `allow` mode, the base configuration is used regardless of the override
- If base has `deny` mode and override has `allow` mode, the base configuration is used (overrides cannot loosen the policy)
- If both base and override have `deny` mode, the hosts from both are merged (deduplicated)
- Example: If workspace config denies all except `api.github.com` and agent config allows all, the final result is deny with `api.github.com` allowed (workspace policy wins)

**Secrets:**
- Secrets are deduplicated by name
- First occurrence wins (base secrets take precedence); later configs only add unseen names
- Order is preserved: base secrets first, then new unique secrets from overrides
- Example: If workspace defines `"my-github-token"` and agent config also defines `"my-github-token"`, the workspace entry is kept and the agent config entry is ignored

### Configuration Files Don't Exist?

All multi-level configurations are **optional**:
- If `agents.json` doesn't exist, agent-specific configuration is skipped
- If `projects.json` doesn't exist, project and global configurations are skipped
- If `workspace.json` doesn't exist, only user-specific configurations are used

The system works without any configuration files and merges only the ones that exist.

### Example: Complete Multi-Level Setup

**Workspace config** (`.kaiden/workspace.json` - committed to git):
```json
{
  "environment": [
    {"name": "NODE_ENV", "value": "development"}
  ]
}
```

**Global config** (`~/.kdn/config/projects.json` - your machine only):
```json
{
  "": {
    "mounts": [
      {"host": "$HOME/.gitconfig", "target": "$HOME/.gitconfig"},
      {"host": "$HOME/.ssh", "target": "$HOME/.ssh"}
    ]
  }
}
```

**Project config** (`~/.kdn/config/projects.json` - your machine only):
```json
{
  "https://github.com/openkaiden/kdn/": {
    "environment": [
      {"name": "DEBUG", "value": "true"}
    ]
  }
}
```

**Agent config** (`~/.kdn/config/agents.json` - your machine only):
```json
{
  "claude": {
    "environment": [
      {"name": "CLAUDE_VERBOSE", "value": "true"}
    ]
  }
}
```

**Result when running** `kdn init --runtime podman --agent claude`:
- Environment: `NODE_ENV=development`, `DEBUG=true`, `CLAUDE_VERBOSE=true`
- Mounts: `$HOME/.gitconfig`, `$HOME/.ssh`

## Secret Commands

kdn manages two related concepts for injecting credentials into workspaces:

- **Secret services** — Built-in definitions that describe how a credential is injected into outbound HTTP requests. Each service specifies the host pattern to match, the HTTP header to set, and the header value template. Use `kdn service list` to see the available services.
- **Secrets** — Named credential entries created with `kdn secret create`. Each secret has a type (a service name or `other`), a value stored securely in the system keychain, and optional metadata. Secrets are referenced by name in workspace configuration.

**Workflow:**
1. Run `kdn service list` to see available service types (e.g., `github`)
2. Create a secret: `kdn secret create my-github-token --type github --value ghp_xxx`
3. Reference the secret by name in workspace configuration: `"secrets": ["my-github-token"]`

**Note:** The `secret` field on environment variable entries (e.g., `{"name": "GH_TOKEN", "secret": "github-token"}`) is a separate mechanism that references runtime secrets (such as Podman secrets) for injecting values as environment variables. It is useful when a local tool inside the workspace needs a credential via an environment variable. For credentials used in outbound network requests, use the Secret abstraction described here instead — secrets are injected as HTTP headers by OneCLI and are not exposed as environment variables.

### `service list` - List Registered Services

Lists all secret services available for workspace configuration.

#### Usage

```bash
kdn service list [flags]
```

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)

#### Examples

**List all services (human-readable table):**
```bash
kdn service list
```
Output:
```text
NAME    HOST PATTERN       PATH  HEADER          HEADER TEMPLATE    ENV VARS                DESCRIPTION
github  api.github.com         Authorization   Bearer ${value}    GH_TOKEN, GITHUB_TOKEN  GitHub API token for accessing GitHub repositories and services
```

**List services in JSON format:**
```bash
kdn service list --output json
```
Output:
```json
{
  "items": [
    {
      "name": "github",
      "description": "GitHub API token for accessing GitHub repositories and services",
      "hostsPatterns": ["api.github.com"],
      "headerName": "Authorization",
      "headerTemplate": "Bearer ${value}",
      "envVars": ["GH_TOKEN", "GITHUB_TOKEN"]
    }
  ]
}
```

**List using short flag:**
```bash
kdn service list -o json
```

#### Notes

- Services are defined in the embedded configuration and are always available regardless of runtime or environment
- Each service describes how to inject credentials into HTTP requests for matching hosts

### `secret create` - Create a New Secret

Creates a new secret and stores its value securely in the system keychain. Non-sensitive metadata (type, hosts, path, header template, envs) is persisted in the kdn storage directory.

#### Usage

```bash
kdn secret create <name> [flags]
```

#### Arguments

- `name` - Unique name to identify this secret

#### Flags

- `--type <type>` - Type of secret: a registered service name (e.g., `github`) or `other` (required)
- `--value <value>` - Secret value to store in the system keychain (required)
- `--description <text>` - Optional human-readable description
- `--host <pattern>` - Host pattern (required for `--type=other`; can be specified multiple times)
- `--header <name>` - HTTP header name (required for `--type=other`)
- `--headerTemplate <template>` - HTTP header value template using `${value}` as placeholder (optional, for `--type=other`)
- `--path <path>` - URL path restriction (optional, for `--type=other`)
- `--env <name>` - Environment variable name to expose the secret value (optional, for `--type=other`; can be specified multiple times)
- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Create a GitHub token secret:**
```bash
kdn secret create my-github-token --type github --value ghp_mytoken
```
Output:
```text
Secret "my-github-token" created successfully
```

**Create a custom secret with all descriptor flags:**
```bash
kdn secret create my-api-key --type other --value secret123 \
  --host api.example.com --host dev.example.com \
  --path /api/v1 \
  --header Authorization --headerTemplate "Bearer ${value}" \
  --env MY_API_KEY --env API_KEY
```

**Create a custom secret with only required flags:**
```bash
kdn secret create my-api-key --type other --value secret123 \
  --host api.example.com --header Authorization
```

**Create a secret with JSON output:**
```bash
kdn secret create my-github-token --type github --value ghp_mytoken --output json
```
Output:
```json
{
  "name": "my-github-token"
}
```

#### Notes

- `--type` must be a registered service name (use `kdn service list` to see available types) or `other`
- For `--type=other`, `--host` and `--header` are required; all other descriptor flags are optional
- For named types (e.g., `github`), the descriptor flags (`--host`, `--header`, `--headerTemplate`, `--env`, `--path`) must not be specified — those are defined by the service
- The secret value is stored in the system keychain (GNOME Keyring on Linux, Keychain on macOS, DPAPI on Windows) and never written to disk in plain text
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `secret list` - List All Secrets

Lists all secrets stored in the kdn storage directory.

#### Usage

```bash
kdn secret list [flags]
```

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**List all secrets (human-readable table):**
```bash
kdn secret list
```
Output:
```text
NAME              TYPE    DESCRIPTION
my-github-token   github
my-api-key        other   Internal API key
```

**List secrets in JSON format:**
```bash
kdn secret list --output json
```
Output:
```json
{
  "items": [
    {
      "name": "my-github-token",
      "type": "github",
      "description": ""
    },
    {
      "name": "my-api-key",
      "type": "other",
      "description": "Internal API key",
      "hosts": ["api.example.com"],
      "header": "Authorization",
      "headerTemplate": "Bearer ${value}"
    }
  ]
}
```

**List using short flag:**
```bash
kdn secret list -o json
```

#### Notes

- Only metadata is listed; secret values are never displayed
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `secret remove` - Remove a Secret

Removes a secret from the system keychain and from the kdn storage directory.

#### Usage

```bash
kdn secret remove <name> [flags]
```

#### Arguments

- `name` - Name of the secret to remove

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Remove a secret by name:**
```bash
kdn secret remove my-github-token
```
Output:
```text
Secret "my-github-token" removed successfully
```

**Remove a secret with JSON output:**
```bash
kdn secret remove my-github-token --output json
```
Output:
```json
{
  "name": "my-github-token"
}
```

**Remove using the alias:**
```bash
kdn secret rm my-github-token
```

#### Notes

- Removing a secret also deletes its value from the system keychain
- Workspaces that reference the removed secret by name will fail to start until a new secret with the same name is created
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

## Commands

### `info` - Display Information About kdn

Displays version, available agents, and supported runtimes.

#### Usage

```bash
kdn info [flags]
```

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Show info (human-readable format):**
```bash
kdn info
```
Output:
```text
Version: 0.3.0
Agents: claude
Runtimes: fake, podman
```

**Show info in JSON format:**
```bash
kdn info --output json
```
Output:
```json
{
  "version": "0.3.0",
  "agents": [
    "claude"
  ],
  "runtimes": [
    "fake",
    "podman"
  ]
}
```

**Show info using short flag:**
```bash
kdn info -o json
```

#### Notes

- Agents are discovered from runtimes that support agent configuration (e.g., the Podman runtime reports agents from its configuration files)
- Runtimes are listed based on availability in the current environment (e.g., the Podman runtime only appears if the `podman` CLI is installed)
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `init` - Register a New Workspace

Registers a new workspace with kdn, making it available for agent launch and configuration.

#### Usage

```bash
kdn init [sources-directory] [flags]
```

#### Arguments

- `sources-directory` - Path to the directory containing your project source files (optional, defaults to current directory `.`)

#### Flags

- `--runtime, -r <type>` - Runtime to use for the workspace (required if `KDN_DEFAULT_RUNTIME` is not set)
- `--agent, -a <name>` - Agent to use for the workspace (required if `KDN_DEFAULT_AGENT` is not set)
- `--model, -m <id>` - Model ID to configure for the agent. Supports three formats: `model`, `provider::model` (auto-configures provider with default base URL), or `provider::model::baseURL` (auto-configures provider with custom endpoint). Localhost aliases in base URLs are auto-converted to `host.containers.internal` for container access
- `--workspace-configuration <path>` - Directory for workspace configuration files (default: `<sources-directory>/.kaiden`)
- `--name, -n <name>` - Human-readable name for the workspace (default: generated from sources directory)
- `--project, -p <identifier>` - Custom project identifier to override auto-detection (default: auto-detected from git repository or source directory)
- `--start` - Start the workspace after registration (can also be set via `KDN_INIT_AUTO_START` environment variable)
- `--verbose, -v` - Show detailed output including all workspace information
- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Register the current directory:**
```bash
kdn init --runtime podman --agent claude
```
Output: `a1b2c3d4e5f6...` (workspace ID)

**Register a specific directory:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude
```

**Register with a custom name:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --name "my-awesome-project"
```

**Register with a custom project identifier:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --project "my project"
```

**Register with custom configuration location:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --workspace-configuration /path/to/config
```

**Register with a specific model:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --model claude-sonnet-4-20250514
```

**Register and start immediately:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --start
```
Output: `a1b2c3d4e5f6...` (workspace ID, workspace is now running)

**Register and start using environment variable:**
```bash
export KDN_INIT_AUTO_START=1
kdn init /path/to/myproject --runtime podman --agent claude
```
Output: `a1b2c3d4e5f6...` (workspace ID, workspace is now running)

**View detailed output:**
```bash
kdn init --runtime podman --agent claude --verbose
```
Output:
```text
Registered workspace:
  ID: a1b2c3d4e5f6...
  Name: myproject
  Project: /absolute/path/to/myproject
  Agent: claude
  Model: (default)
  Sources directory: /absolute/path/to/myproject
  Configuration directory: /absolute/path/to/myproject/.kaiden
  State: stopped
```

**View detailed output with a specific model:**
```bash
kdn init --runtime podman --agent claude --model claude-sonnet-4-20250514 --verbose
```
Output:
```text
Registered workspace:
  ID: a1b2c3d4e5f6...
  Name: myproject
  Project: /absolute/path/to/myproject
  Agent: claude
  Model: claude-sonnet-4-20250514
  Sources directory: /absolute/path/to/myproject
  Configuration directory: /absolute/path/to/myproject/.kaiden
  State: stopped
```

**Register with a model provider (default endpoint):**
```bash
kdn init --runtime podman --agent opencode --model ollama::gemma4:26b
```

**Register with a model provider and custom endpoint:**
```bash
kdn init --runtime podman --agent opencode --model ollama::gemma4:26b::http://192.168.1.50:11434/v1
```

**JSON output (default - ID only):**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with verbose flag (full workspace details):**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --output json --verbose
```
Output:
```json
{
  "id": "a1b2c3d4e5f6...",
  "name": "myproject",
  "agent": "claude",
  "project": "/absolute/path/to/myproject",
  "state": "stopped",
  "paths": {
    "source": "/absolute/path/to/myproject",
    "configuration": "/absolute/path/to/myproject/.kaiden"
  }
}
```

**JSON output with verbose flag and a specific model:**
```bash
kdn init /path/to/myproject --runtime podman --agent claude --model claude-sonnet-4-20250514 --output json --verbose
```
Output:
```json
{
  "id": "a1b2c3d4e5f6...",
  "name": "myproject",
  "agent": "claude",
  "model": "claude-sonnet-4-20250514",
  "project": "/absolute/path/to/myproject",
  "state": "stopped",
  "paths": {
    "source": "/absolute/path/to/myproject",
    "configuration": "/absolute/path/to/myproject/.kaiden"
  }
}
```

**JSON output with short flags:**
```bash
kdn init -r fake -a claude -o json -v
```

**Show runtime command output (e.g., image build logs):**
```bash
kdn init --runtime podman --agent claude --show-logs
```

#### Workspace Naming

- If `--name` is not provided, the name is automatically generated from the last component of the sources directory path
- Names are automatically sanitized: uppercased letters are lowercased, any run of characters that are not alphanumeric, hyphens, dots, or underscores (including spaces) is collapsed into a single hyphen, and leading/trailing separators (hyphens, dots, underscores) are stripped
- If a workspace with the same name already exists, kdn automatically appends an increment (`-2`, `-3`, etc.) to ensure uniqueness

**Examples:**
```bash
# First workspace in /home/user/project
kdn init /home/user/project --runtime podman --agent claude
# Name: "project"

# Directory with spaces in its name
kdn init "/home/user/my project" --runtime podman --agent claude
# Name: "my-project"

# Custom name with uppercase letters
kdn init /home/user/project --runtime podman --agent claude --name MyWork
# Name: "mywork"

# Second workspace with the same directory name
kdn init /home/user/another-location/project --runtime podman --agent claude --name "project"
# Name: "project-2"

# Third workspace with the same name
kdn init /tmp/project --runtime podman --agent claude --name "project"
# Name: "project-3"
```

#### Project Detection

When registering a workspace, kdn automatically detects and stores a project identifier. This allows grouping workspaces that belong to the same project, even across different branches, forks, or subdirectories.

**The project is determined using the following rules:**

**1. Git repository with remote URL**

The project is the repository remote URL (without `.git` suffix) plus the workspace's relative path within the repository:

- **At repository root**: `https://github.com/user/repo/`
- **In subdirectory**: `https://github.com/user/repo/sub/path`

**Remote priority:**
1. `upstream` remote is checked first (useful for forks)
2. `origin` remote is used if `upstream` doesn't exist
3. If neither exists, falls back to local repository path (see below)

**Example - Fork with upstream:**
```bash
# Repository setup:
# upstream: https://github.com/openkaiden/kdn.git
# origin:   https://github.com/myuser/kdn.git (fork)

# Workspace at repository root
kdn init /home/user/kdn --runtime podman --agent claude
# Project: https://github.com/openkaiden/kdn/

# Workspace in subdirectory
kdn init /home/user/kdn/pkg/git --runtime podman --agent claude
# Project: https://github.com/openkaiden/kdn/pkg/git
```

This ensures all forks and branches of the same upstream repository are grouped together.

**2. Git repository without remote**

The project is the repository root directory path plus the workspace's relative path:

- **At repository root**: `/home/user/my-local-repo`
- **In subdirectory**: `/home/user/my-local-repo/sub/path`

**Example - Local repository:**
```bash
# Workspace at repository root
kdn init /home/user/local-repo --runtime podman --agent claude
# Project: /home/user/local-repo

# Workspace in subdirectory
kdn init /home/user/local-repo/pkg/utils --runtime podman --agent claude
# Project: /home/user/local-repo/pkg/utils
```

**3. Non-git directory**

The project is the workspace source directory path:

**Example - Regular directory:**
```bash
kdn init /tmp/workspace --runtime podman --agent claude
# Project: /tmp/workspace
```

**Benefits:**

- **Cross-branch grouping**: Workspaces in different git worktrees or branches of the same repository share the same project
- **Fork grouping**: Forks reference the upstream repository, grouping all contributors working on the same project
- **Subdirectory support**: Monorepo subdirectories are tracked with their full path for precise identification
- **Custom override**: Use `--project` flag to manually group workspaces under a custom identifier (e.g., "client-project")
- **Future filtering**: The project field enables filtering and grouping commands (e.g., list all workspaces for a specific project)

#### Notes

- **Runtime is required**: You must specify a runtime using either the `--runtime` flag or the `KDN_DEFAULT_RUNTIME` environment variable
- **Agent is required**: You must specify an agent using either the `--agent` flag or the `KDN_DEFAULT_AGENT` environment variable
- **Model is optional**: Use `--model` to specify a model ID for the agent. The flag takes precedence over any model defined in the agent's default settings files (`~/.kdn/config/<agent>/`). If not provided, the agent uses its default model or the one configured in settings. All agents support model configuration: Claude (via `.claude/settings.json`), Goose (via `config.yaml`), Cursor (via `.cursor/cli-config.json`), and OpenCode (via `.config/opencode/opencode.json`)
- **Provider configuration**: The `--model` flag supports a `provider::model` format (e.g. `ollama::gemma4:26b`) that auto-configures the provider endpoint and stores the model ID as `provider/model`. Known providers (`ollama`, `ramalama`) have default base URLs pointing to `host.containers.internal`; unknown providers require the full format `provider::model::baseURL`. Localhost aliases (`localhost`, `127.0.0.1`, `0.0.0.0`, `::1`) in base URLs are automatically converted to `host.containers.internal` for container accessibility
- **Project auto-detection**: The project identifier is automatically detected from git repository information or source directory path. Use `--project` flag to override with a custom identifier
- **Auto-start**: Use the `--start` flag or set `KDN_INIT_AUTO_START=1` to automatically start the workspace after registration, combining `init` and `start` into a single operation
- All directory paths are converted to absolute paths for consistency
- The workspace ID is a unique identifier generated automatically
- Workspaces can be listed using the `workspace list` command
- The default configuration directory (`.kaiden`) is created inside the sources directory unless specified otherwise
- JSON output format is useful for scripting and automation
- Without `--verbose`, JSON output returns only the workspace ID
- With `--verbose`, JSON output includes full workspace details (ID, name, agent, model, paths); the `model` field is only present when a model was explicitly set with `--model`
- Use `--show-logs` to display the full stdout and stderr from runtime commands (e.g., `podman build` output during image creation)
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace list` - List All Registered Workspaces

Lists all workspaces that have been registered with kdn. Also available as the shorter aliases `list` and `ls`.

#### Usage

```bash
kdn workspace list [flags]
kdn list [flags]
kdn ls [flags]
```

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**List all workspaces (human-readable format):**
```bash
kdn workspace list
```
Output:
```text
NAME             SHORT ID      PROJECT                              SOURCES                              AGENT    MODEL                          STATE
myproject        a1b2c3d4e5f6  /absolute/path/to/myproject          /absolute/path/to/myproject          claude   claude-sonnet-4-20250514       running for 5min
another-project  f6e5d4c3b2a1  /absolute/path/to/another-project    /absolute/path/to/another-project    goose                                   stopped
```

The `AGENT` and `MODEL` columns are displayed separately. When no model is set, the `MODEL` column is empty.

The `STATE` column shows a human-readable duration for running workspaces: `running for Xs` (under 1 minute), `running for Xmin` (under 1 hour), or `running for H:MMh` (1 hour or more). Stopped, errored, or unknown workspaces show their state name directly.

**Use the short aliases:**
```bash
kdn list
kdn ls
```

**List workspaces in JSON format:**
```bash
kdn workspace list --output json
```
Output:
```json
{
  "items": [
    {
      "id": "a1b2c3d4e5f6...",
      "name": "myproject",
      "agent": "claude",
      "model": "claude-sonnet-4-20250514",
      "project": "/absolute/path/to/myproject",
      "state": "running",
      "paths": {
        "source": "/absolute/path/to/myproject",
        "configuration": "/absolute/path/to/myproject/.kaiden"
      },
      "timestamps": {
        "created": 1752912000000,
        "started": 1752912300000
      }
    },
    {
      "id": "f6e5d4c3b2a1...",
      "name": "another-project",
      "agent": "goose",
      "project": "/absolute/path/to/another-project",
      "state": "stopped",
      "paths": {
        "source": "/absolute/path/to/another-project",
        "configuration": "/absolute/path/to/another-project/.kaiden"
      },
      "timestamps": {
        "created": 1752912000000
      }
    }
  ]
}
```

The `model` field is only present when a model was explicitly specified during `init` with the `--model` flag. When no model is set, the field is omitted from the JSON output.

The `timestamps` object is always present. `created` is a Unix millisecond timestamp recording when the workspace was registered. `started` is a Unix millisecond timestamp recording when the workspace was last started; it is omitted when the workspace is not running.

**List with short flag:**
```bash
kdn list -o json
```

#### Notes

- When no workspaces are registered, the command displays "No workspaces registered"
- The `AGENT` and `MODEL` columns are displayed separately. The `MODEL` column shows the model name when set at registration time, or is empty when no model was specified
- In JSON output, the `model` field is only present when a model was explicitly set with `--model` during `init`
- The JSON output format is useful for scripting and automation
- All paths are displayed as absolute paths for consistency
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace start` - Start a Workspace

Starts a registered workspace by its name or ID. Also available as the shorter alias `start`.

#### Usage

```bash
kdn workspace start NAME|ID [flags]
kdn start NAME|ID [flags]
```

#### Arguments

- `NAME|ID` - The workspace name or unique identifier (required)

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Start a workspace by ID:**
```bash
kdn workspace start a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of started workspace)

**Start a workspace by name:**
```bash
kdn workspace start my-project
```
Output: `a1b2c3d4e5f6...` (ID of started workspace)

**Use the short alias:**
```bash
kdn start my-project
```

**View workspace names and IDs before starting:**
```bash
# First, list all workspaces to find the name or ID
kdn list

# Then start the desired workspace (using either name or ID)
kdn start my-project
```

**JSON output:**
```bash
kdn workspace start a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kdn start a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kdn workspace start a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kdn start invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kdn start invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

#### Notes

- You can specify the workspace using either its name or ID (both can be obtained using the `workspace list` or `list` command)
- The command always outputs the workspace ID, even when started by name
- Starting a workspace launches its associated runtime instance
- The workspace runtime state is updated to reflect that it's running
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace stop` - Stop a Workspace

Stops a running workspace by its name or ID. Also available as the shorter alias `stop`.

#### Usage

```bash
kdn workspace stop NAME|ID [flags]
kdn stop NAME|ID [flags]
```

#### Arguments

- `NAME|ID` - The workspace name or unique identifier (required)

#### Flags

- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Stop a workspace by ID:**
```bash
kdn workspace stop a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of stopped workspace)

**Stop a workspace by name:**
```bash
kdn workspace stop my-project
```
Output: `a1b2c3d4e5f6...` (ID of stopped workspace)

**Use the short alias:**
```bash
kdn stop my-project
```

**View workspace names and IDs before stopping:**
```bash
# First, list all workspaces to find the name or ID
kdn list

# Then stop the desired workspace (using either name or ID)
kdn stop my-project
```

**JSON output:**
```bash
kdn workspace stop a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kdn stop a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kdn workspace stop a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kdn stop invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kdn stop invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

#### Notes

- You can specify the workspace using either its name or ID (both can be obtained using the `workspace list` or `list` command)
- The command always outputs the workspace ID, even when stopped by name
- Stopping a workspace stops its associated runtime instance
- The workspace runtime state is updated to reflect that it's stopped
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace terminal` - Connect to a Workspace

Connects to a workspace with an interactive terminal session. If the workspace is stopped, it is automatically started before connecting. Also available as the shorter alias `terminal`.

#### Usage

```bash
kdn workspace terminal NAME|ID [COMMAND...] [flags]
kdn terminal NAME|ID [COMMAND...] [flags]
```

#### Arguments

- `NAME|ID` - The workspace name or unique identifier (required)
- `COMMAND...` - Optional command to execute instead of the default agent command

#### Flags

- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Connect using the default agent command (by ID):**
```bash
kdn workspace terminal a1b2c3d4e5f6...
```

**Connect using the default agent command (by name):**
```bash
kdn workspace terminal my-project
```

This starts an interactive session with the default agent (typically Claude Code) inside the workspace container, auto-starting it if needed.

**Use the short alias:**
```bash
kdn terminal my-project
```

**Run a bash shell:**
```bash
kdn terminal my-project bash
```

**Run a command with flags (use -- to prevent kdn from parsing them):**
```bash
kdn terminal a1b2c3d4e5f6... -- bash -c 'echo hello'
```

The `--` separator tells kdn to stop parsing flags and pass everything after it directly to the container. This is useful when your command includes flags that would otherwise be interpreted by kdn.

**List workspaces and connect:**
```bash
# First, list all workspaces to find the ID
kdn list

# Optionally start a workspace explicitly
kdn start a1b2c3d4e5f6...

# Connect with a terminal (auto-starts stopped workspaces)
kdn terminal a1b2c3d4e5f6...
```

#### Error Handling

**Workspace not found:**
```bash
kdn terminal invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not running (auto-started):**

If the workspace is stopped, `terminal` automatically starts it before connecting. You can also start it explicitly with `kdn start` beforehand.

#### Notes

- If the workspace is stopped, it is automatically started before connecting. You can also use `workspace start` explicitly beforehand
- You can specify the workspace using either its name or ID (both can be obtained using the `workspace list` or `list` command)
- By default (when no command is provided), the runtime uses the `terminal_command` from the agent's configuration file
  - For example, if the workspace was created with `--agent claude`, it will use the command defined in `claude.json` (typically `["claude"]`)
  - This ensures you connect directly to the configured agent
- You can override the default by providing a custom command (e.g., `bash`, `python`, or any executable available in the container)
- Use the `--` separator when your command includes flags to prevent kdn from trying to parse them
- The terminal session is fully interactive with stdin/stdout/stderr connected to your terminal
- The command execution happens inside the workspace's container/runtime environment
- JSON output is **not supported** for this command as it's inherently interactive
- Runtime support: The terminal command requires the runtime to implement the Terminal interface. The Podman runtime supports this using `podman exec -it`

### `workspace remove` - Remove a Workspace

Removes a registered workspace by its name or ID. Also available as the shorter aliases `remove` and `rm`.

#### Usage

```bash
kdn workspace remove NAME|ID [flags]
kdn remove NAME|ID [flags]
kdn rm NAME|ID [flags]
```

#### Arguments

- `NAME|ID` - The workspace name or unique identifier (required)

#### Flags

- `--force, -f` - Stop the workspace if it is running before removing it
- `--output, -o <format>` - Output format (supported: `json`)
- `--show-logs` - Show stdout and stderr from runtime commands (cannot be combined with `--output json`)
- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Remove a workspace by ID:**
```bash
kdn workspace remove a1b2c3d4e5f6...
```
Output: `a1b2c3d4e5f6...` (ID of removed workspace)

**Remove a workspace by name:**
```bash
kdn workspace remove my-project
```
Output: `a1b2c3d4e5f6...` (ID of removed workspace)

**Use the short aliases:**
```bash
kdn remove my-project
kdn rm my-project
```

**View workspace names and IDs before removing:**
```bash
# First, list all workspaces to find the name or ID
kdn list

# Then remove the desired workspace (using either name or ID)
kdn remove my-project
```

**Remove a running workspace (stops it first):**
```bash
kdn workspace remove a1b2c3d4e5f6... --force
```
Output: `a1b2c3d4e5f6...` (ID of removed workspace)

**JSON output:**
```bash
kdn workspace remove a1b2c3d4e5f6... --output json
```
Output:
```json
{
  "id": "a1b2c3d4e5f6..."
}
```

**JSON output with short flag:**
```bash
kdn remove a1b2c3d4e5f6... -o json
```

**Show runtime command output:**
```bash
kdn workspace remove a1b2c3d4e5f6... --show-logs
```

#### Error Handling

**Workspace not found (text format):**
```bash
kdn remove invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Workspace not found (JSON format):**
```bash
kdn remove invalid-id --output json
```
Output:
```json
{
  "error": "workspace not found: invalid-id"
}
```

**Removing a running workspace without --force:**

Attempting to remove a running workspace without `--force` will fail because the runtime refuses to remove a running instance. Stop the workspace first, or use `--force`:

```bash
# Stop first, then remove
kdn stop a1b2c3d4e5f6...
kdn remove a1b2c3d4e5f6...

# Or remove in one step
kdn remove a1b2c3d4e5f6... --force
```

#### Notes

- You can specify the workspace using either its name or ID (both can be obtained using the `workspace list` or `list` command)
- The command always outputs the workspace ID, even when removed by name
- Removing a workspace only unregisters it from kdn; it does not delete any files from the sources or configuration directories
- If the workspace name or ID is not found, the command will fail with a helpful error message
- Use `--force` to automatically stop a running workspace before removing it; without this flag, removing a running workspace will fail
- Tab completion for this command suggests only non-running workspaces by default; when `--force` is specified, all workspaces are suggested
- JSON output format is useful for scripting and automation
- When using `--output json`, errors are also returned in JSON format for consistent parsing
- **JSON error handling**: When `--output json` is used, errors are written to stdout (not stderr) in JSON format, and the CLI exits with code 1. Always check the exit code to determine success/failure

### `workspace dashboard` - Open the Dashboard for a Workspace

Prints the dashboard URL for a running workspace and opens it in the default browser. Also available as the shorter alias `dashboard`.

The dashboard is only available for runtimes that support it (e.g. Podman via the OneCLI web interface).

#### Usage

```bash
kdn workspace dashboard NAME|ID [flags]
kdn dashboard NAME|ID [flags]
```

#### Arguments

- `NAME|ID` - The workspace name or unique identifier (required)

#### Flags

- `--storage <path>` - Storage directory for kdn data (default: `$HOME/.kdn`)

#### Examples

**Open dashboard by ID:**
```bash
kdn workspace dashboard a1b2c3d4e5f6...
```
Output: `http://localhost:8888` (URL printed; browser opened automatically)

**Open dashboard by name:**
```bash
kdn workspace dashboard my-project
```

**Use the short alias:**
```bash
kdn dashboard my-project
```

#### Error Handling

**Workspace not found:**
```bash
kdn dashboard invalid-id
```
Output:
```text
Error: workspace not found: invalid-id
Use 'workspace list' to see available workspaces
```

**Runtime does not support dashboard:**
```bash
kdn dashboard my-project
```
Output:
```text
Error: dashboard not supported for workspace "my-project"
```

#### Notes

- The workspace must be running; the command does not auto-start it
- The URL is always printed to stdout, even when the browser opens successfully
- Opening the browser is best-effort; errors are silently ignored
- Tab completion suggests only running workspaces whose runtime supports the Dashboard interface
- JSON output is **not supported** for this command
