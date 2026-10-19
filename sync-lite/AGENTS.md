# sync-lite Agent Guidance

Aligned with installed `codex-coding-rules` revision `2026-09-12.2` on
2026-09-13. The shared skill and its references are read-only guidance.

## Scope And Read First

- Keep our custom changes inside `sync-lite/`. Official upstream updates may
  change files outside it through an explicitly authorized upstream sync; do not
  independently patch those files. See the upstream policy below.
- Read this file, [README.md](README.md), [DEVELOPMENT_SETUP.md](DEVELOPMENT_SETUP.md),
  [TODO.md](TODO.md), [FINISHED_TASKS.md](FINISHED_TASKS.md),
  [MAINTENANCE.md](MAINTENANCE.md), and the latest
  [CHANGELOG.md](CHANGELOG.md) entries before substantial work. Read critical
  project docs in full; preserve the established documentation filenames.
- Follow the installed skill's full `references/AGENT_RULES.md` and applicable
  `references/VALIDATION_AND_OPERATIONS.md` sections, subject to higher-priority
  instructions and these project-specific exceptions.
- Establish execution, documentation-only, or answer-only mode each time.
  Documentation-only authorization does not permit inspecting code, configuration,
  secrets, or private data when the user excludes those. It does not authorize
  builds, tests, installs, service actions, commits, or pushes.
- Check available tools and effective permissions in the current session.
  Earlier download/cache failures do not establish current tool limitations.
  Use supported scoped approval for denied authorized writes; do not bypass it.

## Accepted Project Decisions

- Maintain a simple Go service for a few devices on Debian/Ubuntu Linux x86-64,
  with SQLite, no Docker requirement, and no DynamoDB or Redis service requirement.
  Linux x86-64 is the supported server target. Modernc stays the preferred
  default. Version `0.3` adds an optional mattn build selected with `sqlite_cgo`;
  CGO and a suitable C compiler are required. Both backends have automated
  Linux x86-64 coverage. MIPS/ARM remain experimental and hardware-unverified.
- Driver choice belongs in the small build-tagged driver files. Share datastore
  queries/schema and sync behavior; do not import both drivers into production
  or change the default merely because CGO happens to be enabled. Preserve
  per-connection WAL, synchronous=FULL, and 5000 ms busy-timeout settings using each driver's DSN
  options. `sqlite_cgo` with CGO disabled must fail during compilation.
- Preserve Brave sync API logic, authentication, protobuf fields, and chain
  semantics. The deliberate URL change is `POST /command/`, with
  `POST /v2/command/` retained as an alias.
- The README deployment chapter is the user's tested source of truth. Brave
  takes a base URL such as `https://go-sync-lite.your-domain.com:8295/`.
  Do not append `/command/` to that setting. Direct endpoint checks use `/command/`.
  The upstream client fix for issue #48909 does not retire either server alias.
- Keep port `8295` for HTTP localhost testing and HTTPS deployment. Do not
  silently switch to port 443 or introduce a reverse proxy requirement.
- Preserve systemd `EnvironmentFile` configuration and the optional development
  shell helper. Do not change to TOML merely to follow generic defaults.
- Preserve the documented binary/config/state layout and
  `WorkingDirectory=/var/lib/sync-lite`; `/opt` is not required for binary deployment.
- Upstream Go package reuse is intentional. The user accepted inherited
  AWS/Redis/Prometheus build dependencies for now; reducing them requires a
  separate refactor. Runtime service removal is not dependency removal.
- No existing DynamoDB data migration was required for the initial release.
  Existing SQLite data from subsequent real use must be preserved.
- Metrics and health-check endpoints are intentionally omitted. Do not add
  them solely to satisfy generic service guidance.
- Certificate acquisition, renewal, readable copies, and service restart are
  operator responsibilities. Preserve the accepted copy-and-restart workaround;
  do not change the user's ACME setup automatically.

## Syncing The Fork With Upstream

- Upstream is `brave/go-sync`; this fork is `luxzg/go-sync`. "Syncing the fork
  with upstream" means incorporating official upstream commits, normally by
  merging upstream changes, not writing custom modifications to upstream files.
- Review upstream monthly during maintenance, including commits since the last
  incorporated revision, not only releases. This is not an unattended schedule
  or standing permission to fetch, merge, deploy, commit, or push.
- In an authorized maintenance pass, fetch for comparison, review changes and
  compatibility, then obtain approval for the merge. Preserve fork history and
  unrelated work. If conflicts require custom edits outside `sync-lite/`, stop
  and discuss the resolution rather than silently choosing it.
- Record the incorporated upstream revision and validate the reused API/auth
  behavior, SQLite compatibility, and sync-lite regressions before deployment.
  Make any needed custom adaptations inside `sync-lite/` only.

## Workflow And Safety

- Preserve unrelated edits and historical records. Use synthetic isolated
  fixtures for tests; never use live sync chains, browser profiles, phones,
  SQLite files, certs, keys, local `.env`, backups, or private diagnostics without
  specific authorization and a necessary task scope.
- A documented command is not authorization to run it. Do not restart production
  services, copy credentials, restore data, or touch real devices during testing.
- Consistent backups require quiesced writers or a supported snapshot procedure.
  Retain required WAL state; do not infer backup safety from file sizes or remove
  journal files manually. A backup does not authorize a restore.
- Sessions recovery work must remain scoped to explicitly selected identities.
  Same-tag tombstones under other server IDs are not the missing records, and
  a successful synthetic retry is not proof of real-client recovery. The
  [reconciliation experiment](MAINTENANCE.md#2026-10-19-synthetic-sessions-reconciliation-investigation)
  is test-only; its helper accepts no database path and must never become an
  automatic upsert or bypass of authentication/version checks. Production repair
  requires a fresh, consistent backup, rechecked preconditions, and explicit scope.
- Do not recommend toggling a combined History and tabs setting to clear Sessions
  metadata. Disabling History can remove imported visits; server-side History
  expires after 14 days, so a lossless re-download cannot be assumed. See the
  [manual-testing warning](MANUAL_TESTING.md#diagnosing-persistent-commit-conflicts).
- Before deploying binary, dependency, or SQLite changes, require a consistent
  backup and isolated upgrade/read-write/restart/integrity checks using synthetic
  representative databases from the prior version. Determine migration needs
  explicitly and verify rollback compatibility; an older binary may not read a
  newer schema. Never migrate, reset, or restore valuable live data implicitly.
- Keep `.env`, binaries, and SQLite runtime artifacts out of commits. Preserve
  `.env.example` and other sanitized templates. Package only intended artifacts.
- Public documentation uses generic hosts such as `go-sync-lite.your-domain.com`
  or `your-server`, never provider-specific examples or personal paths. Public
  repository attribution, including the `luxzg` fork URL, is acceptable.
- For authorized code work, the documented commands run from `sync-lite/`:
  `./scripts/check.sh`, `./scripts/test-upgrade.sh`, `./scripts/smoke-test.sh`,
  `./scripts/security-check.sh`, and `./scripts/build.sh`.
  `GOWORK=off go test ./...` and `go build -o sync-lite .` remain valid.
  The preferred deployment artifact is `dist/sync-lite-linux-amd64`, built with
  CGO disabled. Verify it with `--version`; do not deploy a stale earlier build.
  Format touched Go files with `gofmt`; use `git diff --check` on intended changes.
  Inspect relevant code/helpers before relying on commands in a new code task.
- `scripts/run-with-env.sh` remains a development launcher. Check/upgrade/smoke
  helpers own their temporary synthetic state and processes. The upgrade helper
  archives an explicit source allowlist from `0.2` baseline `1c0d957`; it never
  reads or restores a live database. No helper automates production backup or
  installation. See DEVELOPMENT_SETUP for prerequisites and GO/tool overrides.
- Run relevant safe checks during authorized implementation. Report agent-run
  automation, user-run local tests, remote device tests, and field use separately.
  Past passing tests and user reports do not prove complete protocol/security coverage.
- Ask the user to install missing tools with exact commands and a short reason;
  do not silently install packages or update the shared skill.
- The October 2026 maintenance baseline is recorded in MAINTENANCE. Go 1.26+
  is required; checks used Go 1.26.9, govulncheck, and ShellCheck. Check actual
  toolchain/dependency versions again at the next maintenance pass. Keep modernc's
  libc dependency pinned to its upstream-required version, not an arbitrary
  independent upgrade. Compiler/library updates require rebuilt artifacts.
- The user granted standing commit/push authorization on 2026-10-19: after a
  coherent authorized pass or part of a larger task, validate and review the
  intended changes, commit, and push without asking again. Honor explicit holds
  on committing or pushing. If push fails, report the failure and ask the user
  to push; do not force-push or leave completed work unpublished without reason.
  This does not authorize new work, upstream merges, deployments, or data changes.

## Versioning And Records

- The current release is `0.3.1`, dated 2026-10-19. Documentation-only
  updates do not bump the service version or imply a new deployed binary.
- Code changes require a version change. The next normal code release is `0.4`;
  minor patches may use `0.3.2`. Components are integers:
  `0.9` is followed by `0.10`, not automatically `1.0`.
- Prepend CHANGELOG entries, newest first, including documentation-only changes.
  Use `## docs - YYYY-MM-DD HH:MM TZ (timezone)` or the version in place of
  `docs` for code releases. Obtain the current local time for each new entry.
  Preserve historical facts; where time/timezone was not recorded, say so rather
  than inventing it. The user authorized chronological reordering of old entries.
- TODO contains only open work; archive completed, skipped, dropped, or deferred
  decisions in FINISHED_TASKS. Keep proposals distinct from authorized implementation.
- Record late decisions and field observations with their evidence and limits.
  The latest field report is 2026-09-13: updated Debian Flatpak and Android Nightly
  APK clients resume sync after downtime, service restarts, and server reboots.
  README records the tested categories and limits; do not call this agent testing.
  Automated `0.3` validation on 2026-10-19 is separate from those `0.2` browser
  field results. Do not claim MIPS build/runtime success without user evidence.
- The 2026-10-19 `0.3` operator check found desktop-to-phone bookmark delivery,
  with an initially absent reverse rename/groups and Android backoff/CONFLICT.
  A later `0.3.1` report confirmed the reverse rename arrived, and fresh groups
  arrived in both directions, while missing Sessions (`50119`) conflicts recur.
  Its start date and cause remain unknown; do not call it a proven new regression, full sync
  outage, or a resolved issue. `0.3.1` adds opt-in
  `SYNC_LOG_CONFLICTS=1` diagnostics only; do not claim it fixes the incident.
  Keep logs bounded and limited to fixed reasons and numeric data types; never
  add IDs, versions, names, URLs, keys, or payloads to these diagnostics.
