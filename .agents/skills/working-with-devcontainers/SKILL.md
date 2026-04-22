---
name: working-with-devcontainers
description: Guide to the devcontainers features package including the Feature/FeatureMetadata/FeatureOptions interfaces, OCI and local feature implementations, and the two-phase FromMap→Download→Order workflow
argument-hint: ""
---

# Working with Dev Container Features

The `pkg/devcontainers/features` package models, downloads, and orders Dev Container Features as defined by the [Dev Container spec](https://containers.dev/implementors/features/). A feature is a reusable environment component distributed as a directory containing `install.sh` and `devcontainer-feature.json`.

## Interfaces

All public types are interfaces; implementations are unexported.

```go
// Feature models a single feature that can be resolved to a local directory.
type Feature interface {
    ID() string
    Download(ctx context.Context, destDir string) (FeatureMetadata, error)
}

// FeatureMetadata holds data parsed from devcontainer-feature.json.
type FeatureMetadata interface {
    ContainerEnv() map[string]string  // env vars baked into the image
    Options() FeatureOptions
    InstallsAfter() []string          // always versionless IDs per spec
}

// FeatureOptions validates and merges user options with spec defaults.
type FeatureOptions interface {
    Merge(userOptions map[string]interface{}) (map[string]string, error)
}
```

## Two-Phase Workflow

```go
// Phase 1 – construct Feature instances (no I/O).
feats, userOptions, err := features.FromMap(cfg.Features, workspaceConfigDir)

// Phase 2 – download each feature into the build context.
metadata := make(map[string]features.FeatureMetadata, len(feats))
for i, feat := range feats {
    destDir := filepath.Join(buildContextDir, "features", strconv.Itoa(i))
    meta, err := feat.Download(ctx, destDir)
    metadata[feat.ID()] = meta
}

// Phase 3 – order for installation.
ordered, err := features.Order(feats, metadata)

// Phase 4 – emit Containerfile instructions using ordered features.
for i, feat := range ordered {
    merged, err := metadata[feat.ID()].Options().Merge(userOptions[feat.ID()])
    // COPY features/<i>/ /tmp/feature-<i>/
    // RUN for each merged env var + install.sh
}
```

## FromMap

`FromMap` classifies each feature ID and returns a deterministically sorted slice:

| ID pattern | Implementation | Notes |
|---|---|---|
| `./…` or `../…` | `localFeature` | resolved relative to `workspaceConfigDir` |
| `https?://…` | — | returns `"feature Tgz URI is not supported: <id>"` |
| anything else | `ociFeature` | OCI registry artifact |

`FromMap` does **no** network I/O. The returned slice is sorted by ID so behaviour is deterministic before `Order` runs.

## Order

`Order` runs Kahn's topological sort on `installsAfter` dependencies. Every feature in `feats` must have an entry in `metadata` (i.e. all features must be downloaded first).

**Version-stripped matching:** `installsAfter` values are always versionless per the spec (e.g. `ghcr.io/devcontainers/features/common-utils`). `Order` strips the version tag from registered feature IDs when resolving dependencies, so a feature registered as `…/common-utils:2` is correctly matched by an `installsAfter` entry of `…/common-utils`.

**Soft-dependency semantics:** Per the Dev Container spec, `installsAfter` is a hint — not a hard requirement. An `installsAfter` value that refers to a feature not present in `feats` (e.g. a feature the user didn't select, or one from a different layer) is silently ignored. This is intentional and distinct from a _missing metadata entry_, which is always an error because it means `Download` was not called before `Order`.

```go
ordered, err := features.Order(feats, metadata)
// err is non-nil on:
//   - a cycle in installsAfter dependencies
//   - a feature in feats with no corresponding metadata entry
// installsAfter references to features not in feats are silently ignored
```

## FeatureOptions.Merge

Key normalisation: keys are uppercased and runs of non-alphanumeric characters replaced with `_`.

```text
"install-tools" → "INSTALL_TOOLS"
"go.version"    → "GO_VERSION"
```

Type rules:
- `"string"` — value must be a Go `string`; validated against `enum` if present
- `"boolean"` — value is `bool` or `"true"`/`"false"` string; coerced to `"true"`/`"false"` in output
- Defaults from `devcontainer-feature.json` are applied before user options

```go
merged, err := meta.Options().Merge(map[string]interface{}{
    "version":       "20",
    "install-tools": true,
})
// merged["VERSION"] == "20"
// merged["INSTALL_TOOLS"] == "true"
```

## OCI Feature Implementation

`Download` on an `ociFeature`:

1. Parses the OCI reference — `[registry/]repository[:tag|@digest]`. First component is a registry if it contains `.` or `:` or equals `localhost`; otherwise defaults to `ghcr.io`.
2. Fetches the manifest (`application/vnd.oci.image.manifest.v1+json`). On HTTP 401 it parses the `WWW-Authenticate: Bearer` challenge and fetches an anonymous token from the realm URL.
3. Downloads each layer blob and extracts it via `extractTar`: peeks at the first two bytes — `0x1f 0x8b` → gzip-compressed tar, otherwise plain tar. (`application/vnd.devcontainers.layer.v1+tar` blobs are plain tar despite the name.)
4. Sanitises extracted paths against directory traversal.
5. Parses `devcontainer-feature.json` from `destDir` and returns it as `FeatureMetadata`.

The bearer token is reused for all subsequent blob fetches within the same `Download` call.

## Local Feature Implementation

For IDs beginning with `./` or `../`, `Download`:

1. Resolves the path relative to `workspaceConfigDir` using `filepath.FromSlash` for cross-platform safety.
2. Copies the directory tree into `destDir`.
3. Parses `devcontainer-feature.json` from `destDir`.

## Spec Coverage

### Feature ID formats

| Format | Status |
|---|---|
| OCI registry artifact (`ghcr.io/org/repo/feature:1`) | ✅ implemented |
| Local file tree (`./…`, `../…`) | ✅ implemented |
| HTTPS tarball URI (`https://…`) | ❌ not supported — `FromMap` returns an explicit error |

### devcontainer-feature.json fields

| Field | Status |
|---|---|
| `containerEnv` | ✅ parsed, returned via `ContainerEnv()` |
| `options` (type, default, enum) | ✅ parsed, returned via `Options()` |
| `installsAfter` | ✅ parsed, used by `Order()` |
| `id`, `version`, `name`, `description`, `documentationURL` | parsed by `devcontainerFeatureJSON` but not exposed in `FeatureMetadata` |
| `options.proposals` | parsed but not enforced — proposals are free-form UI hints, not a strict enum |
| `mounts` | ❌ not parsed — planned in a follow-up issue |
| `privileged` | ❌ not parsed — planned in a follow-up issue |
| `capAdd`, `securityOpt`, `init`, `entrypoint` | ❌ not parsed |
| `dependsOn` | ❌ not parsed — hard dependencies are not yet modelled; only `installsAfter` (soft) is used |
| `overrideFeatureInstallOrder` | ❌ not parsed — priority-based round ordering is not implemented |
| `customizations`, `legacyIds`, `deprecated`, `keywords`, `licenseURL` | ❌ not parsed |

### OCI support

| Capability | Status |
|---|---|
| Tag references (`:1`, `:latest`) | ✅ implemented |
| Digest references (`@sha256:…`) | ✅ implemented |
| Anonymous Bearer token auth (via `WWW-Authenticate` challenge) | ✅ implemented |
| Gzip-compressed tar layers (`0x1f 0x8b` magic) | ✅ implemented |
| Plain tar layers (`application/vnd.devcontainers.layer.v1+tar`) | ✅ implemented |
| Private / authenticated registries (username + password) | ❌ not implemented |
| OCI image index / multi-platform manifests | ❌ not implemented — `Download` will fail if the registry returns an index instead of a single manifest |
| Docker manifest v2 (`application/vnd.docker.distribution.manifest.v2+json`) | ❌ not implemented — only `application/vnd.oci.image.manifest.v1+json` is requested |

### Ordering

| Capability | Status |
|---|---|
| `installsAfter` soft-dependency ordering | ✅ implemented, with version-stripped ID matching; references to features not in `feats` are silently ignored per spec |
| Cycle detection | ✅ implemented |
| `dependsOn` hard-dependency ordering | ❌ not implemented |
| `overrideFeatureInstallOrder` priority-based sorting | ❌ not implemented |

### Out of scope for this package

Lifecycle hooks (`onCreateCommand`, `updateContentCommand`, `postCreateCommand`, `postStartCommand`, `postAttachCommand`) and `customizations` are container-orchestration concerns handled outside this package and are not modelled here.

## Testing

### Unit tests — local features as fixtures

Use a local feature directory in `t.TempDir()` to test `FeatureOptions.Merge` without network I/O:

```go
featureDir := filepath.Join(workspaceDir, "my-feature")
os.MkdirAll(featureDir, 0755)
data, _ := json.Marshal(map[string]interface{}{
    "options": map[string]interface{}{
        "version": map[string]interface{}{"type": "string", "default": "latest"},
    },
})
os.WriteFile(filepath.Join(featureDir, "devcontainer-feature.json"), data, 0644)

feats, _, _ := features.FromMap(
    map[string]map[string]interface{}{"./my-feature": nil},
    workspaceDir,
)
meta, _ := feats[0].Download(context.Background(), t.TempDir())
result, _ := meta.Options().Merge(nil)
// result["VERSION"] == "latest"
```

### Unit tests — Order with fakes

Implement the `Feature` and `FeatureMetadata` interfaces inline:

```go
type fakeFeature struct{ id string }
func (f *fakeFeature) ID() string { return f.id }
func (f *fakeFeature) Download(_ context.Context, _ string) (features.FeatureMetadata, error) {
    return nil, nil
}

type fakeMetadata struct{ installsAfter []string }
func (m *fakeMetadata) ContainerEnv() map[string]string  { return nil }
func (m *fakeMetadata) Options() features.FeatureOptions { return nil }
func (m *fakeMetadata) InstallsAfter() []string          { return m.installsAfter }
```

### Integration tests — OCI with httptest

To test OCI download without hitting a real registry, inject a custom `*http.Client` via `features.NewOCIFeatureWithClient` and redirect `https://` to a local `httptest.NewServer`:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // serve manifest and blobs
}))
defer srv.Close()

host := strings.TrimPrefix(srv.URL, "http://")
feat := features.NewOCIFeatureWithClient(host+"/repo/feature:tag", &http.Client{
    Transport: &rewriteTransport{host: host},
})
meta, err := feat.Download(ctx, destDir)
```

```go
type rewriteTransport struct{ host string }
func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    u := *req.URL
    u.Scheme, u.Host = "http", rt.host
    req2 := req.Clone(req.Context())
    req2.URL = &u
    return http.DefaultTransport.RoundTrip(req2)
}
```

### Integration tests against the real registry

Tag the file `//go:build integration` and name tests `TestIntegration_*`. Run with:

```bash
go test -tags integration -run TestIntegration_ -timeout 2m ./pkg/devcontainers/features/...
```

Real-registry tests live in `pkg/devcontainers/features/oci_integration_test.go`.
