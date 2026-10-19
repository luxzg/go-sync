# sync-lite

Minimal Brave Sync server implementation for small self-hosted setups.

This is a fork of https://github.com/brave/go-sync.
All custom modifications are contained within `/sync-lite/`; official upstream
updates may also be merged into the fork.

Get the source from [luxzg/go-sync](https://github.com/luxzg/go-sync/tree/master/sync-lite).
Supported server target: **Debian/Ubuntu Linux x86-64**. ARM and MIPS are currently
unverified. An [experimental MIPS build option](#experimental-mips-build) is
available for community testing.

## Documentation
- [Agent guidance](AGENTS.md): scope, accepted decisions, safety, and workflow.
- [Development setup](DEVELOPMENT_SETUP.md): clone, run, test, and build instructions.
- [Open work](TODO.md): prioritized follow-up work.
- [Completed and deferred work](FINISHED_TASKS.md): status and evidence archive.
- [Changelog](CHANGELOG.md): release and documentation history.
- [Maintenance record](MAINTENANCE.md): checked versions, upstream revision, and validation results.

## Upstream Reference
- Original project: https://github.com/brave/go-sync
- Latest listed upstream release: [`v0.1.21`](https://github.com/brave/go-sync/releases/tag/v0.1.21), released by `@johnhalbert` on **September 18, 2023**.

Upstream repository changes are reviewed separately from release tags. See the
[upstream-sync policy](AGENTS.md#syncing-the-fork-with-upstream).

## Version
- Current `sync-lite` version: `0.3` (2026-10-19)

Documentation updates do not change the binary version. Patch releases may use
`0.3.1`; the next normal code release will be `0.4`. Minor versions count upward as
integers, so `0.9` is followed by `0.10`.

Print the binary version and selected backend without opening a database:

```bash
./sync-lite --version
```

Version `0.3` retains the existing configuration and database schema. Existing
users should use the default build and follow [Upgrade from 0.2](#upgrade-from-02-to-03).

## Choose A SQLite Build

Build from `go-sync/sync-lite/` with **Go 1.26 or newer**; a maintained patch
release is recommended. Go 1.26.9 was used for this release's checks.

| Build | Selection | Validation |
| --- | --- | --- |
| **Preferred: modernc** | Default; no C compiler required | Tested on Linux x86-64, including automated sync, HTTP/HTTPS, restart, and upgrade checks |
| Optional: mattn/go-sqlite3 | `-tags sqlite_cgo`, CGO and a C compiler required | Same checks passed on Linux x86-64; MIPS compilation and router operation remain untested here |

For the preferred Linux x86-64 binary:

```bash
./scripts/check.sh
./scripts/build.sh
./dist/sync-lite-linux-amd64 --version
```

The build helper disables CGO for the default build, producing a static binary
for Debian/Ubuntu deployment. The checks also exercise the optional backend and
therefore require a C compiler and ShellCheck on the **development machine**;
see [development prerequisites](DEVELOPMENT_SETUP.md#prerequisites). Building
only the preferred binary needs Go, without those test tools. The build helper
also writes SHA-256 and Go build-information files beside the binary.

`go build -o sync-lite .` still selects modernc. Use the helper above when you
want the documented static deployment artifact.

### Experimental MIPS Build

The optional backend addresses the [reported modernc MIPS limitation](https://github.com/brave/go-sync/issues/393#issuecomment-6260138531).
It uses the same sync logic and SQLite schema. Driver selection happens when
compiling; it is not an environment setting for an existing binary.

For a native Linux x86-64 mattn build, use `./scripts/build.sh mattn`. For the
reporter's MIPS little-endian, soft-float target, this is an **untested example**
requiring an appropriate Zig installation:

```bash
CC="zig cc --target=mipsel-linux-musleabi -static" \
GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
./scripts/build.sh mattn
```

Expected output: `dist/sync-lite-linux-mipsle-cgo`. The helper enables CGO and
adds `-tags sqlite_cgo`. A direct custom build must include both settings;
`-tags sqlite_cgo` with CGO disabled intentionally fails at compile time.
Static linking depends on the selected C toolchain and target; check the result
before deployment. A compiler is needed on the build machine, not the router.

Please report whether the build and actual sync work, along with the router
manufacturer/model, CPU architecture, OS/distribution, firmware version, Go/Zig
versions, and exact build command. Include sanitized errors and `--version`
output. Keep sync codes, tokens, certificates, databases, and private hostnames
out of public reports. MIPS/ARM remain community-test targets, not supported
deployment claims.

## What it keeps
- Brave sync protobuf API behavior via `ClientToServerMessage` and `ClientToServerResponse`
- Auth token validation compatible with Brave's existing server logic
- Commit/GetUpdates/ClearServerData flow from Brave `go-sync` command logic

## Runtime Simplifications
- External DynamoDB service requirement, replaced by SQLite
- External Redis service requirement, replaced by an in-memory adapter
- Prometheus/metrics
- Health-check endpoints

The build still reuses packages from the parent `go-sync` checkout. This retains
inherited AWS, Redis, and Prometheus module dependencies to keep changes small;
their presence does not mean those services need to run. SQLite is the added
storage dependency. Building from source requires
both the parent sources and `sync-lite/`; a plain upstream clone does not contain
these additions. A compatible prebuilt binary does not require Go on the server.

## Endpoints
- `POST /command/` (primary endpoint)
- `POST /v2/command/` (compatibility alias)

`/command/` originally worked around Brave stripping `/v2/` from the server URL.
That client bug has been fixed upstream; both endpoints remain supported.

## Configuration
- `LISTEN_ADDR` default `:8295`
- `SQLITE_PATH` default `./sync-lite.db`
- `TLS_CERT_FILE` optional path to certificate file (enable HTTPS when set with key)
- `TLS_KEY_FILE` optional path to private key file (enable HTTPS when set with cert)
- `BLOCKED_CLIENT_IDS` optional comma-separated list
- `HIGH_DEVICE_LIMIT_CLIENT_IDS` optional comma-separated list

## First Run and SQLite Schema
`sync-lite` creates/opens the SQLite DB file on startup and automatically creates required schema if missing.
- DB init path: `NewSQLiteStore(...)` in `sync-lite/sqlite_store.go`
- Schema bootstrap: `initSchema(...)` in `sync-lite/sqlite_store.go`
- WAL checkpoint behavior:
  - Startup runs `PRAGMA wal_checkpoint(TRUNCATE)` once.
  - Graceful shutdown (`SIGINT`/`SIGTERM`, including `Ctrl+C`) runs `PRAGMA wal_checkpoint(TRUNCATE)` again before DB close.
  - A busy checkpoint is reported as an error; journal files must never be deleted manually.
- Both drivers use synchronous=FULL and a 5000 ms busy timeout on each connection.

## Brave Setup Notes
### 1) Custom sync URL support (closed/resolved)
Brave has support for overriding sync server URL, including Android workflows. See:
- https://github.com/brave/brave-browser/issues/43181 (Closed)

### 2) `/v2` stripping bug (fixed upstream)
Brave issue [#48909](https://github.com/brave/brave-browser/issues/48909) is closed.
The [client fix](https://github.com/brave/brave-core/pull/37237) preserving the URL
path merged on June 18, 2026. Availability depends on the installed Brave version.

No sync-lite configuration change is needed: keep the base URL shown below.
Both `/command/` and `/v2/command/` remain accepted for compatibility.

## Run
First [clone the fork and prepare the module](DEVELOPMENT_SETUP.md#clone-run-test-and-build).
From the fork's root directory:

```bash
cd sync-lite
go mod tidy
go run .
```

### Run with HTTPS (Let's Encrypt)
Set both TLS vars and start the service:

```bash
export LISTEN_ADDR=":8295"
export TLS_CERT_FILE="/etc/letsencrypt/live/your-domain/fullchain.pem"
export TLS_KEY_FILE="/etc/letsencrypt/live/your-domain/privkey.pem"
go run .
```

If `TLS_CERT_FILE` and `TLS_KEY_FILE` are both set, `sync-lite` serves HTTPS directly via `ListenAndServeTLS`.

### Better than repeated `export`: use an env file
Use one env file and load it automatically instead of exporting every shell session.

Recommended files included in this repo:
- Env template: `sync-lite/deploy/sync-lite.env.example`
- systemd unit template: `sync-lite/deploy/sync-lite.service.example`
- Optional local helper script: `sync-lite/scripts/run-with-env.sh`

#### Option A (recommended): systemd + EnvironmentFile
Follow [Deploy on Debian/Ubuntu](#deploy-on-debianubuntu-start-to-end) below for
the complete user, binary, configuration, certificate, and service setup.
Runtime settings persist in `/etc/sync-lite/sync-lite.env`.

#### Option B: local script for non-systemd runs
From the fork's root directory (do not overwrite an existing `.env`):

```bash
cd sync-lite
cp deploy/sync-lite.env.example .env
$EDITOR .env
./scripts/run-with-env.sh ./.env
```

Then point Brave to:

```text
https://go-sync-lite.your-domain.com:8295/
```

Use `http://localhost:8295/` instead for local HTTP testing. Brave's setting takes
the base URL; `/command/` is the request endpoint used for direct smoke tests.

## Deploy on Debian/Ubuntu (Start to End)
This is the tested first-install flow for a small self-hosted setup. For an
existing installation, read [Upgrading Safely](#upgrading-safely) before replacing
anything.

The steps assume a compatible Linux binary has already been built or obtained.
Go is needed only on the build machine. Start with
[clone, run, test, and build](DEVELOPMENT_SETUP.md#clone-run-test-and-build), using
this fork rather than the original Brave repository. You can build on the server
if Go is installed there, or copy a compatible binary from another build machine.
`go run .` is useful for development; systemd runs the compiled binary below.

### 1) Prepare DNS/network/certificate
1. Get a domain and point it to your server.
2. Make sure NAT/port-forwarding allows your chosen port (here: `8295`) to reach the server.
3. Make sure to obtain a valid TLS certificate (for example with Let's Encrypt).

Allow inbound TCP `8295` in the server firewall for HTTPS. Keep plain HTTP testing
local and do not expose it publicly. `:8295` listens on available interfaces, not
only localhost; restrict access during the HTTP test.

### 2) Download or copy `sync-lite` binary
When built with the preferred helper, copy its output under the installed name:

```bash
scp dist/sync-lite-linux-amd64 user@server:/tmp/sync-lite
ssh user@server
sudo mv /tmp/sync-lite /usr/local/bin/sync-lite
sudo chmod +x /usr/local/bin/sync-lite
```

### 3) Create service user and directories
```bash
sudo useradd --system --home /var/lib/sync-lite --shell /usr/sbin/nologin sync-lite || true
sudo mkdir -p /etc/sync-lite /var/lib/sync-lite
sudo chown -R sync-lite:sync-lite /var/lib/sync-lite
```
Note: Make sure your new user `sync-lite` can access and read certificate and key files!

### 4) Create env config (HTTP test or HTTPS production)
Create env file:
```bash
sudo nano /etc/sync-lite/sync-lite.env
```
And you can start with example template:
```dotenv
# sync-lite runtime environment
# Copy to /etc/sync-lite/sync-lite.env (or another secure path)

LISTEN_ADDR=:8295
SQLITE_PATH=/var/lib/sync-lite/sync-lite.db

# Enable HTTPS by setting both values
# Make sure user that is used to run the binary can access these files!
TLS_CERT_FILE=/var/lib/sync-lite/fullchain.cer
TLS_KEY_FILE=/var/lib/sync-lite/private.key

# Optional comma-separated client-ID policy lists (not a chain admission allowlist)
BLOCKED_CLIENT_IDS=
HIGH_DEVICE_LIMIT_CLIENT_IDS=
```

Set one of these modes:

Mode A: HTTP local testing on `8295`
```dotenv
LISTEN_ADDR=:8295
SQLITE_PATH=/var/lib/sync-lite/sync-lite.db
TLS_CERT_FILE=
TLS_KEY_FILE=
```
Test URL example:
```text
http://localhost:8295/
```

Mode B: HTTPS on `8295`
```dotenv
LISTEN_ADDR=:8295
SQLITE_PATH=/var/lib/sync-lite/sync-lite.db
TLS_CERT_FILE=/var/lib/sync-lite/fullchain.cer
TLS_KEY_FILE=/var/lib/sync-lite/private.key
```
Example HTTPS URL:
```text
https://go-sync-lite.your-domain.com:8295/
```

### 5) Install and start systemd service
```bash
sudo nano /etc/systemd/system/sync-lite.service
```

Populate with this example service unit:
```ini
[Unit]
Description=sync-lite Brave Sync server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=sync-lite
Group=sync-lite
WorkingDirectory=/var/lib/sync-lite
EnvironmentFile=/etc/sync-lite/sync-lite.env
ExecStart=/usr/local/bin/sync-lite
Restart=on-failure
RestartSec=3s

# Optional hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/sync-lite

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sync-lite
sudo systemctl status sync-lite
```

### 6) Verify logs and connectivity
```bash
journalctl -u sync-lite -n 200 --no-pager
```

Quick endpoint smoke test:
```bash
curl -i http://localhost:8295/command/
```
Expected without token: `401 Unauthorized` (server reachable).

This smoke test applies to HTTP mode. In HTTPS mode, use your certificate's
hostname with HTTPS on the same port and the `/command/` endpoint. A 401 verifies
reachability and the unauthenticated response, not complete sync behavior.

### 7) Configure Brave sync URL
Use:
```text
http://localhost:8295/
```
for local HTTP testing, or:
```text
https://go-sync-lite.your-domain.com:8295/
```
for remote HTTPS deployment.

Notes:
1. Brave is configured with the base URL shown above. The server accepts `/command/` and the `/v2/command/` compatibility alias as request endpoints.
2. The documented native TLS setup requires a service restart to load renewed certificate files. The accepted operator workaround copies the renewed full chain and private key into `/var/lib/sync-lite/`, makes them readable by `sync-lite` with restricted permissions, and restarts the service. Use the exact filenames configured in the env file. Renewal and copy scheduling are external to this project; no ACME automation was added to sync-lite.

## Upgrading Safely
Existing SQLite data is valuable. The initial release needed no DynamoDB
migration, but that does not make later SQLite upgrades safe without checks.

1. Keep the working binary and obtain a consistent database backup before an
   update. Stop all writers before copying database files, or use a supported
   SQLite snapshot procedure. Include any required WAL state; never delete WAL
   files manually or assume a live copy of the main `.db` file is sufficient.
2. Before deploying a binary, dependency, or SQLite change, validate data/schema
   preservation, read/write behavior, integrity, and restart recovery against
   representative synthetic prior-version databases. Check whether migration is
   needed and document it explicitly before upgrading real data.
3. Preserve the existing env file, certificates, database ownership, and backups.
   Verify the deployed artifact and service after the planned restart, then
   confirm client synchronization.
4. Check old-binary compatibility before rollback. Replacing a binary is not a
   database restore; restoration must be an explicit operator decision, never
   automatic update or test cleanup.

Automated synthetic upgrade checks are available in `scripts/test-upgrade.sh`.
Automation of live backup/update operations remains tracked in [TODO.md](TODO.md).

### Upgrade From 0.2 To 0.3

**Keep your existing sync chain, SQLite database, env file, certificate paths,
and systemd unit. No schema migration, database export/import, or client reset
is needed for this update.** Use the preferred modernc build for existing servers.

The automated upgrade checks used the actual `0.2` code to create synthetic
databases, exercised both `0.3` drivers, and reopened them with `0.2`. They
checked schema, records, BLOB bytes, counts, integrity, and recovery of committed
WAL data. Back up your own installation before applying the update.

1. Build `0.3` on the development machine and transfer it to the server:

```bash
./scripts/build.sh
scp dist/sync-lite-linux-amd64 user@server:/tmp/sync-lite-0.3
```

2. On the server, check the candidate before stopping the service:

```bash
chmod +x /tmp/sync-lite-0.3
/tmp/sync-lite-0.3 --version
```

Expected: `sync-lite 0.3`, backend `modernc`, target `linux/amd64`.

3. Stop the service and make a private backup of the working installation:

```bash
sudo systemctl stop sync-lite
sudo install -d -m 0700 /var/backups/sync-lite
backup_dir="$(sudo mktemp -d /var/backups/sync-lite/pre-0.3.XXXXXXXX)"
sudo cp -a /var/lib/sync-lite "$backup_dir/state"
sudo cp -a /etc/sync-lite "$backup_dir/config"
sudo cp -a /usr/local/bin/sync-lite "$backup_dir/sync-lite-0.2"
```

Confirm the backup completed successfully before continuing. Keep any `.db-wal`
and `.db-shm` files with the stopped database. The state/config backup may contain
private keys; keep it private. Never run both versions against the database at
the same time.

4. Replace only the binary and start the existing unit:

```bash
sudo install -m 0755 /tmp/sync-lite-0.3 /usr/local/bin/sync-lite
/usr/local/bin/sync-lite --version
sudo systemctl start sync-lite
sudo systemctl status sync-lite
journalctl -u sync-lite -n 50 --no-pager
```

5. Confirm the startup log reports `0.3` and verify that your existing Brave
   clients resume synchronization. Their server URL stays the same.

If an update fails, stop the service before further action. The test matrix
verified reopening these synthetic databases with `0.2`, so binary-only rollback
is available for that tested scope. Keep the backup and ask for help if your
database or configuration differs; do not restore or replace user data
automatically. This procedure is for the documented systemd layout; adjust paths
only if your installation deliberately uses another layout.

## Public Hosting
The current server does not provide an operator-configured chain admission
allowlist. A reachable public instance permits new chains to register; normal
Brave chain authentication and encryption still apply. Use network restrictions
if access should be limited. An optional chain allowlist is planned for design
review, not available configuration today.

## Testing
From the fork's root directory:

```bash
cd sync-lite
GOWORK=off go test ./...
```

Run this from `sync-lite/`. `GOWORK=off` disables workspace mode; the working
directory and `./...` select this module's packages. It does not prevent their
upstream imports from compiling, and does not select sync-lite when run elsewhere.

The user reported the integration suite passing. Its documented cases cover:
- Unauthorized `/command/` request handling
- Commit + GetUpdates roundtrip
- Disabled-chain response (`DISABLED_BY_ADMIN`)
- Gzip request body handling
- Multi-client isolation (no cross-chain data leakage)
- Same client-side entity ID in different chains (no cross-chain conflict)

For `0.3`, the repeatable checks are:

```bash
./scripts/check.sh
./scripts/test-upgrade.sh
./scripts/smoke-test.sh
./scripts/security-check.sh
```

Run from `sync-lite/`; prerequisites and tool overrides are documented in
[development setup](DEVELOPMENT_SETUP.md#validation-commands). The upgrade helper
needs the full fork history, including the `0.2` baseline commit. All runtime
fixtures and server processes are isolated; these checks never use your live
database or installed service. The [maintenance record](MAINTENANCE.md) describes
the checked scope and remaining limits.

## Validation Results (Real Device Testing)
The user reported additional real-world validation with two Brave clients using the same sync-server override URL:
- Debian Linux Brave (Flatpak)
- Android Brave Nightly (APK)

Two-way sync was tested in both directions between these devices with sync categories enabled.

Confirmed working in testing:
- Passwords (including 1000+ imported entries and manual inserts)
- Bookmarks and bookmark folders
- Autofill address profiles
- General autofill form data
- Sessions
- Open tabs
- Tab groups
- History
- Preferences (partial; more same-platform testing may improve coverage confidence)

User observations also included:
- Encrypted sync payloads visible in SQLite (not a claim of whole-database or metadata encryption)
- `brave://sync-internals/` shows Nigori keys
- `brave://sync-internals/` data is consistent across both tested devices

These observations support the tested setup; they are not a cryptographic audit
or proof of compatibility with every client version or corner case.

### Longer-Term Use
On 2026-05-26 the user reported that syncing between desktops and a phone was
still working well after a couple of months of use.

On **2026-09-13**, syncing was again confirmed working between updated Debian
Brave (Flatpak) and Android Brave Nightly (APK), including passwords, bookmarks,
tabs, and tab groups. Testing included extended service downtime, service
restarts, and server reboots/restarts. Sync resumes once the service is available
and both clients have network access to it.

These are user-reported field results for version `0.2`, not automated recovery
tests or a guarantee for every platform and browser version.
