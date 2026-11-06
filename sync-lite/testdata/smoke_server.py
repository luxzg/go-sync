"""Exercise only helper-owned processes, certificates, and synthetic databases."""

import os
from pathlib import Path
import signal
import socket
import sqlite3
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.request


def probe(url, context=None):
    # Ignore operator HTTP proxy variables for these loopback-only checks.
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=context)
    )
    try:
        with opener.open(url, timeout=1) as response:
            return response.status
    except urllib.error.HTTPError as error:
        return error.code


def stop(process, requested_signal):
    if process.poll() is None:
        process.send_signal(requested_signal)
    try:
        stdout, stderr = process.communicate(timeout=20)
    except subprocess.TimeoutExpired:
        process.kill()
        process.communicate()
        raise AssertionError("owned server did not stop within 20 seconds")
    assert process.returncode == 0, f"server failed: {stdout} {stderr}"
    assert "stopped cleanly" in stderr, stderr
    assert "checkpoint error" not in stderr, stderr


def verify_payload(db):
    row = db.execute("SELECT specifics FROM sync_entities WHERE id='opaque-item'").fetchone()
    assert row == (b"\x00\xff\x80\x00synthetic",), "persisted sentinel missing or changed"


def run_server(binary, directory, tls, requested_signal, seed=False):
    with socket.socket() as available:
        available.bind(("127.0.0.1", 0))
        port = available.getsockname()[1]
    environment = os.environ.copy()
    database = directory / "synthetic.db"
    environment.update(
        LISTEN_ADDR=f"127.0.0.1:{port}",
        SQLITE_PATH=str(database),
        TLS_CERT_FILE=str(directory / "cert.pem") if tls else "",
        TLS_KEY_FILE=str(directory / "key.pem") if tls else "",
        BLOCKED_CLIENT_IDS="",
        HIGH_DEVICE_LIMIT_CLIENT_IDS="",
    )
    context = ssl.create_default_context(cafile=str(directory / "cert.pem")) if tls else None
    scheme = "https" if tls else "http"
    base = f"{scheme}://127.0.0.1:{port}"
    process = subprocess.Popen(
        [str(binary)], cwd=directory, env=environment,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )
    try:
        deadline = time.monotonic() + 15
        while True:
            if process.poll() is not None:
                stdout, stderr = process.communicate()
                raise AssertionError(f"server exited during startup: {stdout} {stderr}")
            try:
                if probe(base + "/command/", context) == 401:
                    break
            except (OSError, urllib.error.URLError):
                pass
            if time.monotonic() >= deadline:
                raise AssertionError("server never became reachable")
            time.sleep(0.05)
        assert probe(base + "/v2/command/", context) == 401
        with sqlite3.connect(database) as db:
            assert db.execute("PRAGMA journal_mode").fetchone()[0] == "wal"
            if seed:
                db.execute(
                    "INSERT INTO sync_entities "
                    "(client_id, id, specifics) VALUES ('smoke-chain', 'opaque-item', ?)",
                    (b"\x00\xff\x80\x00synthetic",),
                )
            else:
                # Check immediately after restart, before any fixture writes.
                verify_payload(db)
        stop(process, requested_signal)
        with sqlite3.connect(database) as db:
            assert db.execute("PRAGMA integrity_check").fetchone()[0] == "ok"
            verify_payload(db)
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate()


def main():
    work = Path(sys.argv[1]).resolve()
    with sqlite3.connect(":memory:") as db:
        db.execute("CREATE TABLE sync_entities (id TEXT, specifics BLOB)")
        try:
            verify_payload(db)
        except AssertionError:
            pass
        else:
            raise AssertionError("missing sentinel incorrectly passed verification")
    for backend in ("modernc", "mattn"):
        binary = work / backend
        directory = work / f"state-{backend}"
        directory.mkdir()
        probe_path = directory / "must-not-be-created.db"
        environment = os.environ.copy()
        environment["SQLITE_PATH"] = str(probe_path)
        version = subprocess.check_output([str(binary), "--version"], env=environment, text=True)
        expected_version = (Path(__file__).resolve().parent.parent / "main.go").read_text()
        assert f'version          = "{version.split()[1]}"' in expected_version, version
        assert f"sqlite: {backend}" in version, version
        assert not probe_path.exists(), "--version opened a database"
        subprocess.run(
            ["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
             "-keyout", str(directory / "key.pem"), "-out", str(directory / "cert.pem"),
             "-days", "1", "-subj", "/CN=localhost",
             "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1"],
            check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        for tls in (False, True):
            run_server(binary, directory, tls, signal.SIGINT, seed=not tls)
            run_server(binary, directory, tls, signal.SIGTERM)
        invalid_environment = environment.copy()
        invalid_environment.update(
            LISTEN_ADDR="127.0.0.1:0", TLS_CERT_FILE=str(directory / "cert.pem"), TLS_KEY_FILE=""
        )
        invalid = subprocess.run(
            [str(binary)], env=invalid_environment, capture_output=True, text=True, timeout=15
        )
        assert invalid.returncode != 0 and "TLS config invalid" in invalid.stderr
        print(f"PASS: {backend} version, HTTP/HTTPS aliases, SIGINT/SIGTERM, persistence, TLS validation")


if __name__ == "__main__":
    main()
