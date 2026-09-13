# sync-lite Agent Guidance

Aligned with installed `codex-coding-rules` revision `2026-09-12.2` on
2026-09-13. The shared skill and its references are read-only guidance.

## Scope And Read First

- Keep our custom changes inside `sync-lite/`. Official upstream updates may
  change files outside it through an explicitly authorized upstream sync; do not
  independently patch those files. See the upstream policy below.
- Read this file, [README.md](README.md), [DEVELOPMENT_SETUP.md](DEVELOPMENT_SETUP.md),
  [TODO.md](TODO.md), [FINISHED_TASKS.md](FINISHED_TASKS.md), and the latest
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
  Linux x86-64 is the supported server target. ARM/MIPS are unverified and
  deferred pending access to suitable testing hardware, not promised targets.
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
  `GOWORK=off go test ./...` and `go build -o sync-lite .`.
  Format touched Go files with `gofmt`; use `git diff --check` on intended changes.
  Inspect relevant code/helpers before relying on commands in a new code task.
- `scripts/run-with-env.sh` is documented as a development launcher, not a test,
  backup, or production deployment helper. Additional helpers are proposals in
  TODO, not existing capabilities.
- Run relevant safe checks during authorized implementation. Report agent-run
  automation, user-run local tests, remote device tests, and field use separately.
  Past passing tests and user reports do not prove complete protocol/security coverage.
- Ask the user to install missing tools with exact commands and a short reason;
  do not silently install packages or update the shared skill.
- Before the next code pass, check the actual installed Go version, module
  requirements, dependency/SQLite versions, and compatibility. The user updated
  Go in September; this does not update the deployed binary. Plan govulncheck
  and ShellCheck alongside regression tests; their integration remains in TODO.
- No standing commit/push authorization is established here. Obtain it for the
  relevant work; never infer it from historical publication discussions.

## Versioning And Records

- The documented tested release is `0.2`, dated 2026-02-13. Documentation-only
  updates do not bump the service version or imply a new deployed binary.
- Code changes require a version change. The next code release is `0.3`, then
  `0.4`, and so on; minor patches may use `0.3.1`. Components are integers:
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
