---
name: working-with-secrets
description: Guide to the secrets abstraction including the Store, SecretService registry, and how to add new named secret types
argument-hint: ""
---

# Working with Secrets

The secrets system uses a two-layer architecture: a **Store** that persists secrets securely, and a **SecretService registry** that describes how each named type maps to HTTP requests inside workspaces.

## Overview

- Secret **values** live exclusively in the system keychain — never on disk
- Non-sensitive **metadata** (type, hosts, path, header descriptors) is persisted to `<storage-dir>/secrets.json`
- Named types (e.g. `github`) derive all their descriptor fields from a registered `SecretService`
- The built-in `other` type requires the user to supply all descriptor fields explicitly

## Key Components

- **Store interface** (`pkg/secret/secret.go`): `Create(CreateParams) error` — the only operation currently exposed
- **Store implementation** (`pkg/secret/store.go`): writes the value to the keychain, metadata to `secrets.json`
- **SecretService interface** (`pkg/secretservice/secretservice.go`): describes a named type — host pattern, path, header name, header template, env vars
- **Registry** (`pkg/secretservice/registry.go`): maps names to `SecretService` implementations
- **Centralized registration** (`pkg/secretservicesetup/register.go`): loads definitions from the embedded `secretservices.json` and exposes `ListAvailable()` and `RegisterAll()`

## Storage Layout

```text
<storage-dir>/
  secrets.json      # metadata only — no secret values on disk
```

Keychain entry: service=`kdn`, user=`<secret-name>`, password=`<secret-value>`

The keychain backend is platform-specific: GNOME Keyring on Linux, Keychain on macOS, DPAPI on Windows (via `github.com/zalando/go-keyring`).

## Secret Types

| Type | How descriptor fields are resolved |
|------|------------------------------------|
| Named (e.g. `github`) | Taken from the registered `SecretService` automatically |
| `other` | User must supply `--host`, `--path`, `--header`, `--headerTemplate` |

## Using the Store

```go
import "github.com/openkaiden/kdn/pkg/secret"

store := secret.NewStore(absStorageDir)

err := store.Create(secret.CreateParams{
    Name:        "my-token",
    Type:        "github",
    Value:       "ghp_xxxx",
    Description: "Personal access token",
})
```

For `other` type, also supply the descriptor fields:

```go
err := store.Create(secret.CreateParams{
    Name:           "my-api-key",
    Type:           secret.TypeOther,
    Value:          "secret123",
    Hosts:          []string{"api.example.com"},
    Path:           "/v1",
    Header:         "Authorization",
    HeaderTemplate: "Bearer ${value}",
})
```

`Create` checks for a duplicate name before touching the keychain. `ErrSecretAlreadyExists` is returned if the name is already taken.

## Adding a New Named Secret Type

Add an entry to `pkg/secretservicesetup/secretservices.json`:

```json
{
  "name": "my-service",
  "hostPattern": "api\\.my-service\\.com",
  "headerName": "Authorization",
  "headerTemplate": "Bearer ${value}",
  "envVars": ["MY_SERVICE_TOKEN"]
}
```

All fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Identifier used as `--type` value |
| `hostPattern` | yes | Regex matched against request host |
| `headerName` | yes | HTTP header to set |
| `headerTemplate` | yes | Header value template; `${value}` is replaced with the secret value |
| `path` | no | URL path prefix restriction |
| `envVars` | no | Environment variable names to populate with the secret value |

No code changes required. Once added, the type is immediately:
- Accepted by `kdn secret create --type my-service`
- Listed in `--type` shell completion
- Returned by `secretservicesetup.ListAvailable()`

## Deriving Valid Types in Commands

Commands get valid type names from `secretservicesetup.ListAvailable()` rather than a hardcoded list. The built-in `other` type is appended separately:

```go
import (
    "github.com/openkaiden/kdn/pkg/secret"
    "github.com/openkaiden/kdn/pkg/secretservicesetup"
)

registeredTypes := secretservicesetup.ListAvailable()
sort.Strings(registeredTypes)
validTypes := append(registeredTypes, secret.TypeOther)
```

Register shell completion from this list:

```go
cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
    return validTypes, cobra.ShellCompDirectiveNoFileComp
})
```

## Testing

Inject a `fakeKeyring` to avoid touching the real system keychain:

```go
// fakeKeyring is unexported in pkg/secret — use the package-internal
// newStoreWithKeyring constructor (available only within the package).
// From outside the package, test via the Store interface with a real temp dir
// and rely on the keychain failing (or use build tags to swap the backend).
```

For command-level tests that bypass `NewSecretCreateCmd()`, populate `validTypes` directly on the struct:

```go
c := &secretCreateCmd{
    secretType: "github",
    value:      "ghp_token",
    validTypes: []string{"github", secret.TypeOther},
}
```
