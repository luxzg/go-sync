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
| A12 | P2 | Restart reseeding and chained WAL tests mask failures | IN PROGRESS | Smoke checks seed once; per-driver old-WAL fixtures; reject duplicate identities | Actual archived versions; validation pending |
| A03 | P1 | Incoming identity controls stored-type/version policy | PENDING | | |
| A04 | P1 | Incomplete HTTP limits and absent deadlines | PENDING | | |
| A08 | P2 | Optional wire fields violate upstream semantic preconditions | PENDING | | |
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
next tracking update. A helper edited during its first run produced a shell
parser failure after the matrix; the complete stable helper was rerun successfully.
