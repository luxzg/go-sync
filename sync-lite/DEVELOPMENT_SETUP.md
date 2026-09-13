# Development Setup

Read [AGENTS.md](AGENTS.md) and [README.md](README.md) first. The README deployment
chapter is authoritative for the tested server layout and Brave base URLs.

## Prerequisites

Use a Linux x86-64 build machine for the supported Debian/Ubuntu server target.
Install Git and Go on the build machine, following the
[official Go installation instructions](https://go.dev/doc/install), then check:

```bash
git --version
go version
```

Use a supported Go version compatible with the module requirements. Exact local
toolchain and dependency versions must be checked before the next code pass;
the development laptop's Go installation was updated in September 2026.

A compatible compiled binary needs no Go installation on the server. VS Code,
Codex, Go generators, and development linters are not runtime requirements.

## Clone, Run, Test, And Build

Clone **this fork**, which includes both the reused upstream sources and
`sync-lite/`. A clone of `brave/go-sync` alone does not include sync-lite.
In a directory where you want the source checkout:

```bash
git clone https://github.com/luxzg/go-sync.git
cd go-sync/sync-lite
git status --short
go mod tidy
```

All remaining commands in this chapter run from `go-sync/sync-lite/`. If you
already have the checkout, enter that directory instead of cloning again. Keep
the parent sources: sync-lite intentionally reuses local upstream packages.
`go mod tidy` may download dependencies and modify module records; review any
changes rather than treating them as an automatic dependency upgrade.

For a development run:

```bash
go run .
```

With the documented defaults, the service listens on port `8295` and creates
`./sync-lite.db` and SQLite journal files. Use an isolated development checkout
and synthetic test data, not a production database or real sync chain. Ensure
the port is available and keep HTTP access local. Stop with `Ctrl+C` before
continuing. For a local Brave test, use the base URL `http://localhost:8295/`,
not `/command/`.

Run the module's tests and build the binary:

```bash
GOWORK=off go test ./...
go build -o sync-lite .
```

`GOWORK=off` disables workspace mode; the current directory selects the sync-lite
module. Tests do not require a separately running server. The build produces
`./sync-lite` for the selected Go target; a native Linux x86-64 build is the
supported deployment path. Cross-compilation alone does not prove runtime support.

Continue with [Deploy on Debian/Ubuntu](README.md#deploy-on-debianubuntu-start-to-end)
to install that binary and configure systemd. Building on the server is also
possible with Go installed, but the production unit runs the binary, not `go run`.

## Optional VS Code Setup

Open the full fork checkout in VS Code and install the Go extension published by
Go Team at Google / go.dev. Codex is optional. There is no need to regenerate
sync-lite or install code-generation tools just to build the existing service.

For persistent local environment settings, use the documented
[`scripts/run-with-env.sh` workflow](README.md#option-b-local-script-for-non-systemd-runs).
Keep local `.env`, binaries, SQLite artifacts, and any credentials out of commits.
Sanitized `.env.example` templates remain tracked.

## Maintenance And Release Work

- Follow the [upstream-sync policy](AGENTS.md#syncing-the-fork-with-upstream).
  The fork is `luxzg/go-sync`, upstream is `brave/go-sync`. Review official
  upstream commits monthly, not just releases, and merge only after approval.
  Our custom adaptations remain inside `sync-lite/`; merging official upstream
  changes outside that folder is permitted. Discuss custom conflict resolutions.
- Before code changes, record the actual Go/toolchain and dependency versions,
  including the SQLite driver and embedded engine. Review official release notes
  and advisories before proposing compatible upgrades. Updating Go locally does
  not update an existing deployed binary.
- Add govulncheck for Go vulnerability checks and ShellCheck for shell scripts in
  the later tooling pass. Helpers and exact invocations will be documented after
  implementation and validation; neither tool is assumed installed today.
- Require the [data-safety checks](README.md#upgrading-safely) before binary or
  SQLite updates. Use synthetic prior-version fixtures, establish migration and
  rollback compatibility, and back up consistent real state before deployment.
- The current release is `0.2`; the next code release is `0.3`, with optional
  patch versions such as `0.3.1`. Documentation-only changes keep the version.
  Prepend dated changelog entries with time and timezone. Review intended changes
  with `git diff --check` and `git status --short`; commit and push need approval.

Prioritized helpers, regression tests, maintenance, and optional features are in
[TODO.md](TODO.md). MIPS/ARM validation is deferred in
[FINISHED_TASKS.md](FINISHED_TASKS.md#deferred--out-of-scope-by-decision).

## Historical Setup Notes

The original February 2026 workflow installed Go 1.26.0 manually under
`/usr/local/go`, added it to the shell PATH, installed `gowrap` and
`golangci-lint`, cloned `brave/go-sync`, and used VS Code with the Go extension
and Codex to create the new `sync-lite/` subfolder. It then ran `go mod tidy` and
built the new module. These are historical steps, not current prerequisites or
instructions to recreate the implementation. The current recipe above clones
the fork containing the completed subproject instead.
