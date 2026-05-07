---
name: working-with-autoconf
description: Guide to the autoconf package — how secret detection, filtering, and the runner are wired together, and how to extend the system with new detector types
argument-hint: ""
---

# Working with Autoconf

The `pkg/autoconf` package implements `kdn autoconf`: it scans the environment for known API keys, checks whether each one is already configured, and orchestrates creation + config-target recording for those that aren't.

## Architecture

```text
pkg/autoconf/
  detect.go      — SecretDetector interface + envSecretDetector
  filter.go      — SecretFilter interface + alreadyConfiguredFilter + FilterResult
  autoconf.go    — Autoconf interface + autoconfRunner + ConfigTarget constants
```

The data flow inside `autoconfRunner.Run()`:

```text
detector.Detect()             → FilterResult
  ├── FilterResult.Configured → print "already configured (location)" per entry
  └── FilterResult.NeedsAction
        ↓ (one per secret)
        confirm creation?
        createSecret()
        addToConfig() → selectTarget() → applyTarget()
                                          (local target creates workspace.json if absent)
```

## Key Types

### `SecretDetector` (`detect.go`)

```go
type SecretDetector interface {
    Detect() (FilterResult, error)
}
```

`Detect()` takes no parameters — data sources are baked in at construction time. This keeps the interface consistent as new detector types are added (language detectors, config-dir detectors, etc.) — each owns its own data source and exposes only `Detect()`.

Two constructors:

```go
// No filtering — returns all detected secrets in FilterResult.NeedsAction.
func NewSecretDetector(services []secretservice.SecretService) SecretDetector

// Filters out already-configured secrets; those appear in FilterResult.Configured.
func NewFilteredSecretDetector(
    services     []secretservice.SecretService,
    store        secret.Store,
    loader       config.ProjectConfigLoader,
    projectID    string,
    workspaceConfig config.Config,
) SecretDetector
```

`kdn autoconf` always uses `NewFilteredSecretDetector`. `NewSecretDetector` is useful when you want raw detections without any store/config checks.

### `FilterResult` and `ConfiguredSecret` (`filter.go`)

```go
type FilterResult struct {
    NeedsAction []DetectedSecret   // secrets missing from store or any config source
    Configured  []ConfiguredSecret // secrets fully set up — store + ≥1 config source
}

type ConfiguredSecret struct {
    DetectedSecret                // embeds ServiceName, EnvVarName, Value
    Locations []ConfigTarget      // which config sources reference this secret
}
```

`ConfigTarget` values: `ConfigTargetGlobal`, `ConfigTargetProject`, `ConfigTargetLocal`.

### `SecretFilter` (`filter.go`)

```go
type SecretFilter interface {
    Filter(detected []DetectedSecret) (FilterResult, error)
}
```

`alreadyConfiguredFilter` checks three sources in parallel:

| Source | How loaded |
|--------|-----------|
| Global config | `loader.Load("")` |
| Project-specific config | `loader.Load(projectID)` minus global entries |
| Local workspace config | `workspaceConfig.Load()` |

A secret is classified as **Configured** when it is in the store **and** present in at least one source. Location info is recorded for display. A secret missing from the store or from every config source goes to **NeedsAction**.

### `Autoconf` (`autoconf.go`)

```go
type Autoconf interface {
    Run(out io.Writer) error
}

func New(opts Options) Autoconf
```

`Options` carries all dependencies — inject fakes in tests instead of wiring real implementations.

Important `Options` fields:

| Field | Purpose |
|-------|---------|
| `Detector` | `SecretDetector` |
| `Store` | `secret.Store` for `Get`/`Create` |
| `ProjectUpdater` | `config.ProjectConfigUpdater` — writes `projects.json` |
| `WorkspaceUpdater` | `config.WorkspaceConfigUpdater` — writes `.kaiden/workspace.json` (creates it if absent) |
| `ProjectID` | computed project identifier for the current directory |
| `Yes` | skip all confirmations, default to global target |
| `Confirm` | `func(prompt string) (bool, error)` — injectable |
| `SelectTarget` | `func(name string, opts []ConfigTargetOption) (ConfigTarget, error)` — injectable |

Return `ErrSkipped` from `SelectTarget` to skip adding the secret to any config target (secret is still created in the store).

## Adding a New Secret Service for Auto-detection

`autoconf` detects services registered in `secretservices.json`. To make a new service auto-detectable, just add it to `pkg/secretservicesetup/secretservices.json`:

```json
{
  "name": "my-service",
  "hostsPatterns": ["api.my-service.com"],
  "headerName": "Authorization",
  "headerTemplate": "Bearer ${value}",
  "envVars": ["MY_SERVICE_TOKEN", "MY_SERVICE_KEY"]
}
```

The `envVars` field drives detection: `autoconf` iterates the list in order and uses the first non-empty env var found. No code changes required.

## Adding a New Detector Type

The `SecretDetector` interface intentionally takes no parameters so new detectors can be added without changing the interface. Example skeleton for a language detector:

```go
// pkg/autoconf/detectlang.go
package autoconf

type DetectedLanguage struct {
    Name    string // e.g. "go", "python"
    Version string
}

type LanguageDetector interface {
    Detect() ([]DetectedLanguage, error)
}

type fileLanguageDetector struct {
    root string
}

func NewLanguageDetector(root string) LanguageDetector {
    return &fileLanguageDetector{root: root}
}

func (d *fileLanguageDetector) Detect() ([]DetectedLanguage, error) {
    // read go.mod, requirements.txt, etc.
}
```

The autoconf runner can then hold both a `SecretDetector` and a `LanguageDetector`, each with its own result type. Keep them separate — don't try to unify them into a single interface.

## Testing Patterns

### Fake `SecretDetector`

```go
type fakeDetector struct {
    detected   []DetectedSecret
    configured []ConfiguredSecret
}

func (f *fakeDetector) Detect() (FilterResult, error) {
    return FilterResult{NeedsAction: f.detected, Configured: f.configured}, nil
}
```

### Fake `SecretFilter`

```go
type fakeFilter struct {
    result FilterResult
    err    error
}

func (f *fakeFilter) Filter(_ []DetectedSecret) (FilterResult, error) {
    return f.result, f.err
}
```

### Fake `secret.Store` for filter tests

```go
type fakeStore struct {
    existing map[string]struct{}
}

func (f *fakeStore) Create(_ secret.CreateParams) error         { return nil }
func (f *fakeStore) List() ([]secret.ListItem, error)           { return nil, nil }
func (f *fakeStore) Remove(_ string) error                      { return nil }
func (f *fakeStore) Get(name string) (secret.ListItem, string, error) {
    if _, ok := f.existing[name]; ok {
        return secret.ListItem{Name: name}, "value", nil
    }
    return secret.ListItem{}, "", fmt.Errorf("%q: %w", name, secret.ErrSecretNotFound)
}
```

### Fake `config.ProjectConfigLoader`

```go
type fakeLoader struct {
    secrets []string
}

func (f *fakeLoader) Load(_ string) (*workspace.WorkspaceConfiguration, error) {
    cfg := &workspace.WorkspaceConfiguration{}
    if len(f.secrets) > 0 {
        s := make([]string, len(f.secrets))
        copy(s, f.secrets)
        cfg.Secrets = &s
    }
    return cfg, nil
}
```

### Fake `config.Config` (workspace config)

```go
type fakeWorkspaceConfig struct {
    secrets  []string
    notFound bool
}

func (f *fakeWorkspaceConfig) Load() (*workspace.WorkspaceConfiguration, error) {
    if f.notFound {
        return nil, config.ErrConfigNotFound
    }
    cfg := &workspace.WorkspaceConfiguration{}
    if len(f.secrets) > 0 {
        s := make([]string, len(f.secrets))
        copy(s, f.secrets)
        cfg.Secrets = &s
    }
    return cfg, nil
}
```

### Testing `autoconfRunner` directly

Inject all fakes via `Options`; use a `*bytes.Buffer` for `out`:

```go
runner := autoconf.New(autoconf.Options{
    Detector:         &fakeDetector{detected: []autoconf.DetectedSecret{{...}}},
    Store:            &fakeStore{},
    ProjectUpdater:   &fakeProjectUpdater{},
    WorkspaceUpdater: &fakeWorkspaceUpdater{}, // omit to hide the local target
    ProjectID:        "test-project",
    Confirm:          func(string) (bool, error) { return true, nil },
    SelectTarget: func(_ string, _ []autoconf.ConfigTargetOption) (autoconf.ConfigTarget, error) {
        return autoconf.ConfigTargetGlobal, nil
    },
})
var buf bytes.Buffer
err := runner.Run(&buf)
```

## Vertex AI Detection for Claude (`ClaudeVertexAutoconf`)

In addition to the secret-based flow, `kdn autoconf` also runs a separate Vertex AI detection step for Claude workspaces. This is handled by `ClaudeVertexAutoconf` in `autoconfclaudevertex.go`.

### How it works

```text
VertexDetector.Detect()   → *VertexConfig (nil = not detected)
  ↓ non-nil
  findExistingLocations()  → skip if CLAUDE_CODE_USE_VERTEX already in agent or workspace cfg
  confirm?  (skipped when --yes)
  selectTarget()           → ClaudeVertexConfigTargetAgent | ClaudeVertexConfigTargetLocal
  applyTarget()
    Agent target  → AgentConfigUpdater.AddEnvVar("claude", ...) × 3 + AddMount
    Local target  → WorkspaceConfigUpdater.AddEnvVar(...) × 3 + AddMount
```

### Key types

**`VertexDetector`** (`detectclaudevertex.go`):
```go
type VertexDetector interface {
    Detect() (*VertexConfig, error)
}
```
Returns a non-nil `*VertexConfig` only when all three env vars (`CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`) are set and the credentials file exists on disk. The credentials file path is resolved in this priority order:
1. `GOOGLE_APPLICATION_CREDENTIALS` — if set and non-empty, the file at that path is used
2. Platform-specific ADC default — `~/.config/gcloud/application_default_credentials.json` (Linux/macOS) or `%APPDATA%\gcloud\...` (Windows)

Constructor: `NewVertexDetector() (VertexDetector, error)`.

**`VertexConfig`**:
```go
type VertexConfig struct {
    EnvVars     map[string]string // the three detected env var values
    ADCHostPath string            // host path for the credentials file mount
}
```

`ADCHostPath` is expressed as `$HOME/<rel>` when the credentials file is under the user's home directory (portable across machines); otherwise it is the absolute path as-is.

**`ClaudeVertexAutoconf`** (`autoconfclaudevertex.go`):
```go
type ClaudeVertexAutoconf interface {
    Run(out io.Writer) error
}
func NewClaudeVertexAutoconf(opts ClaudeVertexAutoconfOptions) ClaudeVertexAutoconf
```

Important `ClaudeVertexAutoconfOptions` fields:

| Field | Purpose |
|-------|---------|
| `Detector` | `VertexDetector` |
| `AgentUpdater` | `config.AgentConfigUpdater` — writes to `agents.json` |
| `WorkspaceUpdater` | `config.WorkspaceConfigUpdater` — nil means local target not offered |
| `AgentLoader` | `config.AgentConfigLoader` — checks if already configured in agent config |
| `WorkspaceConfig` | `config.Config` — checks if already configured in workspace config |
| `Yes` | skip confirm + select, defaults to agent target |
| `Confirm` / `SelectTarget` | injectable for testing |

**`ADCContainerPath`** — `$HOME/.config/gcloud/application_default_credentials.json` — the fixed container path where the credentials file is mounted, regardless of where it lives on the host. The helper `credHostPath(p, homeDir)` converts the host path to `$HOME/<rel>` when possible.

### Env var write order

The three env vars are always written in the order defined by `vertexEnvVars`:
1. `CLAUDE_CODE_USE_VERTEX`
2. `ANTHROPIC_VERTEX_PROJECT_ID`
3. `CLOUD_ML_REGION`

### Testing patterns

Fake `VertexDetector`:
```go
type fakeVertexDetector struct{ cfg *autoconf.VertexConfig }
func (f *fakeVertexDetector) Detect() (*autoconf.VertexConfig, error) { return f.cfg, nil }
```

Fake `AgentConfigUpdater`:
```go
type fakeAgentUpdater struct {
    envVars []struct{ agentName, name, value string }
    mounts  []struct{ agentName, host, target string; ro bool }
}
func (f *fakeAgentUpdater) AddEnvVar(agent, name, value string) error { ... }
func (f *fakeAgentUpdater) AddMount(agent, host, target string, ro bool) error { ... }
```

## Home Config File Detection (`HomeConfigFilesAutoconf`)

`kdn autoconf` also detects home-directory config files (e.g. `$HOME/.gitconfig`) and offers to mount them read-only into workspace containers. This is a separate flow from secret detection, handled by `HomeConfigFilesAutoconf` in `autoconfhomeconfigfiles.go`.

### How it works

```text
HomeConfigFilesDetector.Detect()   → []DetectedHomeConfigFile (files present on disk)
  ↓ per file
  findExistingLocations()            → skip with "already mounted" message if found
  confirm?  (skipped when --yes)
  selectTarget()                     → Global | Project | Local
  applyTarget()
    Global  → ProjectUpdater.AddMount("", hostPath, containerPath, true)
    Project → ProjectUpdater.AddMount(projectID, hostPath, containerPath, true)
    Local   → WorkspaceUpdater.AddMount(hostPath, containerPath, true)
```

Default when `--yes`: `HomeConfigFilesConfigTargetGlobal`.

### Registered files

`registeredHomeConfigFiles` in `detecthomeconfigfiles.go` is the authoritative list. Each entry is a `homeConfigFileSpec`:

```go
type homeConfigFileSpec struct {
    name             string // display identifier (e.g. "gitconfig")
    hostRelPath      string // path relative to $HOME on the host (Windows may differ)
    containerRelPath string // path relative to $HOME inside the Linux container
}
```

`hostRelPath` and `containerRelPath` are kept separate so Windows config files (e.g. `AppData/Roaming/tool/config`) can be mounted at the correct Linux container path (e.g. `.config/tool/config`). To register a new file, add one entry to the slice — no other changes needed.

`Detect()` stats `filepath.Join(homeDir, filepath.FromSlash(spec.hostRelPath))` for each entry, and returns:
```go
DetectedHomeConfigFile{
    Name:          spec.name,
    HostPath:      path.Join("$HOME", spec.hostRelPath),       // e.g. "$HOME/.gitconfig"
    ContainerPath: path.Join("$HOME", spec.containerRelPath),  // e.g. "$HOME/.gitconfig"
}
```

### Key types

**`HomeConfigFilesDetector`** (`detecthomeconfigfiles.go`):
```go
type HomeConfigFilesDetector interface {
    Detect() ([]DetectedHomeConfigFile, error)
}
```
Constructor: `NewHomeConfigFilesDetector() (HomeConfigFilesDetector, error)`. For tests: `newHomeConfigFilesDetectorWithInjection(homeDir, statFile, specs)`.

**`HomeConfigFilesAutoconf`** (`autoconfhomeconfigfiles.go`):
```go
type HomeConfigFilesAutoconf interface {
    Run(out io.Writer) error
}
func NewHomeConfigFilesAutoconf(opts HomeConfigFilesAutoconfOptions) HomeConfigFilesAutoconf
```

Important `HomeConfigFilesAutoconfOptions` fields:

| Field | Purpose |
|-------|---------|
| `Detector` | `HomeConfigFilesDetector` |
| `ProjectUpdater` | `config.ProjectConfigUpdater` — writes global or project entry to `projects.json` |
| `WorkspaceUpdater` | `config.WorkspaceConfigUpdater` — nil means local target not offered |
| `ProjectLoader` | `config.ProjectConfigLoader` — checks if already configured in global/project config |
| `WorkspaceConfig` | `config.Config` — checks if already configured in workspace config |
| `ProjectID` | empty string means project target not offered |
| `Yes` | skip confirm + select, defaults to global target |
| `Confirm` / `SelectTarget` | injectable for testing |

### Testing patterns

```go
type fakeHomeConfigFilesDetector struct {
    files []autoconf.DetectedHomeConfigFile
    err   error
}
func (f *fakeHomeConfigFilesDetector) Detect() ([]autoconf.DetectedHomeConfigFile, error) {
    return f.files, f.err
}

// ProjectConfigUpdater fake must implement both AddSecret and AddMount
type fakeProjectUpdater struct {
    mounts []struct{ projectID, host, target string; ro bool }
}
func (f *fakeProjectUpdater) AddSecret(projectID, secretName string) error { return nil }
func (f *fakeProjectUpdater) AddMount(projectID, host, target string, ro bool) error {
    f.mounts = append(f.mounts, ...)
    return nil
}

runner := autoconf.NewHomeConfigFilesAutoconf(autoconf.HomeConfigFilesAutoconfOptions{
    Detector:       &fakeHomeConfigFilesDetector{files: []autoconf.DetectedHomeConfigFile{{
        Name: "gitconfig", HostPath: "$HOME/.gitconfig", ContainerPath: "$HOME/.gitconfig",
    }}},
    ProjectUpdater: &fakeProjectUpdater{},
    Yes:            true,
})
var buf bytes.Buffer
err := runner.Run(&buf)
```

## Key Files

| File | Purpose |
|------|---------|
| `pkg/autoconf/detect.go` | `SecretDetector` interface + `envSecretDetector` |
| `pkg/autoconf/filter.go` | `SecretFilter` + `alreadyConfiguredFilter` + `FilterResult` |
| `pkg/autoconf/autoconf.go` | `Autoconf` interface + `autoconfRunner` + `ConfigTarget` |
| `pkg/autoconf/detectclaudevertex.go` | `VertexDetector` interface + `envVertexDetector` + `VertexConfig` |
| `pkg/autoconf/autoconfclaudevertex.go` | `ClaudeVertexAutoconf` interface + runner |
| `pkg/autoconf/adcpath.go` / `adcpath_windows.go` | Platform-specific ADC file path helpers |
| `pkg/autoconf/detecthomeconfigfiles.go` | `HomeConfigFilesDetector` interface + `envHomeConfigFilesDetector` + `registeredHomeConfigFiles` |
| `pkg/autoconf/autoconfhomeconfigfiles.go` | `HomeConfigFilesAutoconf` interface + runner + target constants |
| `pkg/cmd/autoconf.go` | Thin CLI wiring: flag parsing, dependency construction, `project.Detector` injection |
| `pkg/project/project.go` | `Detector` interface + shared project-ID detection logic (git remote → URL, no remote → path) |
| `pkg/config/projectsupdater.go` | `ProjectConfigUpdater` — reads/writes `~/.kdn/config/projects.json`; `AddMount` is idempotent by (host, target) pair |
| `pkg/config/workspaceupdater.go` | `WorkspaceConfigUpdater` — reads/writes `.kaiden/workspace.json` |
| `pkg/config/agents.go` | `AgentConfigUpdater` + `AgentConfigLoader` — reads/writes `~/.kdn/config/agents.json` |
| `pkg/secretservicesetup/register.go` | `ListServices()` — returns fully-constructed service instances |
| `pkg/secretservicesetup/secretservices.json` | Authoritative list of known secret services and their env vars |
