# Changelog

Newest entries first. Documentation-only entries do not change the binary version.
Historical entries retain their known dates; missing times/timezones are not
invented. February follow-up changes are grouped because individual timestamps
were not recorded.

## docs - 2026-10-19 22:32 CEST (Europe/Zagreb)

- Streamlined release, maintenance, and validation documentation around public
  features and supported workflows. Kept the optional diagnostic setting,
  privacy restrictions, backup guidance, and tested platform limits.
- Consolidated operator-reported two-way sync results and retained general
  accounting and client-tag lifecycle follow-up tasks.

Validation: reviewed Markdown links, documented commands, and the documentation
diff. No code, tests, dependencies, schema, or binary changes; version remains
`0.3.1`.

## docs - 2026-10-19 19:08 CEST (Europe/Zagreb)

- Recorded successful `0.3.1` field transfers: desktop bookmark to phone,
  phone-side rename back to desktop, and fresh groups in both directions.
  Exact delays were not measured.
  Recorded the reported Nightly/Chromium/Android versions separately from
  historical coverage, without personal device information.
- Updated remaining field-validation scope without implying complete coverage
  of all data types or platforms. No runtime change or build; service version
  remains `0.3.1`.

## 0.3.1 - 2026-10-19 18:39 CEST (Europe/Zagreb)

- Added opt-in `SYNC_LOG_CONFLICTS=1` diagnostics for missing-record,
  version-mismatch, duplicate-tag, and duplicate-ID conflicts. Logs contain only
  operation, reason, and numeric type, with a 30-second per-key rate limit and
  bounded memory. The default remains disabled; schema, data, authentication,
  write acceptance, and endpoint behavior are unchanged.
- Added synthetic protocol coverage for remote bookmark edits, opaque encrypted
  payloads, incremental polling, stale-version rejection/retry, and fresh saved
  groups/tabs from both devices. Extended the isolated upgrade helper with a
  reviewed source-baseline override for checking actual `0.3` as well as `0.2`.
- Added backup-first deployment and non-destructive diagnostic instructions.

Validation: both backend suites/race checks, diagnostic privacy/rejection and
rate-limit tests, HTTP/HTTPS lifecycle smoke checks, both govulncheck runs, and
clean/WAL upgrade/rollback matrices from actual `0.2` and `0.3` passed. All state
was synthetic. No production deployment or real-device test was performed.

## docs - 2026-10-19 18:06 CEST (Europe/Zagreb)

- Added MANUAL_TESTING with a two-device browser checklist: fresh uniquely
  named items, bookmark edits in the reverse direction, tabs/groups and optional
  categories, sync-internals views, delays, safe troubleshooting, and privacy.
  Linked it from README setup/upgrade/testing and the pending operator task.
  Diagnostic counters or Nigori keys alone are not treated as proof of arrival.
- Version remains `0.3`; no code, builds, deployment, or real-device tests were
  performed. Browser field validation remains pending operator results.

## docs - 2026-10-19 17:39 CEST (Europe/Zagreb)

- Recorded standing user authorization to commit and push completed authorized
  passes or coherent parts of a larger task unless explicitly held. If push
  fails, ask the user to push instead of forcing it. Separate approval rules
  for upstream merges, deployment, and production data remain unchanged.
  Service version remains `0.3`.

## 0.3 - 2026-10-19 17:25 CEST (Europe/Zagreb)

- Retained modernc as the preferred default and added optional
  `mattn/go-sqlite3` selection with the `sqlite_cgo` build tag. Both driver
  configurations passed Linux x86-64 tests. MIPS compilation/router operation
  remain untested; documented the experimental Zig command and requested
  hardware/OS/firmware feedback in a prepared issue reply.
- Integrated official Brave upstream master at
  `71f27af460f4524a1d4dac4c35d8e7ef2a072c50`, preserving fork history and leaving
  upstream files unchanged from that revision. Custom lite adaptations cover
  `GetUpdatesForType`'s `int` limit and explicit high-device-limit env loading.
  The update includes upstream auth fixes, command refactors, and AI Chat
  protobuf definitions; actual AI Chat browser sync is not claimed tested.
- Updated SQLite drivers to modernc `v1.60.1` and mattn `v1.14.52`, both embedding
  SQLite 3.53.4. Pinned modernc libc to the required `v1.77.1`, updated protobuf
  to `v1.36.12`, and reconciled inherited modules with upstream. Source builds
  now require Go 1.26+. Checked with the already installed Go 1.26.9 compiler.
- Kept the application schema, sync-chain identities, endpoint aliases, runtime
  config, and service layout. Added `--version`, escaped SQLite filename URIs,
  and driver-specific per-connection WAL/5000 ms busy-timeout options. Both
  backends explicitly use synchronous=FULL to preserve default durability;
  the `:memory:` development special case remains supported.
- Fixed a pre-existing insert-conflict contract bug: duplicate ID/tag inserts
  now return the error required by upstream to report `CONFLICT`, preventing
  false successful acknowledgments and count increments. Busy WAL checkpoint
  results now produce explicit errors.
- Added driver-neutral storage/protocol regression tests and a synthetic `0.2`
  fixture. Added helpers for formatting/ShellCheck/vet/tests, upgrade checks,
  real-process HTTP/HTTPS smoke tests, govulncheck, and static deployment builds
  with checksums and version/build metadata.
- Verified actual archived `0.2` -> `0.3` modernc -> `0.3` mattn -> `0.2`
  read/write/reopen compatibility, exact BLOB/metadata/schema preservation, and
  committed-WAL recovery on disposable databases. No schema migration or client
  reset is needed. Documented a private stopped-service backup and binary-only
  upgrade, retaining explicit operator control of any restore.
- Recorded dated maintenance results and remaining work in MAINTENANCE/TODO.
  Both govulncheck configurations found no vulnerabilities. SQLite 3.54.1 is
  newer than the drivers' bundled engine; its 32-bit fix needs review before
  promoting experimental MIPS builds. Inherited unused/deprecated modules and
  community distribution/access-control suggestions remain follow-up work.

Validation: default no-CGO tests, both race-enabled driver suites, vet,
ShellCheck, the cross-version upgrade matrix, actual HTTP/HTTPS aliases and
SIGINT/SIGTERM/restart/TLS validation, and both govulncheck runs passed. All data
and certificates were synthetic. No production service or real browser/router
was used; the existing `0.2` field evidence remains separately recorded.

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
