# Open Work

Updated 2026-10-19 for `0.3`. This file contains remaining work; completed `0.3`
checks/helpers are archived in [FINISHED_TASKS.md](FINISHED_TASKS.md).
Follow [AGENTS.md](AGENTS.md) before starting. The next normal code release is
`0.4`; focused patches may use `0.3.1`.

## P1: Validation And Repeatability

- Extend protocol tests for general batch parent-ID mapping, command-level
  quota/history-period boundaries, high-device-limit admission at 50/100 devices,
  AI Chat roundtrip, and larger 499/500/501-entry pagination workloads.
- Investigate timestamp ties across GetUpdates page boundaries: tokens use only
  mtime, so a page ending inside a tie can omit remaining tied records. Also
  review upstream new-client device scanning, which repeats token zero when more
  pages exist. Keep fixes compatible with Brave and confined to sync-lite or
  propose an upstream patch separately. These are pre-existing review findings,
  not regressions introduced by the driver switch.
- Extend lifecycle checks to shutdown during in-flight requests, shutdown timeout
  handling, damaged/missing TLS files, and failed startup cleanup. `0.3` already
  covers normal SIGINT/SIGTERM, restart, WAL recovery/busy reporting, real HTTPS,
  and incomplete TLS settings.
- Obtain operator confirmation of `0.2` -> `0.3` upgrade and actual Brave sync on
  the home server after a consistent backup. Synthetic upgrade tests passed;
  no production server was accessed or updated by the agent.
- Ask the MIPS reporter to build the optional backend and report actual sync,
  router model/CPU, OS/firmware versions, Go/Zig versions, exact command, and
  sanitized errors. [Original request](https://github.com/brave/go-sync/issues/393#issuecomment-6260138531).
  Keep MIPS/ARM support experimental until actual target validation is available.

## P2: Maintenance And Operations

- Refresh the [maintenance baseline](MAINTENANCE.md) monthly and after advisories;
  next routine review is November 2026. Check Go, both SQLite drivers/embedded
  engines, inherited modules, govulncheck, ShellCheck, and build prerequisites.
  This is a review reminder, not an unattended upgrade schedule.
- Watch for SQLite driver releases incorporating SQLite 3.54.1 or later. The
  selected latest driver tags both bundle 3.53.4; SQLite's newer release includes
  a 32-bit heap-buffer fix whose applicability needs review before promoting a
  MIPS production build. Do not independently patch generated driver internals
  or bump modernc's libc beyond the required matching pin.
- Review `brave/go-sync` monthly and after relevant advisories. Compare commits
  since the fork's last incorporated upstream revision, not just release tags:
  the current incorporated revision is `71f27af460f4524a1d4dac4c35d8e7ef2a072c50`.
  Follow the [upstream-sync policy](AGENTS.md#syncing-the-fork-with-upstream):
  review first, merge official changes only with approval, adapt custom code
  inside `sync-lite/`, and validate before deployment. Record review date,
  revisions, findings, and any deferred changes. No unattended pulls or merges.
- Investigate the unverified report of excessive SQLite writes/SSD wear,
  described as a write after each query. Measure reads, commits, checkpoints,
  and filesystem I/O under a small synthetic workload before diagnosing or
  optimizing. Preserve durability and sync semantics; retain a regression test
  for any confirmed defect. Do not label this as a proven bug.
- Extend build artifacts into reviewed release packages with required notices
  and source links, checksums, optional CI and downloadable binaries. The `0.3`
  build helper and version output already exist. Add config-check mode only if
  useful; avoid metrics or health endpoints by default.
- Automate the documented consistent backup and safe binary update/rollback
  procedure if requested. Keep restores explicitly authorized and preserve
  user-managed config, certificates, and backups; do not overwrite old backups.
- Assess HTTP header/resource timeouts for public hosting; the standalone lite
  server does not inherit timeout settings from upstream's HTTP server wrapper.

## P3: Optional Access Control And Coverage

- Design optional chain admission control for public hosting. Proposed setup:
  create a chain on a private LAN, identify its authenticated public chain ID,
  then enable a server-side allowlist before exposing the service. Confirm which
  identifier is safe and stable; never store the sync seed/password or encryption
  keys as configuration. Define rejection, existing-chain behavior, reload, and
  recovery before implementation. Preserve Brave auth/encryption and require no
  client modification. Keep the feature opt-in with current behavior unchanged
  by default. Test unknown-chain rejection and allowed-chain device joins.
- Preferences were only partly tested across desktop and Android. Same-platform
  coverage and exact client/version combinations remain unrecorded.

## Community Feedback Follow-Up

- Improve fork discoverability and consider a separate repository or upstream
  backend-selection contribution in a future structural pass. The root README
  still belongs to upstream; discuss the ownership/rule change before editing
  it. Keep the current source layout working until a replacement is validated.
  [Issue discussion](https://github.com/brave/go-sync/issues/393).
- Consider optional lightweight container packaging and downloadable release
  binaries after the native workflow is stable. A contributor offered packaging
  and testing help in the issue. Containers must remain optional; no Docker
  runtime requirement is introduced by this request.
- Clarify what a requested "sync profile export/import" would mean before
  planning it. A server database backup and a browser profile/chain export have
  different data and security requirements; the server cannot implement browser
  features by itself. [Browser-integration discussion](https://github.com/luxzg/go-sync/discussions/1#discussioncomment-17490864).
- Keep reports from other Chromium browser projects separate from Brave support
  claims. Client protocol/auth integration and custom UI/preference sync need
  their own tests and belong primarily to those browser projects.
- For public-server access control, assess the existing chain-allowlist proposal
  before mTLS/client-certificate features, especially Android provisioning.
  Do not treat a hidden hostname or Host-header restriction as authentication.
  [Public-access discussion](https://github.com/luxzg/go-sync/discussions/1#discussioncomment-17495429).
