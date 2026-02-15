# sync-lite

Minimal Brave Sync server implementation for small self-hosted setups.

This is a fork of https://github.com/brave/go-sync.
All modifications are contained within `/sync-lite/`.

## Upstream Reference
- Original project: https://github.com/brave/go-sync
- Upstream latest release: `v0.1.21` (Latest), released by `@johnhalbert` on **September 18, 2023**
  - Source: https://github.com/brave/go-sync/releases

## Version
- Current `sync-lite` version: `0.2` (2026-02-13)

## What it keeps
- Brave sync protobuf API behavior via `ClientToServerMessage` and `ClientToServerResponse`
- Auth token validation compatible with Brave's existing server logic
- Commit/GetUpdates/ClearServerData flow from Brave `go-sync` command logic

## What it removes
- DynamoDB
- Redis
- Prometheus/metrics
- Health-check endpoints

## Endpoints
- `POST /command/` (primary endpoint)
- `POST /v2/command/` (compatibility alias)

`/command/` is provided to work around Brave URL behavior that may strip `/v2/`.

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

### 2) `/v2` stripping bug (open)
There is an open Brave issue where `/v2` may be stripped after relaunch:
- https://github.com/brave/brave-browser/issues/48909 (Open)

Because of that, `sync-lite` supports `POST /command/` directly.

## Run
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
1. Copy env template:
```bash
sudo mkdir -p /etc/sync-lite /var/lib/sync-lite
sudo cp sync-lite/deploy/sync-lite.env.example /etc/sync-lite/sync-lite.env
sudoedit /etc/sync-lite/sync-lite.env
```
2. Build/install binary:
```bash
cd sync-lite
go build -o /tmp/sync-lite .
sudo mv /tmp/sync-lite /usr/local/bin/sync-lite
```
3. Install unit:
```bash
sudo cp deploy/sync-lite.service.example /etc/systemd/system/sync-lite.service
sudo systemctl daemon-reload
sudo systemctl enable --now sync-lite
```

Now cert/key paths and all runtime config are persisted in `/etc/sync-lite/sync-lite.env`.

#### Option B: local script for non-systemd runs
```bash
cd sync-lite
cp deploy/sync-lite.env.example .env
$EDITOR .env
./scripts/run-with-env.sh ./.env
```

Then point Brave to:

```text
https://your-server/
```

## Deploy on Debian/Ubuntu (Start to End)
This is a deployment flow for a small self-hosted setup.

### 1) Prepare DNS/network/certificate
1. Get a domain and point it to your server.
2. Make sure NAT/port-forwarding allows your chosen port (here: `8295`) to reach the server.
3. Make sure to obtain a valid TLS certificate (for example with Let's Encrypt).

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
TLS_CERT_FILE=/etc/letsencrypt/live/your-domain/fullchain.cer
TLS_KEY_FILE=/etc/letsencrypt/live/your-domain/privkey.key

# Optional comma-separated allowlists/denylists
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
https://brave-sync-lite.duckdns.org:8295/
```

### 5) Install and start systemd service
```bash
sudo nano /etc/systemd/system/sync-lite.service
```

Populate with this example service unit:
```bash
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
sudo systemctl start sync-lite
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

### 7) Configure Brave sync URL
Use:
```text
http://localhost:8295/
```
for local HTTP testing, or:
```text
https://your-domain:8295/
```
for remote HTTPS deployment.

Notes:
1. `sync-lite` also accepts `/v2/command/`, but `/command/` is recommended because of Brave `/v2` stripping issue.
2. If TLS certs are auto-renewed, restart service after renewal if your environment does not hot-reload cert files.

## Testing
Run:

```bash
cd sync-lite
GOWORK=off go test ./...
```

`GOWORK=off` ensures tests run only for the `sync-lite` module, even if you have a workspace (`go.work`) configured.

Current test coverage includes integration tests for:
- Unauthorized `/command/` request handling
- Commit + GetUpdates roundtrip
- Disabled-chain response (`DISABLED_BY_ADMIN`)
- Gzip request body handling
- Multi-client isolation (no cross-chain data leakage)
- Multi-client same-ID commit behavior (no cross-client conflict)

## Validation Results (Real Device Testing)
Additional real-world validation was performed with two Brave clients using the same sync-server override URL:
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

Also validated:
- SQLite-stored sync payloads are encrypted
- `brave://sync-internals/` shows Nigori keys
- `brave://sync-internals/` data is consistent across both tested devices

Based on these tests and practical use, sync-lite is functioning correctly in this setup.
