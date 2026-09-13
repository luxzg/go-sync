# Open Work

Updated 2026-09-13. The user accepted these priorities for a separate code and
maintenance pass. This documentation pass does not implement them or authorize
installs, live-data access, deployment, commits, or pushes. Current release: `0.2`;
next code release: `0.3`. Follow [AGENTS.md](AGENTS.md) before starting.

## P1: Validation And Repeatability

- Review the current test suite and add meaningful regression coverage for
  same-chain devices with distinct device identities, isolation in both
  directions, stale-version conflicts, deletions, parent ID mapping, new-chain
  initialization, pagination boundaries, history expiry, and quota behavior.
  Existing six documented integration cases are a baseline, not full coverage.
- Add a small `scripts/check.sh` or `scripts/test.sh` with stable invocation,
  module-scoped tests, temporary synthetic SQLite fixtures, and clear cleanup.
  Review prerequisites and side effects first. Include appropriate Go checks and
  ShellCheck for shell helpers; keep online govulncheck separate from quick tests.
- Verify lifecycle behavior with synthetic data: SIGINT/SIGTERM, in-flight
  requests, restart, and WAL checkpoint completion or busy results. File-size
  observations and a startup log do not establish all of these properties.
- Review the existing TLS startup path for suitable tests of certificate/key
  configuration and HTTPS requests. A test-only TLS server alone does not prove
  the production certificate-loading or service startup path.
- Establish an upgrade-safety baseline before changing storage or dependencies:
  use synthetic prior-version databases to check schema/data preservation,
  read/write behavior, integrity, WAL recovery, and rollback compatibility.
  Document explicit migration requirements when needed. Before any actual
  deployment, require a consistent backup; never test against live user data or
  restore automatically.

## P2: Maintenance And Operations

- Perform a separately authorized, dated Go/toolchain/dependency maintenance
  review before the next code change. The user updated Go on the development
  laptop: record the installed compiler, module requirements, selected toolchain,
  and built/deployed versions separately. Check SQLite driver and embedded engine
  releases, compatibility, and advisories, not just the host SQLite package.
  Add govulncheck and ShellCheck to the documented workflow; ask the user to
  install missing tools rather than installing silently. No local version or
  vulnerability-check baseline was established in this documentation pass.
  Preserve the accepted upstream dependency graph unless a refactor is approved.
- Review `brave/go-sync` monthly and after relevant advisories. Compare commits
  since the fork's last incorporated upstream revision, not just release tags:
  `v0.1.21` is still listed as latest, but the user reported recent repository
  activity. Follow the [upstream-sync policy](AGENTS.md#syncing-the-fork-with-upstream):
  review first, merge official changes only with approval, adapt custom code
  inside `sync-lite/`, and validate before deployment. Record review date,
  revisions, findings, and any deferred changes. No unattended pulls or merges.
- Investigate the unverified report of excessive SQLite writes/SSD wear,
  described as a write after each query. Measure reads, commits, checkpoints,
  and filesystem I/O under a small synthetic workload before diagnosing or
  optimizing. Preserve durability and sync semantics; retain a regression test
  for any confirmed defect. Do not label this as a proven bug.
- Propose a build/package helper that verifies the Linux target and resulting
  artifact identity. Consider version output/config validation if inspection
  confirms a need; avoid introducing metrics or health endpoints by default.
- Document and, when approved, automate consistent SQLite backup and safe binary
  update/rollback procedures with synthetic validation. Keep restores explicitly
  authorized and preserve user-managed config, certificates, and backups.
- Assess the documented local HTTP bind scope and shutdown/checkpoint claims
  against code in a future authorized review. Preserve the tested setup pending
  that review; these are questions to verify, not confirmed defects.

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

Architecture reports are acknowledged but not active implementation work; see
[deferred MIPS/ARM support](FINISHED_TASKS.md#deferred--out-of-scope-by-decision).
