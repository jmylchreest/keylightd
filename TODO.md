# keylightd — TODO

## ~~1. Release Pipeline Migration~~ DONE

Migrated to `jmylchreest/gha-extract-git-version@v1` with semver-compliant versioning,
test gate, `gh release create`, concurrency control, deployment summary, and consistent
archive naming. See `.github/workflows/build-release.yml`.

<details>
<summary>Completed items</summary>

- [x] Version extraction via `gha-extract-git-version@v1` (format: `X.Y.Z-dev.N+hash`)
- [x] Tag-safe format for snapshots (`vX.Y.Z-dev.N-hash`)
- [x] Duplicate build detection (`should_skip`)
- [x] Concurrency control (cancel-in-progress)
- [x] Test gate with race detector before builds
- [x] `fail-fast: false` on all matrix strategies
- [x] Upgraded to `actions/checkout@v6`, `upload-artifact@v6`, `download-artifact@v7`, `setup-go@v6`
- [x] Switched from `softprops/action-gh-release@v2` to `gh release create`
- [x] Deployment summary via `$GITHUB_STEP_SUMMARY`
- [x] Explicit `permissions: contents: write`
- [x] SBOM generation retained
- [x] Consistent archive naming (no `v` prefix on tray archives)
- [x] `github-actions[bot]` identity for AUR/Homebrew publishing
- [x] Snapshot cleanup matches all prereleases

</details>

**Deferred:**
- [ ] Go build caching via `actions/cache@v5` (Go setup action handles this adequately)
- [ ] Source-build AUR package (low priority)
- [ ] Lint job with golangci-lint (separate concern)
- [ ] Additional binary architectures (low demand)

---

## ~~2. HTTP API Migration: Chi + Huma~~ DONE

Migrated to Chi + Huma with typed request/response structs, auto-generated OpenAPI spec,
Chi middleware ecosystem, and full backward compatibility. See `internal/http/`.

<details>
<summary>Completed items</summary>

- [x] All 13 routes preserved with exact backward compatibility
- [x] Typed Huma handlers with input/output structs
- [x] 204 No Content for DELETE endpoints
- [x] 207 Multi-Status for group state (partial failure)
- [x] Lights returned as map (not array)
- [x] Comma-separated group IDs supported
- [x] Both auth header formats (`Authorization: Bearer` and `X-API-Key`)
- [x] Auth middleware at Huma level (typed routes) and Chi level (raw 207/WebSocket routes)
- [x] Rate limiting via `go-chi/httprate`
- [x] Comprehensive test coverage (handlers + HTTP client)

</details>

---

## ~~3. Structured Logging with slog-logfilter~~ DONE

Integrated `slog-logfilter` with hot-reload, validation, and runtime filter management.

<details>
<summary>Completed items</summary>

- [x] `slog-logfilter` integration with `logfilter.SetDefault()`
- [x] Config-driven filters with validation
- [x] Hot-reload on config file changes
- [x] Filter validation (type, pattern, level, expiry)
- [x] Invalid filters logged at WARN, existing filters preserved
- [x] HTTP API for filter management (`/api/v1/logging/`)

</details>

---

## ~~4. Tray App Visual/UX Improvements~~ DONE

<details>
<summary>Completed items</summary>

- [x] Slider fill visualization with accent-color linear-gradient
- [x] Temperature gradient (warm-amber to cool-blue)
- [x] On/off card states (green-tinted on, reduced opacity off)
- [x] Collapsible sections (CSS max-height transition)
- [x] Hidden empty sections (groups auto-hides when empty)
- [x] Footer removed (version only in Settings > About)
- [x] Master toggle (power icon in header, green when any on)
- [x] Settings panel slide animation (translateX, 250ms)

</details>

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
