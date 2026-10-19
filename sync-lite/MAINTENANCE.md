# Maintenance Record

## 2026-10-19: Reconciliation Count-Baseline Follow-Up

Read-only operator aggregates show two accounting entries: one has only the five
server roots and no Sessions, while the populated entry has both live and deleted
Sessions and a normal-counter shortfall larger than those five roots. This does
not establish unauthorized use, corruption, or lost records. Setup inserts five
server-defined roots without going through the normal commit-count increment;
therefore the earlier equality check was deliberately conservative, not a
universal consistency invariant. The extra drift's origin remains unknown and
does not itself explain an update rejected for a missing ID.

Investigation helper v0.2 extends only the synthetic experiment. Unreviewed count
differences still abort. An explicitly supplied stored/live baseline permits
staging only when both values match exactly inside the same transaction; matching
the difference alone is insufficient. It adds one stored count per staged live
identity, preserving the existing difference rather than recounting unrelated
data. History buckets and metadata, old tombstones, roots, and unrelated rows
remain unchanged. The service and its version stay `0.3.1`.

Both backend experiments passed with root-induced drift and synthetic under- and
over-counts, including zero stored count. Changed baselines abort with no writes;
normal authenticated retries succeed afterward while preserving the count gap.
The standard check helper also passed. No production repair was performed.
Before any real repair, take a fresh consistent backup and separately recheck
target/donor metadata, current exact base versions, and the same-chain stored/live
baseline. Do not delete old Sessions merely because their tombstone count is high.

## 2026-10-19: Synthetic Sessions Reconciliation Investigation

The operator later confirmed fresh open tabs and history arrived in both
directions, after delays of several minutes. Two positive-version phone Session
updates still conflict. Their requested server IDs are absent from the pre-0.3
backup; both backup and current database contain multiple deleted Sessions rows
matching each client tag, plus a reservation per tag. The operator confirmed this
is a single-chain installation. The failed requests have `deleted:false`: they
are updates, not pending deletion acknowledgments. Matching tags identify logical
items, not equal server IDs, URLs, or payload bytes. The old rows being deleted
does not prove the absent requested IDs were deleted. Root cause remains unknown.

The operator accepts loss, resurrection, or duplication of those two obsolete
tab items only. This is not permission to reset History, the chain, or unrelated
records. Private IDs, tag hashes, browsing content, and raw diagnostics are not
stored in this repository. No agent accessed the production database or phone.

Investigation helper v0.1, `scripts/test-session-reconciliation.sh`, runs an
overlay of `testdata/session_reconciliation_test.go` against disposable databases
with the unchanged `0.3.1` handler. It accepts no database path and cannot repair
an installation. The experiment uses invented identities and opaque payloads:

- Reproduce missing-ID conflicts with same-tag tombstones and existing tag
  reservations. Do not undelete or rename those original tombstones.
- In one transaction, stage the missing IDs at the exact positive base versions
  from the simulated client. Require only deleted non-folder Sessions donors,
  valid typed protobuf specifics, no unique position/server tag/expiry, no existing target,
  a same-chain reservation, and a consistent normal-item count baseline.
- Staged rows use `deleted=0` so omitted deletion flags and later genuine
  deletion follow the existing update/count contract. Add one normal-item count
  per staged row atomically, without changing History buckets or old rows.
- Use the minimum signed 64-bit mtime, not zero: strict `mtime > token` excludes
  the staged contents for every signed 64-bit progress token, including negative
  ones. This relies on the current GetUpdates implementation, not a new general
  access-control mechanism. A real retry replaces specifics and timestamp.
- Keep unknown original originator metadata absent, rather than copying another
  record's identity. Borrowed tombstone bytes are temporary working state, not
  claimed historical recovery. The simulated retry supplies its actual opaque
  payload through the normal authenticated update path.

Both default modernc and race-enabled mattn experiments passed: conflict before,
successful retry after, exact ID retention, fresh version, incremental peer
download of the client payload, and later normal deletion/count/tag release.
Explicit false, omitted, and true retry deletion flags were tested. Old
tombstones, unrelated entity rows, and History counters were preserved. Stale
versions still conflict. Refusals for ambiguous/wrong-type/live/missing donors,
missing reservations, existing IDs, disabled/wrong chains, bad specifics, expiry,
folder state, count mismatch, and an invalid second target leave all tables
unchanged. Reopen and integrity checks passed. Go remains 1.26.9; no dependencies,
schema, application source, or server artifacts changed. Service version stays
`0.3.1`; this is not a recovery release or a production repair utility.

One inherited limitation was also reproduced: a correctly versioned deletion of
an older same-tag tombstone releases that tag reservation even while the staged
identity is active. Reservations are not entity-ownership mappings. This is not
fixed by staging and needs explicit consideration before production repair; do
not advertise reconciliation as a general cure for all same-tag state. A normal
same-tag competing insert is rejected while the reservation remains present.

Before preparing any real repair, check the current count baseline and donor
metadata using bounded read-only queries, then require a fresh stopped-service
backup and recheck exact identity/tag/base-version preconditions. A count
discrepancy must be investigated separately, not silently recounted. Real Brave
retry, conflict clearance, and continued two-way sync remain operator checks.
The fixture does not run Chromium's client conflict resolver or establish the
cause of the existing state. Do not copy its mutation routine into a live query.

Do not toggle the phone's combined History and tabs setting for a Sessions
reset. Chromium's History-disable path removes imported visits, and sync-lite
filters History older than its 14-day server expiration. A complete re-download
is not guaranteed; a server backup does not protect phone-only history. See the
[manual-testing safety warning](MANUAL_TESTING.md#diagnosing-persistent-commit-conflicts).

## 2026-10-19: Bookmark And Group Transfers Confirmed

The operator subsequently confirmed the phone's bookmark rename reached desktop
and fresh test tab groups appeared in both directions on `0.3.1`. Together with
the earlier desktop-to-phone bookmark delivery, this confirms those transfers
work despite the recurring `update/missing_entity data_type=50119` diagnostic.
Arrival delays were not measured; initial absence was not a confirmed permanent
failure. Do not attribute successful transfer to a repair in `0.3.1`, which only
added diagnostics, or claim the recurring conflict is harmless/resolved.

Reported phone client: Brave Nightly 1.100.5, Chromium 156.0.8078.39, official
64-bit APK on Android 16; APK versionCode 430000504, target SDK 37. Exact desktop
browser version was not supplied. No personal device identifiers or Android
build fingerprint are needed for this record.

The conflict may have predated the update, but no prior diagnostic baseline
establishes its start. It is not yet attributed to Nightly or the server update.
Next narrow user check: independently verify newly opened tabs in each client's
other-device tabs view, keeping the source tabs open. Retain the conflict as an
open Sessions investigation without speculative database/client repair.

## 2026-10-19: Operator Diagnostic Result

The operator confirmed `0.3.1` with modernc is running over HTTPS and both
clients reach the service. Opt-in diagnostics recorded
`operation=update reason=missing_entity data_type=50119`. The checked protocol
schema maps `50119` to Sessions (open-tab/window records), distinct from
Bookmarks (`32904`) and Saved Tab Groups (`1004874`). This confirms at least one
attempted session update references an ID absent from that chain; it does not
establish why it is missing, prove database corruption, or identify all affected
types. The log deliberately does not identify the sending device.

The earlier legacy false-acknowledgment hypothesis remains unconfirmed. Do not
reinsert missing records, accept arbitrary IDs/stale versions, or clear browser
metadata based on this log alone. Next evidence: fresh retries, other conflict
types/reasons, current bookmark/group outcomes, and client versions. Retain the
existing database, chain, backups, and privacy-safe logging. Private hostnames,
certificate filenames, process IDs, and the raw journal are not recorded here.

## 2026-10-19: sync-lite 0.3.1 Diagnostic Follow-Up

The operator reported that a new desktop bookmark reached Android, but the
phone's rename and fresh tab groups did not propagate during the observation
window. Android showed a successful Download step, a last Commit result of
CONFLICT, and backoff. No insert/update error appeared in the journal. This
does not establish a phone bug or a particular server defect.

The existing store silently returns update conflicts for missing records and
version mismatches. `0.3.1` adds opt-in, bounded `SYNC_LOG_CONFLICTS=1` logs for
those cases and duplicate inserts, without IDs, versions, payloads, or changed
write acceptance. No schema/data migration or automatic recovery is introduced.
One code-review hypothesis is legacy false acknowledgments after duplicate
inserts in `0.2`; no evidence yet connects that mechanism to this installation.
Do not treat it as a diagnosis or silently adopt client-supplied IDs.

Go remains 1.26.9 for this pass. Dependencies, compiler series, embedded SQLite,
and official upstream revision are unchanged from the baseline below. New
protocol tests cover a remote bookmark edit (plain and opaque encrypted),
incremental polling, stale-version rejection and fresh-version retry, and saved
groups/tabs from both devices. These synthetic cases pass; target reproduction
and the actual conflicting type/reason remain pending.

Validation passed on both Linux x86-64 backends: the full check helper including
race tests, diagnostic privacy/disabled-mode/rejection-preservation/rate-limit/
capacity/concurrency tests, HTTP/HTTPS lifecycle smoke checks, and govulncheck.
Both actual archived `0.2` (`1c0d957`) and `0.3` (`287bdec`) passed the synthetic
upgrade/read-write/rollback matrix through both `0.3.1` backends, with clean
databases and committed-WAL recovery. No real data, browser profiles, production
service, or devices were accessed. An operator retry with diagnostics is still
needed; these automated passes do not resolve the field incident.

## 2026-10-19: sync-lite 0.3

Supported deployment target: Debian/Ubuntu Linux x86-64. The preferred build uses
modernc with CGO disabled; the optional mattn backend was also tested on Linux
x86-64. MIPS and ARM builds/runtime were not tested.

### Upstream

Reviewed and integrated official `brave/go-sync` master at
[`71f27af460f4524a1d4dac4c35d8e7ef2a072c50`](https://github.com/brave/go-sync/commit/71f27af460f4524a1d4dac4c35d8e7ef2a072c50).
The previous fork baseline was `1c0d957` (documentation from September; its sync
code was still based on the February checkout). Official upstream files were
incorporated unchanged. The merge preserves fork history; our custom adaptations
remain inside `sync-lite/`.

Relevant upstream changes include case-insensitive blocked-key checks, command
refactors, AI Chat protobuf definitions, DynamoDB clear-data fixes, and module
updates. Sync-lite adapts the datastore's new `maxSize int` signature and calls
the high-device-limit environment loader explicitly. Generic SQLite BLOB/type
storage requires no schema change for the added protobuf definitions. Actual
AI Chat browser synchronization has not been validated.

### Toolchain And Dependencies

| Component | Checked/selected version |
| --- | --- |
| Build and test compiler | Go 1.26.9, Linux amd64 |
| Module minimum | Go 1.26.0 |
| Default modernc SQLite driver | `modernc.org/sqlite v1.60.1` |
| Required modernc libc pin | `modernc.org/libc v1.77.1` |
| Optional mattn SQLite driver | `github.com/mattn/go-sqlite3 v1.14.52` |
| Embedded SQLite, both selected drivers | 3.53.4, verified through `SELECT sqlite_version()` |
| Previous 0.2 embedded SQLite | 3.49.2, verified with archived source |
| Protobuf | `google.golang.org/protobuf v1.36.12` |
| GCC | 16.2.0 |
| ShellCheck | 0.11.0 |
| govulncheck | 1.8.0 |

The shell's default Go was 1.26.8; checks explicitly selected the already
installed 1.26.9 patch. [Official Go downloads](https://go.dev/dl/) also list
Go 1.27.2; this pass kept the supported 1.26 series rather than changing compiler
series unnecessarily. The host's `sqlite3` package is not the embedded engine in
either default driver build.

Modernc's release requires Go 1.26 and its exact libc pin; do not independently
upgrade libc just because a newer version is available. Release requirements
were checked against the downloaded module's `go.mod` and changelog. The new
modernc and protobuf tags, and the mattn tag, were checked through the Go registry.

The complete module graph was checked with `go list -m -u all`. Many inherited
AWS/Redis/Prometheus and parent-server modules have newer releases; they remain
at the official parent graph's selections unless this pass's direct driver
dependencies require a higher version. This avoids an unrelated dependency
refactor. Deprecated `github.com/go-chi/chi` v4 and retracted
`github.com/gomodule/redigo` appear in the inherited graph, but `go mod why -m`
reports that the sync-lite module does not need them. The same check showed
`golang.org/x/net` is not needed by this module's package graph.

Both source configurations returned **No vulnerabilities found** from
govulncheck, using Go 1.26.9. The queried vulnerability database reported an
update time of 2026-10-08 22:31:09 UTC. This is a dated scanner result for these
builds, not a full security audit or proof about unreported/native SQLite bugs.

SQLite itself has released [3.54.1](https://sqlite.org/changes.html), newer than
the 3.53.4 bundled by both latest selected driver tags. Its notes include a
32-bit heap-buffer fix; applicability to the experimental MIPS build needs
review before it is promoted for production. The release notes say 64-bit builds
are unaffected by that particular problem. Track driver updates and upstream
[SQLite vulnerability information](https://sqlite.org/cves.html); do not patch
generated SQLite internals ad hoc or infer coverage from govulncheck alone.

### Validation

- The unchanged `0.2` integration suite passed before implementation.
- `scripts/check.sh`: formatting, ShellCheck, vet for both drivers, default
  no-CGO tests, race-enabled suites for both drivers, and rejection of
  `sqlite_cgo` with CGO disabled passed. `go mod verify` verified all modules.
- New storage regressions cover per-connection WAL/timeout/synchronous=FULL, first-run schema,
  records/counts/disabled chains across reopen, conflicts and rollback, stale
  updates, deletion, pagination/filtering, chain-scoped clear-data, legacy schema,
  checkpoint busy results, and literal filenames containing URI punctuation.
- New protocol tests cover duplicate-tag rejection/count preservation, two
  same-chain devices with distinct identities, persistence after reopen,
  new-client/Nigori roots, and case-insensitive blocked-key checks.
- `scripts/test-upgrade.sh`: archived `0.2` -> `0.3` modernc -> `0.3` mattn ->
  `0.2` read/write/reopen checks passed for clean and committed-WAL fixtures.
  Schema, metadata, BLOB bytes, row counts, integrity, and writes were checked.
  No schema or data migration is required for the unchanged application schema.
- The upgrade test initially confused an empty BLOB represented as a nil byte
  slice with SQL NULL in JSON. The fixture comparison now tags BLOB values and
  compares their exact hex bytes, preserving the NULL/empty distinction. The
  corrected cross-driver matrix passed; no stored-data repair was needed.
- Mattn's default synchronous=NORMAL differs from the existing modernc build.
  Both adapters explicitly select FULL to preserve durability; the upgrade
  helper also verifies FULL in the actual archived `0.2` build. The documented
  `:memory:` store special case remains supported for isolated development use.
- `scripts/smoke-test.sh`: real-process version output, HTTP/HTTPS and both URL
  aliases, SIGINT/SIGTERM, persisted synthetic data, restart, and incomplete TLS
  configuration checks passed for both backends. Certificates were disposable.
- `scripts/security-check.sh`: both govulncheck runs passed.

All runtime checks used helper-owned synthetic state. No real browser profiles,
sync chains, production databases, secrets, or installed services were accessed.
The September `0.2` field results remain in README; `0.3` home-server/browser and
MIPS router validation still require operator feedback.

Next routine review: November 2026, or earlier after a relevant advisory. Record
the checked revisions, exceptions, and outcomes here; do not perform unattended
upgrades or automatic restores.

### Community Feedback Reviewed

Read the public comments in [Brave issue #393](https://github.com/brave/go-sync/issues/393)
and [fork discussion #1](https://github.com/luxzg/go-sync/discussions/1).
The normal web tool could not load the discussion; its public HTML was fetched
directly and the visible discussion bodies were read. The exact new MIPS comment
was supplied by the user; the issue web view did not independently resolve its
anchor.

Feedback includes successful user tests, MIPS driver limitations, possible extra
writes, public-chain admission control/mTLS, downloadable binaries, optional
container packaging, repository discoverability/upstream integration, other
Chromium browser integration, and browser profile export/import. Driver choice
and build instructions are addressed in `0.3`; unresolved items are scoped in
TODO. No public replies were posted and no browser/client changes were made.
