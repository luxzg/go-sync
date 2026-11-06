# Legacy Progress Tokens

## Decision For 0.4

Keep the existing scalar token contract and preserve stored entities. The 0.4
writer clock prevents new timestamp ties and late-commit omissions; it does not
repair a client's earlier omissions. No replay, token rewind, retimestamping,
client reset or database repair runs automatically.

The operator explicitly accepted these unrepaired historical-token limitations
when approving integration into `master` and artifact preparation on 2026-11-06.
This acceptance does not claim historical recovery or authorize later data changes.

This completes the local strategy assessment requested after remediation. A
recovery implementation is a separate, explicitly approved data/protocol change,
not a prerequisite silently bundled into the candidate. Existing useful sync
continues; historical completeness is not guaranteed by upgrading the binary.

## What The Current Contract Can Establish

The reused `command/command.go` decodes progress markers with `binary.Varint`,
requests at most 500 entries across all types in a response, and returns the
last emitted modification timestamp as that type's marker. SQLite's
`GetUpdatesForType` selects strictly greater timestamps, in ascending order.
The disposable polling hint is also scalar. A missing hint falls back to SQLite;
clearing it cannot discover a record below an existing client token.

There are two distinct legacy cases:

| Case | Example | Why new ordered writes do not repair it |
| --- | --- | --- |
| A tie crossing a page | 501 eligible records at timestamp 100; first page contains 500 and returns token 100 | The next `mtime > 100` query excludes the remaining record |
| An update committed below an issued token | A client has token 200 when an old writer commits a previously prepared timestamp 100 | The later poll excludes 100 even though that client never downloaded it |

Neither is necessarily missing database data. The server stores current rows,
not a per-device delivery ledger or every historical revision. From a scalar
token and those rows alone it cannot establish which records a device applied.
It also cannot recover expired History or overwritten payload revisions that
are no longer present. A successful current download is not proof of complete
past delivery.

## Repeatable Evidence

`remediation_legacy_token_test.go` seeds raw **synthetic** pre-0.4 timestamps,
then uses authenticated protobuf requests, rather than bypassing the HTTP layer
for downloads. Run from `sync-lite/`:

```bash
./scripts/validate-remediation.sh followups
```

The characterization tests establish:

- A 501-entry tied cohort, including folders and tombstones, still returns 500
  entries followed by an empty page with token 100; all 501 stored rows survive.
- Reads and reopening do not silently rewrite the original records or clocks.
- Restarting and downloading again from zero do not solve a cohort larger than
  the page limit. A client reset is therefore not a general solution.
- A legacy row below an existing token remains absent from incremental results,
  while a new write with a backwards proposed timestamp is ordered above it.
- A small zero-token scan can expose retained old rows, but does not establish
  how real browsers would merge/reapply them or justify rewinding real clients.

These tests intentionally describe a known limitation. They must be updated
alongside any future recovery implementation; passing is not a recovery claim.

## Options Evaluated

| Option | Benefit | Problem / decision |
| --- | --- | --- |
| Restart or clear only poll hints | No stored-data changes | Cannot move a client marker backwards; not a remedy |
| Change `>` to `>=` or enlarge the page | Might include the boundary cohort | Can repeat a full tied page forever; cannot guarantee a finite maximum cohort or recover older skipped updates; reject |
| Return a smaller timestamp after a partial cohort | No entity rewrite | Repeats the same first page without an additional cursor; reject |
| Rewrite all existing timestamps/versions on startup | Eventually makes some retained records visible again | Mutates valuable data, can create stale-write conflicts and a download burst, affects rollback; reject automatic migration |
| Change store birthday or reset client categories/chain | Can trigger some client reinitialization | Risks local/History loss and does not fix oversized ties by itself; reject as an upgrade instruction |
| Stable keyset continuation, then separately approved replay | Bounded traversal of ties without rewriting identities or payloads | Needs integration changes and browser evidence; preferred direction for a future investigation, not implemented |

## Preferred Future Design And Acceptance Gates

Treat **correct traversal** and **historical replay** as separate features.
Neither may infer that every older record was missed.

1. Prototype a versioned per-type cursor containing a last `(mtime, id)` key,
   with an explicitly scoped replay boundary/phase if replay is later approved.
   Read in stable `ORDER BY mtime, id`, continuing lexicographically. Preserve
   chain/type authorization and normal folder/expiry filtering. Do not use
   SQLite row offsets: deletion or concurrent writes can move them.
2. Keep decoding existing scalar tokens. A compound cursor cannot simply be
   appended to the old varint: the parent currently ignores trailing bytes
   when decoding. Introduce a reviewed lite-owned boundary that handles decoding,
   pagination and re-encoding together, without modifying parent sources.
   Coordinate the global 500-entry budget, marker pass-through for unvisited
   types, NEW_CLIENT/Nigori handling and polling hints. Malformed, oversized or
   mismatched cursors must fail predictably with bounded work, never trigger an
   unbounded scan or cross-chain access.
3. Demonstrate on isolated Brave desktop and Android profiles that the client
   preserves and echoes opaque marker bytes across polls, restart and type
   initialization. The protobuf byte field alone is not that evidence. Confirm
   equal-version duplicate delivery, tombstones and remote edits are handled
   safely. Do not use the user's working chain for this experiment.
4. Only after cursor safety is established, design an **opt-in**, bounded replay
   of retained current rows for an explicitly authorized chain/type. Tokens
   would record progress, not rewrite stored modification time, version, creation
   time, tags or encrypted bytes. A scalar boundary-cohort replay addresses ties
   only; discovering arbitrary older skipped updates needs a wider scan. Any
   such scan can redeliver already-applied rows and cannot promise reconstruction
   of expired/overwritten history. Define replay scope and completion explicitly.
5. Before enabling replay, require a consistent backup, reviewed rollback/token
   compatibility, explicit operator consent and independent synthetic upgrade
   checks. Define what an older binary does with the new token **before** rollout;
   current binary-only rollback tests do not cover a future cursor format.

Required future regressions: 499/500/501 and larger tied cohorts; exact identity
sets and finite completion; equal-version re-delivery; tombstones; multiple
types sharing the budget; separate chains; restart mid-page; inserted/updated/
expired rows between pages; late old records; malformed cursors; cancellation;
poll-cache hits/misses; old scalar input and old-binary rollback. Run both SQLite
backends. Real clients must then confirm item arrival and safe duplicate merging.

## Operator Guidance Today

Keep the chain and category settings intact. Use the normal
[two-device checklist](MANUAL_TESTING.md) to distinguish fresh sync from a
historical omission. Do not send databases, tokens, identifiers, keys or node
exports to public reports. If a specific older item appears missing, retain a
consistent private backup and request a scoped investigation; do not fabricate
server rows or run global SQL updates. This document is not a repair procedure.
