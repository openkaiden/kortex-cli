---
name: working-with-onecli
description: Guide to the OneCLI package including the Client, CredentialProvider, SecretMapper, and SecretProvisioner interfaces, how they integrate with the Podman runtime, and the deny-mode networking / approval-handler sidecar design
argument-hint: ""
---

# Working with OneCLI

The `pkg/onecli` package provides a typed HTTP client for the OneCLI API, plus three higher-level abstractions that the Podman runtime uses to provision secrets and configure networking for workspace containers.

## Overview

OneCLI is an HTTP proxy service that runs alongside the workspace container. It:

- Intercepts outbound HTTP requests and injects secret values as headers
- Enforces network rules (allow/block/rate-limit per host pattern)
- Exposes `/api/container-config` which the Podman runtime reads to inject proxy environment variables and a CA certificate into the workspace container

## Key Interfaces

All four public types are interfaces; concrete implementations are unexported.

| Interface | Factory | Purpose |
|-----------|---------|---------|
| `Client` | `NewClient(baseURL, apiKey)` | Raw CRUD against the OneCLI API |
| `CredentialProvider` | `NewCredentialProvider(baseURL)` | Retrieves the `oc_` API key from `/api/user/api-key` |
| `SecretMapper` | `NewSecretMapper(registry)` | Converts `secret.ListItem` + value → `CreateSecretInput` |
| `SecretProvisioner` | `NewSecretProvisioner(client)` | Creates or updates secrets via `Client`, handles 409 conflicts |

## Client

`NewClient(baseURL, apiKey string) Client` — 30-second timeout, Bearer auth header.

### Secrets API

```go
// Create a secret; returns the created Secret or an *APIError.
secret, err := client.CreateSecret(ctx, onecli.CreateSecretInput{
    Name:        "github",
    Type:        "generic",
    Value:       "ghp_xxxx",
    HostPattern: "api.github.com",
    InjectionConfig: &onecli.InjectionConfig{
        HeaderName:  "Authorization",
        ValueFormat: "Bearer {value}",
    },
})

// Update an existing secret by ID (all fields optional).
err = client.UpdateSecret(ctx, secret.ID, onecli.UpdateSecretInput{
    Value: ptr("ghp_new"),
})

// List all secrets.
secrets, err := client.ListSecrets(ctx)

// Delete by ID.
err = client.DeleteSecret(ctx, secret.ID)
```

### Container Config

```go
cfg, err := client.GetContainerConfig(ctx)
// cfg.Env — map of proxy env vars to inject into the workspace container
// cfg.CACertificate — PEM-encoded CA cert
// cfg.CACertificateContainerPath — path where the cert should be written inside the container
```

### Networking Rules API

Valid `Action` values are `"block"`, `"rate_limit"`, and `"manual_approval"`. `"allow"` is **not** a valid action — OneCLI rejects it.

```go
// Create a catch-all manual_approval rule (used for deny-mode networking).
rule, err := client.CreateRule(ctx, onecli.CreateRuleInput{
    Name:        "manual-approval-all",
    HostPattern: "*",
    Action:      "manual_approval",
    Enabled:     true,
})

rules, err := client.ListRules(ctx)
err = client.DeleteRule(ctx, rule.ID)
```

### Error Handling

Non-2xx responses return `*APIError{StatusCode int, Message string}`. Check with `errors.As`:

```go
var apiErr *onecli.APIError
if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
    // handle 409
}
```

## CredentialProvider

`NewCredentialProvider(baseURL string) CredentialProvider` — 10-second timeout, no auth required on the bootstrap call (local mode creates the user on first access).

```go
provider := onecli.NewCredentialProvider("http://localhost:8080")
apiKey, err := provider.APIKey(ctx)
// apiKey starts with "oc_"
```

## SecretMapper

`NewSecretMapper(registry secretservice.Registry) SecretMapper` — converts a `secret.ListItem` (metadata from the Store) and its plaintext value into a `CreateSecretInput` ready for the OneCLI API.

```go
mapper := onecli.NewSecretMapper(secretServiceRegistry)
inputs, err := mapper.Map(item, value) // item: secret.ListItem, value: string from keychain; returns []CreateSecretInput
```

### Mapping rules

- **Known type** (e.g. `github`): looks up the `SecretService` in the registry; uses its `HostsPatterns()`, `Path()`, `HeaderName()`, and `HeaderTemplate()` fields. If the service has a single host pattern, returns a single-element slice; if multiple patterns, returns one `CreateSecretInput` per pattern with the name `<secret-name>-<sanitized-pattern>`. Returns an error if `HostsPatterns()` is empty.
- **`other` type**: uses the secret's own `Hosts`, `Path`, `Header`, `HeaderTemplate` fields. When multiple hosts are provided, one `CreateSecretInput` is returned per host with the name `<secret-name>-<sanitized-host>`; a single or empty `Hosts` returns a single element using `item.Name` unchanged.
- Template conversion: kdn uses `${value}`, OneCLI uses `{value}` — the mapper converts automatically.
- `HostPattern` is `"*"` for `other` type when `Hosts` is nil or empty.

## SecretProvisioner

`NewSecretProvisioner(client Client) SecretProvisioner` — idempotent: creates a secret or, on 409, finds it by name and patches it.

```go
provisioner := onecli.NewSecretProvisioner(client)
err := provisioner.ProvisionSecrets(ctx, []onecli.CreateSecretInput{input1, input2})
```

On conflict the provisioner calls `ListSecrets` to find the ID, then `UpdateSecret`. It returns an error if the named secret cannot be found after a 409.

## Integration: Podman Runtime

The Podman runtime is the primary consumer of this package. The flow during workspace creation and start is:

### Workspace creation (`pkg/runtime/podman/create.go` — `setupOnecli`)

1. `NewCredentialProvider(baseURL).APIKey(ctx)` — get the API key after OneCLI starts
2. `NewClient(baseURL, apiKey)` — create the client
3. `NewSecretProvisioner(client).ProvisionSecrets(ctx, secrets)` — push secrets
4. `client.GetContainerConfig(ctx)` — retrieve proxy env vars and CA cert to inject into the workspace container

### Workspace start (`pkg/runtime/podman/network.go` — `configureNetworking`)

Only runs when the workspace config has `network.mode = deny` **and** at least one host in `network.hosts`. All other cases (allow mode, no config, deny with empty hosts) call `clearNetworkingRules` instead to remove leftover rules.

1. `NewCredentialProvider(baseURL).APIKey(ctx)`
2. `NewClient(baseURL, apiKey)`
3. `client.ListRules(ctx)` + `client.DeleteRule(ctx, id)` — wipe stale rules (idempotency)
4. `client.CreateRule(ctx, CreateRuleInput{HostPattern: "*", Action: "manual_approval"})` — single catch-all rule; individual per-host allow rules are **not** used because `"allow"` is not a valid OneCLI action
5. Write `config.json` to the approval-handler directory (see Deny-mode Networking below)
6. The approval-handler sidecar container is then started by `Start()` — it reads `config.json` and connects to the OneCLI gateway to approve/deny each intercepted request

The network policy is read fresh from `workspace.json` + `projects.json` on every `Start()`, so it takes effect without recreating the workspace.

### Secret flow from manager (`pkg/instances/manager.go`)

The instances manager resolves each secret name from the Store, maps it to a `CreateSecretInput`, and collects any associated environment variable names:

```go
mapper := onecli.NewSecretMapper(m.secretServiceRegistry)
for _, name := range *mergedConfig.Secrets {
    item, value, err := m.secretStore.Get(name)   // metadata + plaintext value
    inputs, err := mapper.Map(item, value)         // → []CreateSecretInput — one per host for type=other
    onecliSecrets = append(onecliSecrets, inputs...)

    // Also collect env var names exposed by this secret type
    // (used by SecretEnvVars in runtime.CreateParams)
}
// runtime.CreateParams.OnecliSecrets = onecliSecrets
```

## Deny-mode Networking

When a workspace is configured with `network.mode = deny`, outbound HTTP traffic from the agent container is intercepted by the OneCLI proxy. A single `manual_approval` rule covering all hosts (`*`) is created. Every intercepted request is held by the OneCLI gateway until the approval-handler sidecar approves or denies it.

### Architecture

```
agent container
  │  (HTTP_PROXY → OneCLI)
  ▼
OneCLI gateway (port 10255) ──► approval-handler sidecar
                                  (polls gateway, approves/denies per hosts list)
```

### Approval-handler sidecar

The sidecar is a TypeScript script (`pkg/runtime/podman/pods/approval-handler.ts`) that runs inside the pod as a UBI Node.js 22 container. It uses the `@onecli-sh/sdk` package.

**Startup sequence (in `Start()`):**

1. `configureNetworking` writes `config.json` to the approval-handler directory on the host (mounted at `/app` inside the container)
2. `podman start <pod>-approval-handler` — container copies `/app/*` to its working directory and runs the script

**`config.json` format** (written by `configureNetworking`, never edited manually):

```json
{
  "onecliUrl":  "http://localhost:10254",
  "gatewayUrl": "http://localhost:10255",
  "apiKey":     "oc_...",
  "hosts":      ["api.github.com", "*.example.com"]
}
```

- `onecliUrl` / `gatewayUrl` use the internal container ports, not the host-mapped ports
- `apiKey` is fetched via `CredentialProvider.APIKey()` (calls `GET /api/user/api-key`)
- `hosts` comes from `network.hosts` in the workspace config

**If `config.json` is absent** the script exits immediately with `"no config.json found, exiting (allow mode)"` — this is expected when the workspace is in allow mode.

### Host matching

The approval-handler checks each request's hostname against the `hosts` list using glob patterns:

| Pattern | Approves |
|---------|---------|
| `*` | everything |
| `api.github.com` | exact match only |
| `*.github.com` | `api.github.com`, `cdn.github.com` but **not** `github.com` |

Only three forms are supported: catch-all `*`, leading-wildcard `*.domain`, or exact hostname. Mid-pattern wildcards like `api.*.com` are treated as literal strings and will never match.

Implementation (`approval-handler.ts`):

```typescript
function matchesPattern(pattern: string, hostname: string): boolean {
  if (pattern === "*") return true;
  if (pattern.startsWith("*.")) {
    const suffix = pattern.slice(1); // ".github.com"
    return hostname.endsWith(suffix) && hostname.length > suffix.length;
  }
  return pattern === hostname;
}
```

### Signal handling

The sidecar registers both `SIGTERM` (Linux/macOS) and `SIGINT` (Ctrl+C, required on Windows) to gracefully stop the SDK polling loop.

### Windows path translation (`pkg/runtime/podman/system`)

On Windows, Podman runs inside a WSL2 VM. Host paths (`C:\Users\...`) must be translated to their VM-side POSIX equivalents (`/mnt/c/Users/...`) before being written into the pod YAML `hostPath` field, and translated back when reading the stored path to write `config.json`.

```go
import podmanSystem "github.com/openkaiden/kdn/pkg/runtime/podman/system"

// In Create(): store the machine-side path in pod-template-data.json
ApprovalHandlerDir: podmanSystem.HostPathToMachinePath(approvalHandlerDir),

// In readPodTemplateData(): convert back to host path before writing config.json
tmplData.ApprovalHandlerDir = podmanSystem.MachinePathToHostPath(tmplData.ApprovalHandlerDir)
```

Both functions are no-ops on Linux and macOS (`//go:build !windows`). The Windows build (`//go:build windows`) performs the `C:\` ↔ `/mnt/c/` conversion.

## Testing

Use a `*httptest.Server` to serve fake API responses and pass its URL to `NewClient` or `NewCredentialProvider`. The `Client` interface makes it straightforward to inject a fake:

```go
type fakeClient struct{ ... }
func (f *fakeClient) CreateSecret(_ context.Context, input onecli.CreateSecretInput) (*onecli.Secret, error) { ... }
// implement remaining methods ...
var _ onecli.Client = (*fakeClient)(nil)

provisioner := onecli.NewSecretProvisioner(&fakeClient{})
```

For `SecretMapper` tests, use a `secretservice.NewRegistry()` and register a fake `SecretService` implementation, or use the real `secretservicesetup.RegisterAll()`.
