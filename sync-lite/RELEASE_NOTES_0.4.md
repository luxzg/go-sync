# sync-lite 0.4

The completed review/remediation is integrated into this fork's `master`.
Linux x86-64 binaries and archives are prepared for controlled operator testing
and backup-first deployment. This is not a claim of real-browser validation or
an update to any running server. Last operator-tested version: 0.3.1.

## Fixed

- **A01:** durable transaction-ordered timestamps prevent new missed updates and
  timestamp ties; ordinary root updates advance their version correctly.
- **A02/A03:** preserve stored type/tag ownership, reject identity changes, and
  account for deletion/resurrection without releasing another item's tag.
- **A04/A08/A14:** bound wire/decoded uploads to 10 MiB, reject malformed gzip
  and unsafe protobuf inputs, set HTTP deadlines, and require authenticated POST.
- **A05:** retire explicitly expired History-family rows in bounded passes,
  preserving live tag owners and clocks; recommitted expired History is visible.
- **A06:** coordinate command accounting and cache publication without holding
  the command gate during network uploads or response writes.
- **A07:** bound disposable in-memory hints without evicting pending counters.
- **A09:** propagate cancellation, drain tracked requests before closing SQLite,
  validate TLS before opening storage, and make disable-and-clear atomic.
- **A10:** clear data using identity metadata instead of loading all payloads.
- **A11:** create new database files privately; retain deliberate existing modes.
- **A12:** make restart/upgrade tests detect missing state and independently
  exercise clean and old-WAL databases through both drivers and old-version reopen.
- **A13:** avoid the repeated-page new-device admission loop; retain 50/100 limits.
- **A15:** keep each disposable in-memory database on one retained connection.

Additional regressions cover nested bookmark parent-ID mapping, plain/opaque AI
Chat payload lifecycles and unknown specifics, quota boundaries/legacy boosts,
History bucket rotation, counter-version states and historical-token limitations.
See [full evidence](AUDIT_REMEDIATION.md) for checks and compatibility limits.

## Artifacts

From a clean, committed checkout, run:

```bash
./scripts/release-artifacts.sh build
./scripts/release-artifacts.sh verify
```

These helpers use installed Go 1.26.10 under `~/sdk/`, build both Linux amd64
backends and prepare versioned `.tar.gz` archives, source, SHA-256 checksums and
build/runtime information under ignored `dist/`. Archives include sanitized
deployment examples, documentation, source references and license notices.
They never install, publish a GitHub release, deploy, or access production data.

After extracting a binary archive, the executable is `./sync-lite`; check it
with `./sync-lite --version`. The included examples are templates, not replacements
for your existing configuration. Source-build commands in the general docs need
the separate source checkout/archive, not just the binary package.

Use **`dist/sync-lite-linux-amd64`**, the static modernc build, for the preferred
Debian/Ubuntu deployment. The optional `-cgo` binary requires a compatible system
C library; check its `.runtime.txt` report or packaged `RUNTIME.txt`. It is not
an experimentally cross-compiled MIPS binary. MIPS/ARM hardware remains untested.
The prepared native CGO build requires `libc.so.6` and GLIBC symbols through
2.34; use the static default if that compatibility is uncertain.

## Updating

Follow the [0.4 upgrade guide](README.md#upgrade-to-04) and then the
[two-device checklist](MANUAL_TESTING.md). Keep the same configuration, database,
certificates, chain and base URL. Both `/command/` and `/v2/command/` remain.
The only schema addition is `sync_clocks`, created automatically; no export,
import or client reset is required. Keep a consistent backup before replacement.

Historical token omissions are an **accepted unrepaired limitation** for 0.4;
there is no replay, global retimestamping or reset. See
[legacy token limits](LEGACY_TOKENS.md). Normal explicit History TTL retirement
cannot be undone by binary-only rollback. Quotas remain approximate; inherited
History-cap acknowledgements do not always imply persistence. No upstream-source
or dependency update is bundled into this final integration/preparation pass.
