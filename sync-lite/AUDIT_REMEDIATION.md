# Audit Remediation

Work branch: `audit-remediation-0.4`. Candidate version: **0.4**, ready for human
review and controlled staging, not a claimed production/browser validation.
Production remains untouched. Each finding is
an independent validated commit; the version identifies the whole release series,
not each intermediate working-branch commit. All fixtures are synthetic.

## Tracking

Statuses become VERIFIED only after validation, documentation, commit and push.
Commit subjects include the finding ID; `git log --oneline --grep='\[A12\]'`
(substitute another ID) resolves the implementation without self-referential hashes.

| ID | Priority | Finding / root cause | Status | Implementation and verification | Upgrade / risks |
| --- | --- | --- | --- | --- | --- |
| A12 | P2 | Restart reseeding and chained WAL tests mask failures | VERIFIED | `287806f`, `ce17b7d`, pushed; check/smoke/supplemental/upgrades pass | All three actual archived baselines, both drivers independently |
| A03 | P1 | Incoming identity controls stored-type/version policy | VERIFIED | `8ad6cd2`, pushed; focused/full/race/upgrades pass | No schema change |
| A04 | P1 | Incomplete HTTP limits and absent deadlines | VERIFIED | `ec4e17b`, pushed; focused/full/race/smoke/upgrades pass | Schema/config unchanged |
| A08 | P2 | Optional wire fields violate upstream semantic preconditions | VERIFIED | `726e2e5`, pushed; focused/full/race pass | No storage/startup/config change |
| A01 | P1 | Precommit timestamps can fall behind issued tokens | VERIFIED | `4a0dc2e`, `612b02d`, pushed; focused/full/race/supplemental/upgrades pass | Additive table; old tied cohorts not replayed |
| A02 | P1 | Tag ownership and resurrection accounting | VERIFIED | `6ae54c3`, pushed; focused/full/race/supplemental/upgrades pass | Requires A06 coordination |
| A06 | P2 | Concurrent absolute quota snapshots lose deltas | VERIFIED | `58ef410`, `3f16969`, pushed; focused/full/race/upgrades pass | Single service process; approximate quotas preserved |
| A09 | P2 | Failed drain closes storage under active requests | VERIFIED | `992ea1f`, `579514b`, pushed; focused/full/race/smoke/upgrades pass | Force failure preserves committed WAL; clear is atomic |
| A05 | P2 | Expired History records remain forever | VERIFIED | `19e533c`, `ee28db8`, pushed; focused/full/race/upgrades pass | Normal tombstones preserved; no startup bulk deletion |
| A07 | P2 | Unvisited expired hints retain memory | VERIFIED | `56166df`, pushed; focused/full/race pass | Interim counters never evicted |
| A10 | P2 | Clear-data unnecessarily loads all payloads | VERIFIED | `0e8715a`, pushed; focused/full/race/benchmark/upgrades pass | Returned identity metadata and disabled marker preserved |
| A11 | P2 | File privacy inherits permissive defaults | VERIFIED | `54daa1b`, pushed; focused/full/race/smoke/upgrades pass | Existing modes deliberately preserved |
| A13 | P2 | Upstream new-device scan repeats the first page | VERIFIED | `b5a8ae5`, pushed; focused/full/race pass | Threshold contract tests; no parent edits |
| A14 | P3 | Authenticated non-POST requests can mutate state | VERIFIED | `5a10b6c`, pushed; focused/full/race/smoke pass | Unauthenticated probes retain 401 |
| A15 | P3 | In-memory database connection lifetime/isolation | VERIFIED | `18af87e`, pushed; focused/full/race/smoke/upgrades pass | Disk pooling unchanged |

## Validation And Scope

Use `./scripts/validate-remediation.sh` with its fixed documented modes; SDK
selection and execution details live inside the helper, never in approval prefixes.
Run upgrade matrices against `1c0d957` (0.2), `287bdec` (0.3), and
`07d147a` (0.3.1). Independently exercise each backend's first open of committed
old-version WAL, plus cross-driver reopen. No live databases, profiles, secrets,
or production service operations are permitted. Existing configuration, both
endpoint aliases, modernc default, optional mattn and FULL durability remain.

Baseline on 2026-11-06: shell Go 1.26.8; installed Go 1.26.10 selected explicitly
for subsequent checks. No dependency or upstream update is bundled into these
fixes. MIPS/ARM execution and actual Brave-device checks remain operator work.

## Evidence

Per-finding validation, compatibility decisions, commit/push outcomes and any
blockers are recorded below as the work progresses. PENDING does not mean fixed.

### A12 - 2026-11-06 14:49 CET

Confirmed by source inspection: reseeding before restart verification concealed
missing state; mattn did not independently encounter old WAL; duplicate outputs
could satisfy a length-only assertion. Added a negative missing-sentinel check
and identity-set exhaustion. `check`, `smoke`, `supplemental`, and `upgrades`
passed with Go 1.26.10. Upgrade matrix: all three baselines, independently through
modernc and mattn, clean/WAL, old-version read/write reopen, plus cross-driver.
No application schema/configuration change; only service version metadata changes
to 0.4 for the candidate series. Commit `287806f` was pushed. A helper edited
during its first run produced a parser failure after the matrix; the complete
stable helper was rerun successfully before committing.

### A03 - 2026-11-06 14:55 CET

All three reproductions failed before the fix: a claimed History type bypassed
version checks, another type replaced payloads, and a mismatched deletion tag
was accepted. Update now reads stored type/tag, rejects identity changes, applies
the History exception only to stored History, and uses stored tag for deletion.
Omitted tags remain valid. Both focused driver tests, complete check helper and
all three clean/WAL upgrade matrices passed. No schema, config or existing-row
rewrite. Commit `8ad6cd2` was pushed.

### A04 - 2026-11-06 14:57 CET

The oversized-body regression failed before implementation. Limits now read one
extra decoded byte and cap the wire reader independently, before dispatch. Tests
cover plain/gzip below/at/above the limit, an otherwise valid maximum-size prefix
with trailing fields, invalid gzip CRC, no writes on rejection, positive server
deadlines and actual slow header/body sockets. Focused tests passed on both
drivers; full checks/races and HTTP/HTTPS smoke passed. No stored-data or config
changes; prior-version fixture matrix rerun for startup compatibility.

### A08 - 2026-11-06 14:59 CET

Five pre-fix regressions reproduced handler panics: absent GetUpdates payload,
missing marker type, empty/unknown-only specifics, and History without its tag.
Malformed envelopes return 400; unsafe commit entries reach upstream's existing
INVALID_MESSAGE result without aborting valid siblings. Both focused backends
and full check/race suites passed, including opaque encrypted remote edits.
No storage, startup, dependencies or configuration changed; the A04 upgrade
matrix remains applicable. Future unknown-only specifics are explicitly rejected,
not acknowledged and lost. Commit `726e2e5` was pushed.

### A01 - 2026-11-06 15:03 CET

Red tests confirmed missed late commits and tied timestamps. Allocate from the
greater of the supplied timestamp and persisted clock/existing-row maximum + 1,
after acquiring SQLite's writer lock, before committing the entity. Fresh pointer
assignment preserves aliased creation times. Initial root version 1 stays unchanged.
The durable table anticipates bounded expiry cleanup without losing watermark
state. No automatic existing-data rewrite or token replay is performed.

Focused tests cover late preconstructed writes, frozen/backward time,
499/500/501 entries including tombstones, two concurrent store handles, failed
transaction rollback, clock schema and restart. Both drivers passed. Full check,
both race suites, supplemental checks and all three archived clean/WAL upgrade
matrices passed. Compatibility assertions exclude only the explicitly added
clock schema object; original table definitions, row values and BLOBs remain
exact. Historical filter tests intentionally seed raw legacy timestamps; new
writes no longer promise caller-chosen out-of-order modification times.

Known limit: old tied cohorts and records skipped by already-issued old tokens
are not repaired. A client-compatible replay/cursor design is separate follow-up;
no chain reset or automatic retimestamping is authorized. Commit `[A01]`.

### A06 - 2026-11-06 15:05 CET

Concurrent regression persisted 24 entities but reported a count of 1 before
the fix. A single cancellable gate now spans the authenticated command, disabled
check, shared interim-cache drains and final absolute counter save. Both URL
aliases and every handler using the store share that gate. This also orders
poll-cache publication against writes. Cancelled waiters do not acquire it.

This intentionally chooses the audit's proportional request-serialization option
instead of rewriting upstream accounting. Direct internal datastore count calls
must remain inside that coordinated command path; multiple service processes
sharing one DB are not supported. Existing version-2 counters/root offsets are
not recounted. Both focused suites, full/race checks and all archived upgrade
matrices passed. No schema/config change beyond A01. Commit `[A06]`.

### A02 - 2026-11-06 15:08 CET

Red protocol tests reproduced releasing another live identity's reservation and
failure to restore the normal count after resurrection. Updates now use stored
identity and effective deletion state; repeated deletion cannot release a live
owner's reservation, and resurrection checks ownership and reacquires its tag.
Omitted tags/deletion fields retain their meanings. A request-owned adapter adds
resurrection deltas when upstream saves counts, under A06's complete-command gate.
This preserves the public deleted-only datastore return contract and existing
counter offsets; quotas remain approximate, not strict per-batch admission.

Both focused driver tests, full/race checks, supplemental lifecycle tests and
all three archived upgrade matrices passed. The supplemental characterization
that expected the old tag-release defect now explicitly requires preservation.
No existing entities/tags/counters are rewritten on startup. Commit `[A02]`.

### A09 - 2026-11-06 15:11 CET

Red tests confirmed both Commit and ClearServerData ignored request cancellation
and mutated state. The adapter now binds every datastore operation to the request
context. Shutdown tracks handlers, stops admission, drains for 10 seconds,
force-closes connections on timeout, and waits another 5 seconds. It never
checkpoints/closes beneath a still-active handler. Failed drain/cleanup returns
failure; only complete graceful shutdown logs success. TLS pair/file validation
occurs before storage initialization; unexpected serve failure also drains.

Both driver focused tests exercise cancelled commit/clear, a forced-close drain,
and a deliberately stuck handler proving SQLite remains open until release.
Full/race suites, HTTP/HTTPS SIGINT/SIGTERM/restart smoke (including invalid TLS
opening no database), and all historical upgrade matrices passed. A cancelled
request can still have committed earlier entries; this is not atomic batch
rollback or permission to automatically retry unknown-outcome writes. `[A09]`.

### A05 - 2026-11-06 15:16 CET

Red tests confirmed an acknowledged History recommit remained expired/invisible
and expired rows were never retired. Read-only probes now identify actual expiry
work, followed by an atomic, at-most-256-row History-family retirement. Targeted
ID/tag cleanup supports recommits without primary-key/reservation collisions;
tag deletion checks remaining ownership. No normal tombstones or non-expiring
rows are removed. Legacy timestamp floors are persisted before retirement.

Both driver tests cover recommit, physical retirement, chain isolation, strict
expiry boundaries, bounded backlog processing, directive tag reuse and legacy
watermarks. Full check/race suites and all archived upgrade matrices passed.
The legacy read test deliberately changes only its expired-History expectation;
startup and all other values still compare exactly. Existing live expiry and
approximate history buckets are not rewritten. No IO benchmark or SSD-wear
claim is made. Binary rollback cannot undo legitimate expiry. `[A05]`.

### A07 - 2026-11-06 15:17 CET

Red tests retained an unvisited expired key and 1100 disposable hints. Set now
sweeps expired hints and declines new ones at 1024; misses safely query SQLite.
Non-expiring pending counters are not candidates for eviction. Both focused
driver tests and full/race suites passed, including concurrent increments and
atomic drain. The measured assertion is retained-key count, not a throughput
benchmark. No storage/config/startup changes; previous upgrade matrix remains
applicable. At idle, expired hints can remain until traffic, but stay bounded.
`[A07]`.

### A10 - 2026-11-06 15:19 CET

The red test confirmed ClearServerData materialized opaque payloads. It now
reads only chain/ID/type metadata inside the same writer transaction as deletion,
preserving per-row identity metadata, cache invalidation and disabled markers.
Synthetic modernc benchmark, 100 rows x 64 KiB, three iterations per sample:
before 13,264,741 B/op, 2,896 allocs/op, 9,391,947 ns/op; after 87,506 B/op,
869 allocs/op, 4,307,933 ns/op. This is local Go allocation/timing evidence,
not a server-wide latency, peak-RSS or disk-wear guarantee.

Both focused driver tests, full/race checks and all prior-version upgrade
matrices passed. No schema/config change. `[A10]`.

### A11 - 2026-11-06 15:22 CET

The fresh-file regression reproduced mode 0644. New database files are now
exclusively created as 0600 before SQLite opens them. Existing files are never
chmodded; a regression preserves deliberate 0640. Both drivers' tests verify
fresh DB/WAL/SHM permissions. README and unit example agree on `UMask=0077` and
first-install state directory 0700. Existing modes/group backup access require
operator review, not automatic recursive changes. No real server or unrelated
account access was tested. Full/race, HTTP/HTTPS smoke and all historical upgrade
matrices passed; no schema/data rewrite. `[A11]`.

### A13 - 2026-11-06 15:24 CET

All five red cases exhausted their bounded context while repeatedly reading 501
tombstones. A NEW_CLIENT-only adapter intercepts just the first setup scan and
returns at most 100 live eligible DeviceInfo metadata rows. That is sufficient
for unchanged 50/100 admission thresholds; subsequent reads in the same request
delegate to normal pagination. NULL legacy deleted state is conservatively live
in this admission projection. Chain/folder/mtime/expiry filters are preserved.

Both focused backends and full/race checks passed: 0/49/50 live devices, high-limit
99/100, and normal 500-entry download/more-changes in the same request. Threshold
tests must be revisited when incorporating upstream limit changes. No schema,
startup or config change, and no independent upstream source edits. The previous
upgrade matrix remains applicable. `[A13]`.

### A14 - 2026-11-06 15:26 CET

Red test confirmed an authenticated GET committed data and returned 200. A guard
now rejects unsupported methods before admission/body/database work, returning
405 and Allow POST. Authentication still runs first, preserving 401 reachability
checks. Both aliases, GET/PUT/PATCH/DELETE/HEAD, and no-mutation assertions pass
on both drivers. Full/race and real HTTP/HTTPS smoke checks passed. No storage,
startup or config change; prior upgrade checks remain applicable. `[A14]`.

### A15 - 2026-11-06 15:39 CET

Red tests opened a second empty database, producing missing sync_entities and
sync_clocks tables. Memory stores now retain exactly one open/idle connection;
waiters respect cancellation and separate stores stay private. Disk-backed pools
are unchanged. Both focused backends, full/race suites, smoke and all archived
upgrade matrices passed. The memory mode remains disposable, not production
persistence. `[A15]`.

## Independent Integration Review Follow-Up

The independent read-only integration review found three additional cases:
A06's gate included slow network IO; A09 cancellation could interrupt the two-step
disable/clear sequence; A05 tag ownership needed NULL-safe expiry. All three now
have red/green regressions and pushed corrections. A targeted A01 root-version
case was also reproduced and corrected. Follow-up static review found no remaining
blocker in these paths; its reviewer did not run the coordinator's tests. No
production deployment occurred.

### A06 Follow-Up - 2026-11-06 15:42 CET

Both slow-upload and slow-response regressions failed before correction. Bounded
body reads and semantic validation now occur before the command gate, and
response encoding/network writes occur after release. Disabled state is checked
inside the gate immediately before dispatch. Accounting/publication remains
serialized. Focused tests, full/race suites and real-process smoke passed.
No storage/config change; previous upgrade evidence remains applicable.

### A09 Follow-Up - 2026-11-06 15:44 CET

Red tests confirmed cancellation between the parent's two calls stranded a
disabled marker, and injected deletion failure was not observed during Disable.
The request adapter now performs marker creation, metadata capture, tag/count
removal and entity deletion in one transaction during Disable, then returns the
committed metadata from Clear without another mutation. Public standalone store
methods retain their contracts. Cancellation before commit rolls back; after
commit, metadata remains available even if the context is cancelled. Synthetic
trigger failure proves all original rows/tags/counts and enabled state survive
rollback. Both focused suites, full/race and upgrade matrices passed.

### A05 Follow-Up - 2026-11-06 15:46 CET

The red legacy-state test retired an expired same-tag tombstone and incorrectly
released its live directive owner's reservation when expiry was NULL. COALESCE
now makes the ownership predicate NULL-safe and preserves non-expiring semantics.
Both focused backends, full/race, supplemental and archived upgrade matrices
passed; HTTP/HTTPS smoke also passed on the integrated state. No schema change.

### A01 Follow-Up - 2026-11-06 15:48 CET

A targeted red regression found that using server-tag presence as the initial
root-version exception could leave an ordinary root update's version behind its
clamped timestamp. The allocation call now explicitly preserves versions only
for the initial server-tag batch. Normal updates, including Nigori roots, advance
matching timestamp/version values and reject stale versions. Both focused
backends, full/race suites and all archived upgrade matrices passed. Initial root
version 1 and original creation times remain covered. No schema change.

### A12 Follow-Up - 2026-11-06 15:54 CET

Upgrade stages now use each compiled version's real InsertSyncEntity and
GetUpdatesForType APIs, rather than raw SQL writes. Exact pre-write snapshot
comparison remains unchanged. All three archived baselines passed independent
clean/WAL modernc/mattn upgrade and old-version reopen/write/readback, plus
cross-driver sequences. A12's helper mode now invokes smoke/upgrade/supplemental
checks explicitly instead of allowing an empty focused-test selection. The check
helper also verifies downloaded module checksums. Candidate docs, TODO and
finished-task status are aligned with this validated series.

## Implementation Map

| Findings | Production files | Regression evidence |
| --- | --- | --- |
| A01 | `sqlite_clock.go`, `sqlite_store.go` | `audit_clock_test.go`, upgrade matrices |
| A02, A03 | `sqlite_store.go`, `command_store.go` | `audit_lifecycle_test.go`, `audit_identity_test.go` |
| A04, A08, A14 | `main.go`, `protocol_validation.go` | `audit_http_test.go`, `audit_validation_test.go`, `audit_methods_test.go` |
| A05 | `sqlite_expiration.go`, `sqlite_store.go` | `audit_expiration_test.go`, legacy preservation tests |
| A06 | `command_dispatch.go`, `command_gate.go` | `audit_accounting_test.go` |
| A07 | `memory_cache.go` | `audit_cache_test.go` |
| A09 | `server_lifecycle.go`, `command_store.go`, `sqlite_store.go` | `audit_shutdown_test.go`, real-process smoke |
| A10 | `sqlite_store.go` | `audit_clear_test.go`, fixed benchmark mode |
| A11 | `sqlite_store.go`, unit/README examples | `audit_permissions_test.go` |
| A12 | test/upgrade/smoke/check helpers and synthetic fixtures | independent WAL matrices, negative sentinel check, identity-set assertions |
| A13 | `device_admission.go`, `command_store.go` | `audit_admission_test.go` |
| A15 | `sqlite_store.go` | `audit_memory_test.go` |

The HTTP layer authenticates and bounds/validates the body before serializing
only command state. A request-owned adapter preserves upstream API contracts
while handling cancellation, resurrection deltas, atomic clear and device
admission. SQLite remains authoritative; hints are disposable and quota counters
remain approximate. No new service, framework or runtime dependency was added.

## Final Verification

All fifteen finding implementations and reviewed integration corrections are
committed/pushed on the working branch. Final complete check (including module
integrity), A12 smoke/upgrade/supplemental mode, and both govulncheck configurations
passed on the integrated source. The default Linux x86-64 artifact builds as
version 0.4 with Go 1.26.10 and CGO disabled; its generated checksum verifies.
Artifacts stay ignored in `dist/`, not in Git. Git diff checks found all custom
changes within sync-lite and no module/dependency changes.

Ready for human review and controlled staging with backup-first operator checks,
not an automatic merge/deployment or production-ready certification. Real Brave
devices, MIPS/ARM and production recovery were not tested. Old tied cohorts and
already-skipped tokens are not automatically replayed; quotas remain approximate,
one process owns the DB, and normal explicit History expiry is not undone by
binary rollback. Remaining work is in TODO. Only helper-owned synthetic data and
TLS fixtures were used; no user databases, profiles or credentials were accessed.

## Local Remediation Follow-Ups

2026-11-06 19:42 CET (Europe/Zagreb). Scope: remaining local tests, legacy-token
assessment, validation and documentation only. No production-source, schema,
dependency, configuration or version change; candidate remains 0.4. Optional
backlog features, upstream updates, deployments and real-device access are not
part of this pass.

| Evidence | Additional coverage |
| --- | --- |
| `remediation_protocol_test.go` | Nested parent-before-child batch mapping with an intervening sibling; later server-parent reference; exact entity sets/relationships after HTTP download and reopen |
| `remediation_protocol_test.go` | AI Chat conversation/entry plain and opaque payloads; remote update, scalar polling, stale conflicts, tombstones and reopen; nested compressed/binary/truncation fields and unknown specifics preserved |
| `remediation_quota_test.go` | Root initialization/count offsets/reopen; combined batch quota and capped legacy History boost; updates/deletion at capacity; History/directive acknowledgement without persistence at 30,000; other-type progress below combined cap |
| `remediation_quota_test.go` | History bucket rotation around one/two/four/40 periods, persistence on command completion, future anchors, positive/zero old count versions and preserved version-2/future-version offsets |
| `remediation_legacy_token_test.go` | Actual HTTP traversal of 501 legacy tied rows; no silent record/clock rewrite; unchanged reopen; already-skipped old-token case versus safely ordered new writes |
| [LEGACY_TOKENS.md](LEGACY_TOKENS.md) | Completed assessment: keep current scalar protocol/data; a future keyset cursor and explicitly scoped replay require browser/rollback evidence and separate authorization |

Quota-limit fixtures seed synthetic counters rather than loading 30,000/50,000
rows. Rotation tests use a 60-second margin on either side of a boundary to
avoid second-level scheduling races; they do not claim a frozen-clock exact
equality test or performance coverage. Positive version-1 recount is inherited,
not a new automatic recount of current version-2 counters. AI Chat tests verify
server transport/storage, not browser UI, encryption correctness or AI execution.
Unknown outer SyncEntity fields and forward parent references are not covered.
Historical omissions remain characterized limitations, not recovered data.

The fixed helper's `followups` mode runs `TestRemediation...` with default
no-CGO modernc and race-enabled mattn. Full `check` also discovers the new tests.
Both focused backends and the full check passed with installed Go 1.26.10,
including module verification, formatting, ShellCheck, both vet configurations,
default tests, both race suites and the invalid-CGO guard. `A12` passed real
HTTP/HTTPS lifecycle checks, all actual 0.2/0.3/0.3.1 independent clean/WAL
upgrade/read-write/old-version reopen matrices and supplemental synthetic tests.
Both govulncheck configurations returned no vulnerabilities found. The default
static Linux x86-64 artifact rebuilt and reports 0.4/modernc/Go 1.26.10.

Initial legacy fixture attempts used storage-only placeholder UniquePosition
bytes and failed protobuf decoding. Corrected the synthetic fixture to a valid
protobuf; no runtime correction was needed or made. Only helper-owned temporary
fixtures and loopback processes were used. Production/browser/MIPS validation
and all unrelated TODOs remain open; the authorized local follow-ups are closed.

Independent static review strengthened the new tests before completion: verify
raw durable counter rows before any normalizing read, compare exact delivered
legacy identity/metadata/payload sets without assuming tie order, and compare
records/clocks again after reopen and the restarted scan. Full/focused checks
were rerun after those changes. No additional production-code fix was needed.
