# keylightd — TODO

## 1. Release Pipeline Migration

Migrate from the current custom `build-release.yml` to a pattern consistent with
[histui](https://github.com/jmylchreest/histui) and
[refyne-api](https://github.com/jmylchreest/refyne-api), leveraging the
[`jmylchreest/gha-extract-git-version@v1`](https://github.com/jmylchreest/gha-extract-git-version)
action.

**Current pipeline:** `.github/workflows/build-release.yml` (658 lines)
**Reference implementations:** `../histui/.github/workflows/build-release.yml`, `../refyne-api/.github/workflows/deploy.yml`

### 1.1 Version Extraction

- [ ] Replace inline bash version calculation with `jmylchreest/gha-extract-git-version@v1`
- [ ] Current format `X.Y.Z-hash-SNAPSHOT` is not semver-compliant
- [ ] Target format: `X.Y.Z-dev.N+hash` (semver prerelease + build metadata)
- [ ] Tag-safe format for snapshots: `vX.Y.Z-dev.N-hash` (no `+` in tags)
- [ ] Remove `workflow_dispatch` tag creation logic (the action handles both trigger types)

### 1.2 Prepare Job Improvements

- [ ] Add `should_skip` output to detect duplicate builds (tag push also triggers main push)
- [ ] Add concurrency control: `concurrency: { group: ${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: true }`
- [ ] Wire version outputs (`version`, `version_tag`, `is_release`, `is_prerelease`, `sha`, `date`, `dirty`) to downstream jobs
- [ ] Remove inline `$(date -u ...)` from ldflags — use action's `date` output instead

### 1.3 Build Improvements

- [ ] Add `fail-fast: false` to all matrix strategies
- [ ] Add Go build caching via `actions/cache@v5`
- [ ] Upgrade to `actions/checkout@v6`, `actions/upload-artifact@v6`, `actions/download-artifact@v6`
- [ ] Add test execution (with race detector) to the build job
- [ ] Gate build jobs on `needs.prepare.outputs.should_skip != 'true'`

#### Binary Builds (keylightd + keylightctl)

- [ ] Current matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` — all `CGO_ENABLED=0`
- [ ] Consider adding more architectures (e.g., `linux/386`, `linux/arm/v7`, `freebsd/amd64`) since there is zero CGO — pure Go cross-compilation works for any `GOOS/GOARCH`
- [ ] No zig needed — these binaries have no CGO dependencies

#### Tray App Builds (keylightd-tray)

- [ ] Current matrix: `linux/amd64`, `linux/arm64`, `darwin/universal`, `windows/amd64`, `windows/arm64`
- [ ] Requires Wails (CGO for WebKit on Linux, native webview on macOS/Windows)
- [ ] Cross-compilation with zig may enable building from fewer runners if needed
- [ ] Pin Wails CLI version in CI instead of `@latest` to match `go.mod` (`v2.11.0`)
- [ ] Fix archive naming inconsistency: tray uses `v` prefix (`keylightd-tray_v${VERSION}_...`), binaries don't

#### GNOME Extension

- [ ] No changes needed — `make zip` build is straightforward

### 1.4 Release Job

- [ ] Switch from `softprops/action-gh-release@v2` to `gh release create` (native GitHub CLI)
- [ ] Make snapshot retention configurable via `workflow_dispatch` input (currently hardcoded to 5)
- [ ] Update snapshot tag pattern to match new semver format (`v*-dev.*` instead of `*-SNAPSHOT`)
- [ ] Add explicit `permissions: contents: write`
- [ ] Keep SBOM generation via syft (histui doesn't have this — keylightd is ahead here)
- [ ] Add deployment summary to `$GITHUB_STEP_SUMMARY`

### 1.5 Publishing Jobs

#### AUR (`publish-aur`)

- [ ] Current: publishes `keylightd-bin` and `keylightd-tray-bin` (prebuilt binaries)
- [ ] Consider adding source-build `keylightd` package (like histui does) for better AUR trust
- [ ] Update PKGBUILD templates to use new version format
- [ ] Use `github-actions[bot]` identity instead of `goreleaserbot`

#### Homebrew (`publish-homebrew`)

- [ ] Update formula templates for new version format
- [ ] No structural changes needed

#### GNOME Extensions (`publish-ego`)

- [ ] No structural changes needed — already has change detection and `continue-on-error: true`

### 1.6 Additional Considerations

- [ ] Add permissions block to workflow (least privilege)
- [ ] Consider adding a `lint` job using golangci-lint (config already exists in `.golangci.yml`)
- [ ] Evaluate whether the `test.yml` workflow should be consolidated or kept separate

---

## 2. HTTP API Migration: Chi + Huma

Migrate from `http.ServeMux` to `go-chi/chi/v5` + `danielgtaylor/huma/v2` (via `humachi` adapter).
Reference implementation: `../refyne-api/api/`.

**Goal:** Typed request/response structs, auto-generated OpenAPI spec, Chi middleware ecosystem.

### 2.1 API Surface (13 routes, all under `/api/v1/`)

All routes must be preserved exactly for backward compatibility with the GNOME extension,
tray app HTTP client, and any third-party integrations.

| Method | Route | GNOME Ext | Tray App | Notes |
|--------|-------|-----------|----------|-------|
| `GET` | `/api/v1/lights` | Yes | Yes | Returns `map[string]*Light` (object keyed by ID) |
| `GET` | `/api/v1/lights/{id}` | Yes | Yes | |
| `POST` | `/api/v1/lights/{id}/state` | Yes | Yes | Body: `{on?, brightness?, temperature?}` |
| `GET` | `/api/v1/groups` | Yes | Yes | Returns `[]*Group` (array) |
| `POST` | `/api/v1/groups` | Yes | Yes | Body: `{name, light_ids?}` |
| `GET` | `/api/v1/groups/{id}` | Yes | Yes | |
| `DELETE` | `/api/v1/groups/{id}` | Yes | Yes | Returns 204 No Content |
| `PUT` | `/api/v1/groups/{id}/lights` | Yes | Yes | Body: `{light_ids}` |
| `PUT` | `/api/v1/groups/{id}/state` | Yes | Yes | Supports comma-separated IDs; 207 on partial failure |
| `POST` | `/api/v1/apikeys` | No | Yes | |
| `GET` | `/api/v1/apikeys` | No | Yes | |
| `DELETE` | `/api/v1/apikeys/{key}` | No | Yes | Returns 204 No Content |
| `PUT` | `/api/v1/apikeys/{key}/disabled` | No | Yes | |

### 2.2 Backward Compatibility Constraints

- [ ] **204 No Content** — DELETE endpoints must return empty bodies (GNOME extension handles this explicitly)
- [ ] **207 Multi-Status** — group state endpoint must support partial failure responses; Huma may need custom handling
- [ ] **Error format** — currently plain text via `http.Error()`; if switching to RFC 7807 Problem Details, GNOME extension still works (logs error body as string) but document the change
- [ ] **Lights as map** — `GET /api/v1/lights` returns `map[string]*Light`, NOT an array; preserve this
- [ ] **Comma-separated group IDs** — `PUT /groups/{id}/state` splits on commas; preserve this
- [ ] **1MB body limit** — `http.MaxBytesReader(w, r.Body, 1<<20)` on all body-reading handlers
- [ ] **Auth headers** — both `Authorization: Bearer <key>` and `X-API-Key: <key>` must work

### 2.3 Implementation Plan

- [ ] Add dependencies: `go-chi/chi/v5`, `danielgtaylor/huma/v2`, `go-chi/httprate`
- [ ] Create `internal/http/routes/config.go` — `NewHumaConfig()` with OpenAPI metadata, security schemes, tags
- [ ] Create `internal/http/routes/register.go` — central `Register(api, handlers)` with all route definitions
- [ ] Create `internal/http/routes/handlers.go` — handler interfaces aggregated in a `Handlers` struct
- [ ] Create `internal/http/routes/stubs.go` — stub implementations returning `nil, nil` for OpenAPI generation
- [ ] Create `internal/http/mw/` — auth middleware (API key validation), rate limiting, request logging
- [ ] Create `internal/http/handlers/` — typed Huma handlers with input/output structs
- [ ] Create `cmd/keylight-openapi/main.go` — OpenAPI spec exporter (CI-only tool, not a release artifact)
- [ ] Move HTTP handler logic from `internal/server/server.go` to new handler package
- [ ] Update `internal/server/server.go` to use Chi router instead of `http.ServeMux`
- [ ] Update `pkg/client/http_client.go` if response shapes change
- [ ] Add rate limiting via `go-chi/httprate`
- [ ] Keep Unix socket protocol in `server.go` unchanged (separate from HTTP)

### 2.4 OpenAPI Spec Generation (CI only)

- [ ] `cmd/keylight-openapi/main.go` creates a minimal chi router, registers routes with stub handlers, serializes `api.OpenAPI()` to JSON
- [ ] Add CI step to generate and upload OpenAPI spec as build artifact
- [ ] Embed spec in documentation deployment
- [ ] **Not** a release binary — CI-only tool

### 2.5 Testing

- [ ] Port existing `internal/server/` tests to work with Chi router
- [ ] Add integration tests for each endpoint with typed request/response structs
- [ ] Test GNOME extension compatibility: 204 responses, 207 multi-status, map vs array returns
- [ ] Test both auth header formats (`Authorization: Bearer` and `X-API-Key`)

---

## 3. Structured Logging with slog-logfilter

Integrate [`github.com/jmylchreest/slog-logfilter`](https://github.com/jmylchreest/slog-logfilter)
for dynamic, filter-based log level overrides with hot-reload support.

### 3.1 Integration

- [ ] Add `github.com/jmylchreest/slog-logfilter` to `go.mod`
- [ ] Replace `internal/utils/logging.go` setup to use `logfilter.SetDefault()` with options:
  - `logfilter.WithLevel()` — from config
  - `logfilter.WithFormat()` — from config (`"json"` or `"text"`)
  - `logfilter.WithSource(true)` — enable source location
  - `logfilter.WithFilters()` — from config
- [ ] Wire up in `cmd/keylightd/main.go` daemon startup

### 3.2 Configuration

- [ ] Add `log_filters` section to config (`internal/config/config.go`):
  ```yaml
  logging:
    level: info
    format: json
    filters:
      - type: "source:file"
        pattern: "internal/server/*"
        level: "debug"
        enabled: true
      - type: "source:function"
        pattern: "*discovery*"
        level: "debug"
        enabled: true
  ```
- [ ] Filters stored as `[]logfilter.LogFilter` (JSON-serializable struct)

### 3.3 Validation (package has none — we must add it)

The `slog-logfilter` package has **no validation** — no error returns on any filter mutation,
invalid patterns silently don't match, `ParseLevel()` silently defaults to `slog.LevelInfo`.
We must validate before applying:

- [ ] Create `internal/logging/validate.go` with filter validation:
  - `Type` must be a known attribute key, `"source:file"`, `"source:function"`, or `"context:*"`
  - `Pattern` must be non-empty
  - `Level` must be one of `"debug"`, `"info"`, `"warn"`, `"error"`
  - `OutputLevel` (if set) must also be a valid level
  - `ExpiresAt` (if set) must be in the future
- [ ] Return structured errors listing all invalid filters (don't fail on first)
- [ ] Log warnings for invalid filters but don't crash the daemon

### 3.4 Hot-Reload

The package fully supports runtime filter changes via mutex-protected methods:
`SetFilters()`, `AddFilter()`, `RemoveFilter()`, `ClearFilters()`, `SetLevel()`.

- [ ] Watch config file for changes (use existing `fsnotify` dependency or viper's built-in watcher)
- [ ] On config change: parse new filters → **validate** → apply only if valid
- [ ] Log the filter change at INFO level (what was added/removed/modified)
- [ ] If validation fails: log the errors at WARN, keep existing filters unchanged
- [ ] Expose filter management via HTTP API (new endpoints under `/api/v1/logging/`):
  - `GET /api/v1/logging/filters` — list current filters
  - `PUT /api/v1/logging/filters` — replace all filters (validated)
  - `PUT /api/v1/logging/level` — change global log level at runtime
- [ ] Consider adding temporary debug filters via API with `ExpiresAt` for live debugging

---

## 4. Tray App Visual/UX Improvements

Location: `contrib/keylightd-tray/` (Wails v2, vanilla JS, Vite 5, CSS variables / Catppuccin Mocha)

### 4.1 Slider Improvements

- [ ] **Slider fill visualization** — track shows no filled portion; add accent color fill up to thumb position
- [ ] **Temperature gradient** — temperature slider should show warm-amber to cool-blue gradient

### 4.2 Light/Group Card States

- [ ] **On/off state differentiation** — cards need stronger visual distinction (background/border/glow, not just opacity)
- [ ] **Transition animations** — `innerHTML` replacement destroys DOM; implement smooth state transitions

### 4.3 Layout & Space Efficiency

- [ ] **Hide empty sections** — groups heading shows even with zero groups; hide in 380x600 window
- [ ] **Collapsible sections** — groups/lights should collapse to save vertical space
- [ ] **Remove footer** — version string wastes space; already available in Settings > About

### 4.4 Controls & Interactions

- [ ] **Master toggle** — add "all on/all off" button in header
- [ ] **Settings panel animation** — full-screen overlay with no animation; add slide or backdrop blur

---

## 5. Dependency Maintenance

### 5.1 fyne.io/systray Replace Directive

`go.mod` pins `fyne.io/systray` to commit `4856ac3adc3c` (Aug 12, 2025) via a `replace` directive
for `SetOnTapped`/`SetOnSecondaryTapped` support. As of Feb 2026, `v1.12.0` has not been released.

- [ ] Monitor for `fyne.io/systray` v1.12.0+ release
- [ ] When released: `go mod edit -dropreplace fyne.io/systray && go get fyne.io/systray@v1.12.0`

### 5.2 Wails CLI Version Pinning

CI installs Wails CLI with `@latest`, which may diverge from the `v2.11.0` library in `go.mod`.

- [ ] Pin Wails CLI install to `@v2.11.0` in CI
- [ ] Monitor Wails v3 stable release status (currently alpha)
