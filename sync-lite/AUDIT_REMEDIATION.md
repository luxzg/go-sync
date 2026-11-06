# Audit Remediation

Work branch: `audit-remediation-0.4`. Target version: **0.4**, unreleased until
the reviewed series is complete. Production remains untouched. Each finding is
an independent validated commit; the version identifies the whole release series,
not each intermediate working-branch commit. All fixtures are synthetic.

## Tracking

Statuses become VERIFIED only after validation, documentation, commit and push.
Commit subjects include the finding ID; `git log --oneline --grep='\[A12\]'`
(substitute another ID) resolves the implementation without self-referential hashes.

| ID | Priority | Finding / root cause | Status | Implementation and verification | Upgrade / risks |
| --- | --- | --- | --- | --- | --- |
| A12 | P2 | Restart reseeding and chained WAL tests mask failures | VERIFIED | `287806f`, pushed; check/smoke/supplemental/upgrades pass | All three actual archived baselines, both drivers independently |
| A03 | P1 | Incoming identity controls stored-type/version policy | VERIFIED | `8ad6cd2`, pushed; focused/full/race/upgrades pass | No schema change |
| A04 | P1 | Incomplete HTTP limits and absent deadlines | VERIFIED | `ec4e17b`, pushed; focused/full/race/smoke/upgrades pass | Schema/config unchanged |
| A08 | P2 | Optional wire fields violate upstream semantic preconditions | VERIFIED | `726e2e5`, pushed; focused/full/race pass | No storage/startup/config change |
| A01 | P1 | Precommit timestamps can fall behind issued tokens | VERIFIED | `4a0dc2e`, pushed; focused/full/race/supplemental/upgrades pass | Additive table; old tied cohorts not replayed |
| A02 | P1 | Tag ownership and resurrection accounting | VERIFIED | `6ae54c3`, pushed; focused/full/race/supplemental/upgrades pass | Requires A06 coordination |
| A06 | P2 | Concurrent absolute quota snapshots lose deltas | VERIFIED | `58ef410`, pushed; focused/full/race/upgrades pass | Single service process; approximate quotas preserved |
| A09 | P2 | Failed drain closes storage under active requests | VERIFIED | `992ea1f`, pushed; focused/full/race/smoke/upgrades pass | Force failure preserves committed WAL; no atomic-batch promise |
| A05 | P2 | Expired History records remain forever | VERIFIED | `19e533c`, pushed; focused/full/race/upgrades pass | Normal tombstones preserved; no startup bulk deletion |
| A07 | P2 | Unvisited expired hints retain memory | IN PROGRESS | Sweep on Set and cap disposable hints at 1024 | Interim counters never evicted |
| A10 | P2 | Clear-data unnecessarily loads all payloads | PENDING | | |
| A11 | P2 | File privacy inherits permissive defaults | PENDING | | |
| A13 | P2 | Upstream new-device scan repeats the first page | PENDING | | |
| A14 | P3 | Authenticated non-POST requests can mutate state | PENDING | | |
| A15 | P3 | In-memory database connection lifetime/isolation | PENDING | | |

## Validation And Scope

Use `GO=/path/to/go` with the existing check, smoke, upgrade and build helpers.
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
to 0.4 for the unreleased series. Commit subject `[A12]`; push pending until the
next tracking update (now confirmed pushed as `287806f`). A helper edited during its first run produced a shell
parser failure after the matrix; the complete stable helper was rerun successfully.

### A03 - 2026-11-06 14:55 CET

All three reproductions failed before the fix: a claimed History type bypassed
version checks, another type replaced payloads, and a mismatched deletion tag
was accepted. Update now reads stored type/tag, rejects identity changes, applies
the History exception only to stored History, and uses stored tag for deletion.
Omitted tags remain valid. Both focused driver tests, complete check helper and
all three clean/WAL upgrade matrices passed. No schema, config or existing-row
rewrite. Commit subject `[A03]`; push recorded by the next tracking update.

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
not acknowledged and lost. Commit `[A08]`, push recorded next.

### A01 - 2026-11-06 15:03 CET

Red tests confirmed missed late commits and tied timestamps. Allocate from the
greater of the supplied timestamp and persisted clock/existing-row maximum + 1,
after acquiring SQLite's writer lock, before committing the entity. Fresh pointer
assignment preserves aliased creation times. Root versions stay unchanged.
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

### A02 - 2026-11-06 15:09 CET

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
