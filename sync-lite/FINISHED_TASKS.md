# Completed And Deferred Work

This archive records scope and decisions from the project conversation. It does
not claim fresh code verification or replace the changelog timeline.

## Completed / Reported

- On 2026-10-19, investigation helper v0.1 reproduced the structural stale
  Sessions state using synthetic IDs, tombstones, reservations, and opaque
  payloads. Both SQLite backends passed exact-version staging/retry, download
  isolation, count/deletion/stale-version checks, and transactional refusals.
  This is an isolated server-side experiment, not a production repair or proof
  of browser recovery. The service stays `0.3.1`; remaining preflight/recovery
  work is in TODO and the contract/limits are in MAINTENANCE.
- The operator also confirmed fresh open tabs and history syncing both ways on
  `0.3.1`, with delays of several minutes. Two stale Session updates continue to
  conflict despite those transfers. Matching tags have only tombstones in both
  old backup and current data; the requested IDs were absent from the backup.
  This is user-run read-only evidence, not proof of their origin or deletion.
- On 2026-10-19, the operator confirmed the desktop bookmark reached Android,
  its phone-side rename returned to desktop, and fresh test groups arrived in
  both directions on `0.3.1`. Phone: Brave Nightly 1.100.5 / Chromium
  156.0.8078.39, 64-bit APK on Android 16. Exact delays and desktop version were
  not recorded. The recurring missing Sessions record (`50119`) conflict is
  still open; these transfers do not establish a complete Sessions pass or
  when the conflict began. Earlier apparently stalled test items did arrive.
- On 2026-10-19 the operator confirmed `0.3.1`/modernc running with HTTPS and
  both clients reaching it. Diagnostics identified an update of a missing
  Sessions record (`50119`), narrowing the reported conflict without resolving
  its origin or proving the bookmark/group symptoms share that cause. This is
  operator-reported evidence, not agent access or a complete sync validation.
- Version `0.3.1` on 2026-10-19 added optional, bounded, privacy-safe conflict
  reason/type logging and remote-edit/group protocol tests. Both driver suites,
  race/privacy/rate-limit checks, HTTP/HTTPS smoke checks, govulncheck, and actual
  `0.2`/`0.3` synthetic upgrade/rollback matrices passed. Schema and write acceptance
  are unchanged. The user's Android commit conflict is not yet reproduced or
  fixed; pending operator diagnostics and field validation remain in TODO.
- On 2026-10-19, added a two-device manual validation guide covering uniquely
  marked items, both directions, remote edits, sync-internals diagnostics,
  category/platform differences, privacy, and non-destructive troubleshooting.
  Linked it from installation/upgrade/testing instructions. This is documentation;
  `0.3` home-server/browser results remain pending operator confirmation.
- Version `0.3` work on 2026-10-19 added optional mattn selection through the
  `sqlite_cgo` build tag while retaining modernc as the preferred default.
  Both configurations passed automated Linux x86-64 tests. MIPS compilation and
  router execution remain unverified; the issue reply requests community testing.
- Incorporated reviewed official upstream revision
  `71f27af460f4524a1d4dac4c35d8e7ef2a072c50` unchanged outside `sync-lite/`.
  Adapted the datastore signature and explicit high-device-limit environment
  loading inside lite. Updated direct SQLite/protobuf dependencies with the
  required matching modernc libc pin; Go 1.26+ is now required for source builds.
- Corrected the SQLite insert-conflict return contract so upstream emits
  `CONFLICT` rather than acknowledging an unpersisted duplicate or increasing
  its item count. Added focused storage and protocol regressions. Busy WAL
  checkpoint results are now reported rather than ignored.
- Added and ran check, upgrade, smoke, security, and build helpers. Tests cover
  both drivers, same-chain devices, fresh schema, roots, conflicts/deletion,
  persistence, legacy schema, WAL busy results, and escaped filenames.
  Real-process checks cover version output and HTTP/HTTPS startup/shutdown.
- The synthetic upgrade matrix passed from actual archived `0.2` code through
  both `0.3` drivers and back to `0.2`, including committed WAL recovery. The
  application schema is unchanged; no data migration or client reset is needed.
  README documents a stopped-service private backup and binary-only update.
- The October maintenance pass used Go 1.26.9 and found no vulnerabilities with
  govulncheck in either driver configuration. Remaining embedded-engine and
  inherited-dependency follow-up is in MAINTENANCE and TODO. No home-server
  deployment or real browser/MIPS testing was performed in this pass.
- Initial Go/SQLite implementation and first-run use were reported working in
  February 2026. Version `0.2` was designated the tested baseline on 2026-02-13.
- The user reported six integration tests passing with
  `GOWORK=off go test ./...`: unauthorized access, commit/GetUpdates, disabled
  chains, gzip, chain isolation, and same client-side ID across chains.
  This is user-run local automated evidence; the suite was not rerun on 2026-09-13.
- Debian/Ubuntu deployment with systemd, a prebuilt binary, port 8295, readable
  certificate copies, and Brave base URLs was tested by the user. Exact desktop
  and phone validation scope is retained in README.
- Expanded two-way sync testing included passwords (1000+ imported), bookmarks
  and folders, autofill, sessions, tabs, tab groups, history, and partial
  preferences. This was user-reported remote/device validation, not an exhaustive
  compatibility or cryptographic audit.
- On 2026-05-26 the user reported desktop/phone syncing still working well after
  a couple of months.
- On 2026-09-13 the user confirmed continued password, bookmark, tab, and tab-group
  sync between updated Debian Brave Flatpak and Android Brave Nightly APK clients.
  Extended downtime, service restarts, and server reboots/restarts were exercised:
  syncing resumes when the service and network connectivity return. This is
  user-reported field evidence, not a fresh agent-run regression suite.
- Documentation alignment with rulebook revision `2026-09-12.2` was completed
  on 2026-09-13, including local agent guidance, evidence limits, open work, and
  this archive. No implementation or runtime validation was part of that pass.
- On 2026-09-13 the user settled versioning: current `0.2`, next `0.3`, optional
  patch versions such as `0.3.1`, and integer minor increments (`0.9` to `0.10`).
  Documentation-only changes retain the binary version. Changelog entries are
  newest first with datetime/timezone; missing historical times stay unknown.
- On 2026-09-13 the user clarified that merging official `brave/go-sync` updates
  into `luxzg/go-sync` is allowed as separately approved maintenance, even when
  upstream files change. Our custom modifications remain inside `sync-lite/`.
  Conflicts needing custom edits outside the folder require discussion.
- Source setup now clones the public `luxzg/go-sync` fork, enters `sync-lite/`,
  and links directly from README deployment to the clone/build instructions.
- Brave issue [#48909](https://github.com/brave/brave-browser/issues/48909) is
  closed; the [client fix](https://github.com/brave/brave-core/pull/37237) merged
  on 2026-06-18. Both server endpoint aliases and the tested base-URL setup remain.

## Deferred / Out Of Scope By Decision

- A dependency-minimization refactor was deferred by the user: keep upstream
  auth, command, cache, datastore, and protobuf package reuse for now.
- Docker, external DynamoDB/Redis services, metrics, and health-check endpoints
  are unnecessary for the intended small self-hosted deployment.
- Importing an existing DynamoDB store was unnecessary for the initial clean
  clients. This is not permission to discard today's SQLite data.
- Automated ACME management in sync-lite is not required; the user chose an
  external certificate-copy-and-restart workaround.
- Linux x86-64 was confirmed as the supported server target on 2026-09-13. ARM and
  MIPS hardware support were deferred because testing hardware was unavailable. A reporter's
  `GOARCH=mipsle GOMIPS=softfloat go build` failed with `modernc.org/sqlite`;
  they reported success after switching to `github.com/mattn/go-sqlite3` and using
  `zig cc -target mipsel-linux-musleabi -static`. Neither result was reproduced
  here. Pure-Go implementation does not establish support for every Go target.
  The October `0.3` pass later added mattn as an optional build without replacing
  the working default. Native hardware validation remains outstanding in TODO.
  The separate write-amplification concern also remains active because it can
  be investigated on the supported target.
