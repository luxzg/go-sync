# sync-lite

Minimal Brave Sync server implementation for small self-hosted setups.

This is a fork of https://github.com/brave/go-sync.
All custom modifications are contained within `/sync-lite/`; official upstream
updates may also be merged into the fork.

Get the source from [luxzg/go-sync](https://github.com/luxzg/go-sync/tree/master/sync-lite).
Supported server target: **Debian/Ubuntu Linux x86-64**. ARM and MIPS are currently
unverified; see [deferred platform support](FINISHED_TASKS.md#deferred--out-of-scope-by-decision).

## Documentation
- [Agent guidance](AGENTS.md): scope, accepted decisions, safety, and workflow.
- [Development setup](DEVELOPMENT_SETUP.md): clone, run, test, and build instructions.
- [Open work](TODO.md): prioritized follow-up work.
- [Completed and deferred work](FINISHED_TASKS.md): status and evidence archive.
- [Changelog](CHANGELOG.md): release and documentation history.

## Upstream Reference
- Original project: https://github.com/brave/go-sync
- Latest listed upstream release: [`v0.1.21`](https://github.com/brave/go-sync/releases/tag/v0.1.21), released by `@johnhalbert` on **September 18, 2023**.

Upstream repository changes are reviewed separately from release tags. See the
[upstream-sync policy](AGENTS.md#syncing-the-fork-with-upstream).

## Version
- Current `sync-lite` version: `0.2` (2026-02-13)

Documentation updates do not change the binary version. The next code release
will be `0.3`; patch releases may use `0.3.1`. Minor versions count upward as
integers, so `0.9` is followed by `0.10`.

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
```bash
scp sync-lite user@server:/tmp/
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

Repeatable backup/update helpers and automated upgrade checks are tracked in
[TODO.md](TODO.md); they are not implemented yet.

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
