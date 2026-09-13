# Changelog

Newest entries first. Documentation-only entries do not change the binary version.
Historical entries retain their known dates; missing times/timezones are not
invented. February follow-up changes are grouped because individual timestamps
were not recorded.

## docs - 2026-09-13 19:13 CEST (Europe/Zagreb)

- Kept the tested release at `0.2`. Recorded the agreed next code version `0.3`,
  optional patch versions such as `0.3.1`, and integer minor increments (`0.9`
  to `0.10`). Removed obsolete numbering uncertainty and arranged this changelog
  as a newest-first timeline while preserving historical facts.
- Clarified upstream maintenance: merge official `brave/go-sync` changes into
  `luxzg/go-sync` after review and approval; our custom edits stay in `sync-lite/`.
  Added monthly review of upstream commits as well as releases. Custom conflict
  resolutions outside the folder require discussion; no unattended updates.
- Corrected development setup to clone the fork and build inside `sync-lite/`.
  Linked README deployment directly to the clone/run/test/build chapter, kept
  the tested binary/systemd layout and port `8295`, consolidated duplicate setup
  instructions, and replaced hostnames with generic examples. Retained the
  initial development workflow as historical prose rather than an active recipe.
- Recorded the user's September 13 field report: updated Debian Brave Flatpak
  and Android Brave Nightly APK clients still synchronize passwords, bookmarks,
  tabs, and tab groups. Sync resumes after extended downtime, service restarts,
  and server reboots when the service becomes reachable again.
- Updated the [Brave URL-path issue](https://github.com/brave/brave-browser/issues/48909)
  to closed, referencing the [merged client fix](https://github.com/brave/brave-core/pull/37237).
  Retained both server endpoint aliases and Brave base-URL instructions. The
  upstream [release listing](https://github.com/brave/go-sync/releases) still
  identifies `v0.1.21` as latest; this does not describe all branch activity.
- Recorded Linux x86-64 as the supported target. Archived the reported MIPS
  cross-build failure and ARM/MIPS validation as deferred pending testing hardware;
  no SQLite driver replacement was selected.
- Updated TODO with accepted later-pass priorities: regression/lifecycle/TLS
  helpers, upgrade safety, Go/SQLite maintenance, govulncheck, ShellCheck, build
  and backup/update helpers, investigation of the unverified excessive-write
  report, and design of an optional chain admission allowlist.
- Strengthened data-safety guidance: consistent backups, synthetic old/new
  database validation, explicit migration decisions, and rollback compatibility
  before deployment. No automatic restoration or changes to live user data.

Scope: documentation only. Reviewed local documentation and public upstream
issue/release metadata; checked intended documentation changes for consistency
and whitespace. No code, configuration, secrets, or private data were inspected;
no builds, runtime tests, tool installs, upstream merges, deployments, commits,
or pushes were performed. Service version remains `0.2`.

## docs - 2026-09-13 17:41 CEST (Europe/Zagreb)

Aligned sync-lite documentation with installed `codex-coding-rules` revision
`2026-09-12.2`. Added local `AGENTS.md`, an open TODO list, and a completed/deferred
work archive. Preserved the tested deployment chapter, historical records, and
release version `0.2`; no new code release was implied.

Clarified the accepted upstream dependency tradeoff: removal notes refer to
external runtime services, not complete removal of their Go dependencies.
Recorded Brave base URL handling, the external certificate-copy/restart workaround,
and the custom-change boundary. Marked the original development recipe and
external release/issue status statements as historical at that stage.

Recorded the user's 2026-05-26 report of successful desktop/phone sync after a
couple of months. Qualified the earlier encryption and test claims by their
observed scope. The May field report was not a new September runtime test.

Flagged future release numbering and source-distribution instructions as
unresolved at that point; the later entry above records their resolution.
Proposed focused helper, regression, lifecycle, maintenance, and recovery work
without implementing it. Validation was documentation reading and review only;
no code/configuration/private data inspection, builds, tests, installs,
deployments, commits, or pushes were performed.

## 0.2 - 2026-02-13 (time/timezone not recorded)

Tested release baseline.

This version marks the current tested state, including:
- real Brave client validation (Debian Brave Flatpak + Android Brave Nightly)
- verified bookmark and tab-group sync in multi-device flow
- integration test suite passing with `GOWORK=off go test ./...`
- finalized Debian/Ubuntu deployment instructions and sample service/env files

## 0.1 follow-up updates - 2026-02-13 (time/timezone not recorded)

Changes made after the initial implementation and included in the `0.2` tested
baseline above. The original record did not assign individual intermediate
release numbers.

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

## 0.1 - 2026-02-13 (time/timezone not recorded)

Initial `sync-lite` implementation, based on Brave's original `go-sync` server:
- Original repository: https://github.com/brave/go-sync
- Upstream latest release at initial implementation: `v0.1.21`, by `@johnhalbert`, released on September 18, 2023.
- Source: https://github.com/brave/go-sync/releases

### What was added vs original `go-sync`
- New standalone subproject in `sync-lite/`.
- New lightweight HTTP server entrypoint in `sync-lite/main.go`.
- New primary endpoint `POST /command/`.
- Compatibility alias endpoint `POST /v2/command/`.
- SQLite datastore in `sync-lite/sqlite_store.go` implementing Brave datastore behavior used by command handling.
- In-memory cache in `sync-lite/memory_cache.go` replacing the Redis service.
- Local module file `sync-lite/go.mod` for independent build/run within the fork.
- Self-hosting and Brave endpoint documentation in `sync-lite/README.md`.

### What was removed/simplified vs original `go-sync`
- Removed the external DynamoDB and Redis service requirements, not all inherited Go module dependencies.
- Removed Prometheus instrumentation/metrics and health-check endpoints from the lite service.
- Preserved core sync protobuf flow and auth behavior by reusing existing Brave sync logic.

### Validation notes
- The original review recorded automatic first-run schema initialization via `initSchema(...)` during `NewSQLiteStore(...)`.
- No additional code change was required for schema bootstrap, so that review retained version `0.1`.
