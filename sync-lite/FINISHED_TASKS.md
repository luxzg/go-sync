# Completed And Deferred Work

This archive records scope and decisions from the project conversation. It does
not claim fresh code verification or replace the changelog timeline.

## Completed / Reported

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
- Linux x86-64 remains the supported server target, agreed 2026-09-13. ARM and
  MIPS support are deferred because testing hardware is unavailable. A reporter's
  `GOARCH=mipsle GOMIPS=softfloat go build` failed with `modernc.org/sqlite`;
  they reported success after switching to `github.com/mattn/go-sqlite3` and using
  `zig cc -target mipsel-linux-musleabi -static`. Neither result was reproduced
  here. Pure-Go implementation does not establish support for every Go target.
  Do not replace the working SQLite driver on this report alone; reopen when
  target validation is possible. The separate write-amplification concern stays
  active in TODO because it can be investigated on the supported target.
