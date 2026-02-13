# Changelog

## 0.2 - 2026-02-13

Tested release baseline.

This version marks the current tested state, including:
- real Brave client validation (Debian Brave Flatpak + Android Brave Nightly)
- verified bookmark and tab-group sync in multi-device flow
- integration test suite passing with `GOWORK=off go test ./...`
- finalized Debian/Ubuntu deployment instructions and sample service/env files

## 0.1 - 2026-02-13

Initial `sync-lite` implementation, based on Brave's original `go-sync` server:
- Original repository: https://github.com/brave/go-sync
- Upstream latest release at time of writing: `v0.1.21` (Latest), by `@johnhalbert`, released on September 18, 2023
  - Source: https://github.com/brave/go-sync/releases

### What was added vs original `go-sync`
- New standalone subproject in `sync-lite/`.
- New lightweight HTTP server entrypoint in `sync-lite/main.go`.
- New primary endpoint `POST /command/`.
- Compatibility alias endpoint `POST /v2/command/`.
- SQLite datastore implementation in `sync-lite/sqlite_store.go` implementing Brave datastore behavior used by command handling.
- In-memory cache implementation in `sync-lite/memory_cache.go` replacing Redis.
- Local module file `sync-lite/go.mod` for independent build/run.
- Documentation for self-hosting and Brave endpoint setup in `sync-lite/README.md`.

### What was removed/simplified vs original `go-sync`
- Removed DynamoDB dependency.
- Removed Redis dependency.
- Removed Prometheus instrumentation/metrics endpoint.
- Removed health-check endpoint.
- Preserved core sync protobuf flow and auth behavior by reusing existing Brave sync logic.

### Validation notes
- Verified that on first run `sync-lite` initializes DB schema automatically via `initSchema(...)` during `NewSQLiteStore(...)`.
- No additional code change was required for schema bootstrap after review, so version remains `0.1`.

### 2026-02-13 updates after initial release
- Added optional native HTTPS support in `sync-lite/main.go`:
  - `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables.
  - If both are set, server starts with `ListenAndServeTLS`.
  - If only one is set, startup fails with explicit config error.
- Updated `sync-lite/README.md` with HTTPS configuration and Let's Encrypt path examples.
- Added deploy/runtime config helpers:
  - `sync-lite/deploy/sync-lite.env.example`
  - `sync-lite/deploy/sync-lite.service.example`
  - `sync-lite/scripts/run-with-env.sh`
- Updated `sync-lite/README.md` with recommended `systemd + EnvironmentFile` setup to avoid repeated shell exports.
- Added explicit SQLite WAL checkpoint behavior:
  - Startup checkpoint (`wal_checkpoint(TRUNCATE)`).
  - Graceful shutdown checkpoint on `SIGINT`/`SIGTERM` (including `Ctrl+C`).
- Added initial `sync-lite` integration test suite in `sync-lite/sync_lite_integration_test.go` covering:
  - unauthorized access handling
  - commit/get-updates roundtrip
  - disabled-chain behavior
  - gzip payload handling
- Expanded integration tests with multi-client scenarios:
  - chain isolation (client A data is not visible to client B)
  - same client-side entity ID committed by different clients without cross-client conflicts
- Updated testing docs to recommend `GOWORK=off go test ./...` for module-scoped test runs.
- Added a full Debian/Ubuntu deployment chapter to `sync-lite/README.md`:
  - DNS/NAT/certificate prerequisites
  - build/install/service setup steps
  - HTTP local test mode on `:8295`
  - HTTPS deployment mode on `:8295`
  - Brave URL examples for both modes
- Updated `sync-lite/deploy/sync-lite.env.example` default `LISTEN_ADDR` to `:8295`.
- Synced deployment docs/templates for consistency with tested flow:
  - `sync-lite/deploy/sync-lite.service.example` now uses `WorkingDirectory=/var/lib/sync-lite`
  - Brave sync URL examples use base URL form (`http(s)://host:port/`) per tested behavior
  - direct endpoint smoke test remains `curl .../command/`
  - env template now uses provider-agnostic TLS file path placeholders
