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

## Key Files

| File | Purpose |
|------|---------|
| `pkg/autoconf/detect.go` | `SecretDetector` interface + `envSecretDetector` |
| `pkg/autoconf/filter.go` | `SecretFilter` + `alreadyConfiguredFilter` + `FilterResult` |
| `pkg/autoconf/autoconf.go` | `Autoconf` interface + `autoconfRunner` + `ConfigTarget` |
| `pkg/cmd/autoconf.go` | Thin CLI wiring: flag parsing, dependency construction, `detectProjectID` |
| `pkg/config/projectsupdater.go` | `ProjectConfigUpdater` — reads/writes `~/.kdn/config/projects.json` |
| `pkg/config/workspaceupdater.go` | `WorkspaceConfigUpdater` — reads/writes `.kaiden/workspace.json` |
| `pkg/secretservicesetup/register.go` | `ListServices()` — returns fully-constructed service instances |
| `pkg/secretservicesetup/secretservices.json` | Authoritative list of known secret services and their env vars |
