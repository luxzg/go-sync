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

Version `0.3.1` requires Go **1.26 or newer**. Use a maintained patch release;
Go 1.26.9 was used for the October 19 checks. The default shell previously selected
an older patch, so check `go version` rather than assuming which installation is
active. See [MAINTENANCE.md](MAINTENANCE.md) for the dated dependency/toolchain record.

For all validation helpers and the optional CGO build, install these development
prerequisites on Debian/Ubuntu if missing:

```bash
sudo apt update
sudo apt install -y git ca-certificates build-essential shellcheck python3 openssl
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
```

Ensure the chosen Go installation and Go's binary directory are on `PATH`.
Building only the default modernc binary does not require GCC, Zig, Python,
ShellCheck, OpenSSL, or govulncheck. Zig is needed only for the documented
experimental cross-compiler example and is installed separately by its user.

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
./scripts/build.sh
./dist/sync-lite-linux-amd64 --version
```

`GOWORK=off` disables workspace mode; the current directory selects the sync-lite
module. Tests do not require a separately running server. The build helper
produces `dist/sync-lite-linux-amd64`, its SHA-256 checksum, and Go build metadata.
It defaults to Linux x86-64 with CGO disabled. The earlier
`go build -o sync-lite .` command also remains valid; the helper is preferred for
a static deployment binary. Cross-compilation alone does not prove runtime support.

Continue with [Deploy on Debian/Ubuntu](README.md#deploy-on-debianubuntu-start-to-end)
to install that binary and configure systemd. Building on the server is also
possible with Go installed, but the production unit runs the binary, not `go run`.

## Validation Commands

From `sync-lite/`:

| Command | Scope and effects |
| --- | --- |
| `./scripts/check.sh` | gofmt check, ShellCheck, vet, default no-CGO tests, race-enabled tests for both drivers, and the invalid-CGO build guard; synthetic temporary databases |
| `./scripts/test-upgrade.sh` | Archives an explicit source allowlist from `0.2` commit `1c0d957`, compiles old/new test executables, tests both drivers and old-version reopen on clean/WAL fixtures, then removes its own temporary files |
| `./scripts/smoke-test.sh` | Builds both drivers in a temporary directory; creates disposable TLS certificates and databases; tests actual loopback HTTP/HTTPS startup, aliases, SIGINT/SIGTERM, persistence, and invalid TLS settings |
| `./scripts/security-check.sh` | Runs govulncheck against both driver configurations; needs network access to the vulnerability database and may download module metadata |
| `./scripts/build.sh` | Builds the preferred static Linux x86-64 binary and metadata in ignored `dist/` |
| `./scripts/build.sh mattn` | Builds the optional CGO binary in `dist/`; requires a suitable C compiler |

The helpers do not install packages, deploy, restart installed services, or read
live SQLite/config/certificate files. Tests use synthetic fixtures. The upgrade
helper requires fork history containing its fixed `0.2` baseline; a shallow
clone may need its history fetched first. The smoke helper additionally needs
Python 3 and OpenSSL, only on the development machine.

The upgrade helper defaults to baseline `1c0d957` (`0.2`). To additionally check
the actual `0.3` release in isolated fixtures, use:

```bash
SYNC_LITE_UPGRADE_BASELINE=287bdec ./scripts/test-upgrade.sh
```

This override selects archived source to compile and run, not a live database.
Use only a reviewed, trusted repository revision. Both matrices keep the same
disposable-fixture safeguards.

Use `GO=/path/to/go ./scripts/check.sh` (and the same override for build, upgrade,
or smoke helpers) to select a specific installed compiler. For govulncheck,
select the compiler through `PATH`; set `GOVULNCHECK=/path/to/govulncheck` if it
is not already found there. No helper silently installs a compiler or scanner.

An optional custom mattn build uses `CGO_ENABLED=1 go build -tags sqlite_cgo .`.
See the [experimental MIPS example](README.md#experimental-mips-build) for target
settings. The `sqlite_cgo` tag intentionally fails when CGO is disabled. Both
drivers must pass the suite after future storage changes. Keep modernc's libc
pin aligned with the exact version required by the selected modernc release.

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
- Use the validation helpers above, including govulncheck and ShellCheck. The
  October baseline is in MAINTENANCE; refresh it monthly and after advisories.
- Require the [data-safety checks](README.md#upgrading-safely) before binary or
  SQLite updates. Use synthetic prior-version fixtures, establish migration and
  rollback compatibility, and back up consistent real state before deployment.
- The current release is `0.3.1`; the next normal code release is `0.4`, with optional
  patch versions such as `0.3.2`. Documentation-only changes keep the version.
  Prepend dated changelog entries with time and timezone. Review intended changes
  with `git diff --check` and `git status --short`. Standing user authorization
  covers committing and pushing completed authorized passes unless explicitly
  held. If push fails, ask the user to push; never force-push to work around it.

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
