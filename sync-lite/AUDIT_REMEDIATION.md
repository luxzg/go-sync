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
| A08 | P2 | Optional wire fields violate upstream semantic preconditions | IN PROGRESS | Semantic validation before dispatch | No storage/startup/config change |
| A01 | P1 | Precommit timestamps can fall behind issued tokens | PENDING | | |
| A02 | P1 | Tag ownership and resurrection accounting | PENDING | | |
| A06 | P2 | Concurrent absolute quota snapshots lose deltas | PENDING | | |
| A09 | P2 | Failed drain closes storage under active requests | PENDING | | |
| A05 | P2 | Expired History records remain forever | PENDING | | |
| A07 | P2 | Unvisited expired hints retain memory | PENDING | | |
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
