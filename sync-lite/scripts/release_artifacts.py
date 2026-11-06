"""Build and verify Linux amd64 handoffs from a clean committed checkout."""

import fcntl
import gzip
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

LITE = Path(__file__).resolve().parent.parent
ROOT = LITE.parent
GO = Path.home() / "sdk/go1.26.10/bin/go"
DOCS = (
    "README.md", "DEVELOPMENT_SETUP.md", "MANUAL_TESTING.md", "CHANGELOG.md",
    "RELEASE_NOTES_0.4.md", "LEGACY_TOKENS.md", "AUDIT_REMEDIATION.md",
    "AGENTS.md", "TODO.md", "FINISHED_TASKS.md", "MAINTENANCE.md",
)
LEGAL = re.compile(r"^(licen[cs]e|copying|notice|copyright|patents)([._-].*)?$", re.I)


def environment(cgo=False):
    result = {k: v for k, v in os.environ.items() if not k.startswith(("GO", "CGO_", "GIT_"))}
    result.update(
        GO=str(GO), GOENV="off", GOWORK="off", GOTOOLCHAIN="local",
        GOOS="linux", GOARCH="amd64", GOAMD64="v1", CGO_ENABLED="1" if cgo else "0",
        GOFLAGS="-mod=readonly", GOPROXY="off", CC="/usr/bin/gcc", CXX="/usr/bin/g++",
        PATH=f"{GO.parent}:{os.defpath}", PYTHONDONTWRITEBYTECODE="1",
        GIT_CONFIG_GLOBAL="/dev/null", GIT_CONFIG_NOSYSTEM="1", GIT_TERMINAL_PROMPT="0",
    )
    return result


def run(args, *, cwd=LITE, cgo=False, binary=False):
    return subprocess.check_output(
        [str(arg) for arg in args], cwd=cwd, env=environment(cgo),
        text=not binary, timeout=600,
    )


def sha(path):
    with path.open("rb") as stream:
        return stream_sha(stream)


def stream_sha(stream):
    digest = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        digest.update(chunk)
    return digest.hexdigest()


def regular(path):
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"Expected a regular, non-symlink file: {path.name}")
    return path


def clean_revision():
    if run(["git", "status", "--porcelain", "--untracked-files=all"], cwd=ROOT).strip():
        raise ValueError("Commit intended changes first; release builds require a clean checkout.")
    return run(["git", "rev-parse", "HEAD"], cwd=ROOT).strip()


def version():
    text = run(["git", "show", "HEAD:sync-lite/main.go"], cwd=ROOT)
    match = re.search(r'^\s*version\s*=\s*"(\d+\.\d+(?:\.\d+)?)"\s*$', text, re.M)
    if not match:
        raise ValueError("Cannot determine the service version")
    return match[1]


def source_snapshot(destination, revision):
    # Only committed objects enter the build, not ignored or index-hidden files.
    run(["git", "clone", "--shared", "--no-checkout", "--quiet", ROOT, destination], cwd=ROOT)
    run(["git", "checkout", "--detach", "--quiet", revision], cwd=destination)
    return destination


def safe_name(name):
    path = PurePosixPath(name)
    if not name or path.is_absolute() or ".." in path.parts or "\\" in name:
        raise ValueError("Unsafe archive member name")
    return path


def write_archive(path, entries):
    with path.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as archive:
                for name, content, mode in sorted(entries):
                    safe_name(name)
                    member = tarfile.TarInfo(name)
                    member.mode = mode
                    member.size = len(content)
                    archive.addfile(member, io.BytesIO(content))


def json_stream(text):
    decoder = json.JSONDecoder()
    while text.strip():
        text = text.lstrip()
        value, consumed = decoder.raw_decode(text)
        yield value
        text = text[consumed:]


def license_text(build_infos, source_lite):
    modules = {m["Path"]: m for m in json_stream(run([GO, "list", "-m", "-json", "all"], cwd=source_lite))}
    used = set()
    for info in build_infos:
        used.update(re.findall(r"^\s*dep\s+(\S+)\s+", info, re.M))
    sections = []
    for name in sorted(used - {"github.com/brave/go-sync"}):
        module = modules[name]
        if module.get("Replace"):
            raise ValueError(f"Unreviewed replacement module: {name}")
        directory = Path(module["Dir"])
        notices = []
        for base, dirs, files in os.walk(directory, followlinks=False):
            dirs[:] = sorted(d for d in dirs if d != ".git" and not (Path(base) / d).is_symlink())
            for filename in sorted(files):
                if LEGAL.match(filename):
                    file = regular(Path(base) / filename)
                    if file.stat().st_size > 2 * 1024 * 1024:
                        raise ValueError(f"Unexpectedly large notice in {name}")
                    notices.append(f"--- {file.relative_to(directory)} ---\n{file.read_text(errors='replace')}")
        if not notices:
            raise ValueError(f"No license notice found for linked module {name}")
        sections.append(f"## {name} {module['Version']}\n\n" + "\n".join(notices))
    goroot = Path(run([GO, "env", "GOROOT"]).strip())
    sections.append("## Go toolchain/runtime\n\n" + regular(goroot / "LICENSE").read_text())
    for filename in ("PATENTS",):
        if (goroot / filename).is_file():
            sections.append(regular(goroot / filename).read_text())
    vendor = goroot / "src/vendor"
    for file in sorted(vendor.rglob("*")):
        if file.is_file() and LEGAL.match(file.name):
            sections.append(f"## Go standard-library vendor notice: {file.relative_to(vendor)}\n\n" + regular(file).read_text(errors="replace"))
    return ("Third-party notices for the linked modules and Go runtime.\n"
            "Module source versions are recorded in BUILD-INFO.txt and go.sum.\n"
            "The fork/upstream MPL license is provided separately as LICENSE.\n\n"
            + "\n\n".join(sections)).encode()


def verify(directory, revision, release):
    manifest = json.loads(regular(directory / "release-manifest.json").read_text())
    if manifest["revision"] != revision or manifest["version"] != release:
        raise ValueError("Artifacts do not belong to the current committed source/version")
    expected = set(manifest["files"]) | {"release-manifest.json"}
    actual = set()
    for line in regular(directory / "SHA256SUMS").read_text().splitlines():
        digest, name = line.split("  ", 1)
        if safe_name(name).name != name or name not in expected or name in actual:
            raise ValueError("Unexpected/duplicate checksum entry")
        if sha(regular(directory / name)) != digest:
            raise ValueError(f"Checksum mismatch: {name}")
        actual.add(name)
    if actual != expected:
        raise ValueError("Incomplete checksum manifest")
    for backend in ("modernc", "mattn"):
        details = manifest["binaries"][backend]
        info = regular(directory / (details["file"] + ".build-info.txt")).read_text()
        if f"vcs.revision={revision}" not in info or "vcs.modified=false" not in info:
            raise ValueError("Binary was not built from the clean release commit")
        if "GOOS=linux" not in info or "GOARCH=amd64" not in info:
            raise ValueError("Unexpected binary target")
        if f"CGO_ENABLED={'1' if backend == 'mattn' else '0'}" not in info:
            raise ValueError("Wrong SQLite backend build configuration")
        with tarfile.open(regular(directory / details["archive"]), "r:gz") as archive:
            names = set()
            for member in archive.getmembers():
                safe_name(member.name)
                if not member.isfile() or member.name in names:
                    raise ValueError("Unexpected non-file/duplicate archive member")
                names.add(member.name)
            prefix = details["prefix"]
            for name in ("sync-lite", "LICENSE", "SOURCE.txt", "THIRD_PARTY_NOTICES.txt", "BUILD-INFO.txt", "README.md", "deploy/sync-lite.env.example", "deploy/sync-lite.service.example"):
                if prefix + name not in names:
                    raise ValueError(f"Missing packaged file: {name}")
            with archive.extractfile(prefix + "sync-lite") as stream:
                if stream_sha(stream) != sha(directory / details["file"]):
                    raise ValueError("Packaged binary differs from verified raw binary")
    with tarfile.open(regular(directory / manifest["source_archive"]), "r:gz") as source:
        names = set()
        for member in source.getmembers():
            safe_name(member.name)
            if not (member.isfile() or member.isdir()):
                raise ValueError("Unexpected source archive link or special file")
            names.add(member.name)
        prefix = f"go-sync-lite-{release}-source/"
        if not {prefix + "LICENSE", prefix + "sync-lite/main.go", prefix + "command/command.go"} <= names:
            raise ValueError("Source archive is missing the fork or reused parent sources")
    print(f"PASS: release artifacts, checksums and source identity ({release}, {revision[:12]})")


def build():
    revision, release = clean_revision(), version()
    if not GO.is_file():
        raise ValueError("Install the pinned Go 1.26.10 SDK under ~/sdk/ first")
    if (LITE / "dist").is_symlink():
        raise ValueError("Refusing a symlinked dist directory")
    directory = LITE / "dist"
    directory.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".release-", dir=directory) as temporary:
        stage = Path(temporary)
        source_root = source_snapshot(stage / ".source-checkout", revision)
        source_lite = source_root / "sync-lite"
        manifest = {"version": release, "revision": revision, "target": "linux/amd64", "binaries": {}}
        infos = []
        for backend in ("modernc", "mattn"):
            filename = "sync-lite-linux-amd64" + ("-cgo" if backend == "mattn" else "")
            command = [GO, "build", "-trimpath", "-o", stage / filename]
            if backend == "mattn":
                command += ["-tags", "sqlite_cgo"]
            run(command + ["."], cwd=source_lite, cgo=backend == "mattn")
            identity = run([stage / filename, "--version"]).strip()
            if f"sync-lite {release} (sqlite: {backend}; linux/amd64;" not in identity:
                raise ValueError("Wrong version/backend in built artifact")
            info = run([GO, "version", "-m", stage / filename])
            # Build metadata's first line otherwise contains the private staging path.
            info = filename + ": " + info.split(": ", 1)[1]
            (stage / (filename + ".build-info.txt")).write_text(info)
            (stage / (filename + ".sha256")).write_text(f"{sha(stage / filename)}  dist/{filename}\n")
            infos.append(info)
            native = run(["readelf", "-d", stage / filename])
            requirements = sorted(set(re.findall(r"GLIBC_[0-9.]+", run(["readelf", "--version-info", stage / filename]))))
            if backend == "modernc" and "(NEEDED)" in native:
                raise ValueError("Preferred modernc binary unexpectedly needs shared libraries")
            manifest["binaries"][backend] = {
                "file": filename, "archive": f"sync-lite-{release}-linux-amd64{'-cgo' if backend == 'mattn' else ''}.tar.gz",
                "prefix": f"sync-lite-{release}-linux-amd64{'-cgo' if backend == 'mattn' else ''}/",
                "glibc_versions": requirements,
            }
            (stage / (filename + ".runtime.txt")).write_text(identity + "\n\n" + native + "\nRequired symbol versions: " + ", ".join(requirements) + "\n")
        notices = license_text(infos, source_lite)
        source_note = (f"sync-lite {release}\nSource commit: {revision}\n"
                       f"https://github.com/luxzg/go-sync/tree/{revision}\n"
                       "Original upstream: https://github.com/brave/go-sync\n"
                       "See the accompanying source archive and LICENSE.\n").encode()
        for backend, details in manifest["binaries"].items():
            files = [(name, regular(source_lite / name).read_bytes(), 0o644) for name in DOCS]
            files += [(f"deploy/{name}", regular(source_lite / "deploy" / name).read_bytes(), 0o644)
                      for name in ("sync-lite.env.example", "sync-lite.service.example")]
            files += [
                ("sync-lite", (stage / details["file"]).read_bytes(), 0o755),
                ("LICENSE", regular(source_root / "LICENSE").read_bytes(), 0o644),
                ("SOURCE.txt", source_note, 0o644), ("THIRD_PARTY_NOTICES.txt", notices, 0o644),
                ("BUILD-INFO.txt", (stage / (details["file"] + ".build-info.txt")).read_bytes(), 0o644),
                ("RUNTIME.txt", (stage / (details["file"] + ".runtime.txt")).read_bytes(), 0o644),
            ]
            write_archive(stage / details["archive"], [(details["prefix"] + name, content, mode) for name, content, mode in files])
        manifest["source_archive"] = f"go-sync-lite-{release}-source.tar.gz"
        source_tar = run(["git", "archive", "--format=tar", f"--prefix=go-sync-lite-{release}-source/", revision], cwd=ROOT, binary=True)
        with (stage / manifest["source_archive"]).open("wb") as stream:
            with gzip.GzipFile(filename="", fileobj=stream, mode="wb", mtime=0) as compressed:
                compressed.write(source_tar)
        manifest["files"] = sorted(path.name for path in stage.iterdir() if path.is_file())
        (stage / "release-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
        (stage / "SHA256SUMS").write_text("".join(f"{sha(stage / name)}  {name}\n" for name in manifest["files"] + ["release-manifest.json"]))
        verify(stage, revision, release)
        # Run the existing isolated smoke against these exact packaged binaries.
        with tempfile.TemporaryDirectory(prefix="sync-lite-release-smoke-") as smoke:
            for backend, details in manifest["binaries"].items():
                with tarfile.open(stage / details["archive"], "r:gz") as archive:
                    with archive.extractfile(details["prefix"] + "sync-lite") as source:
                        target = Path(smoke) / backend
                        target.write_bytes(source.read())
                        target.chmod(0o700)
            print(run([sys.executable, source_lite / "testdata/smoke_server.py", smoke]), end="")
        if clean_revision() != revision:
            raise ValueError("Checkout changed while building; artifacts not published")
        for name in manifest["files"] + ["release-manifest.json", "SHA256SUMS"]:
            target = directory / name
            if target.is_symlink() or (target.exists() and not target.is_file()):
                raise ValueError(f"Refusing an unsafe output: {name}")
        # Publish the completion checksum file last; a partial set fails verify.
        for name in manifest["files"] + ["release-manifest.json", "SHA256SUMS"]:
            os.replace(stage / name, directory / name)
    verify(directory, revision, release)


def main():
    if len(sys.argv) == 1:
        print("Usage: ./scripts/release-artifacts.sh {build|verify|test}")
        print("Clean committed source; Go 1.26.10 from ~/sdk/; Linux amd64 only.")
        print("Writes ignored dist/ and owned temporary fixtures; never deploys or uploads.")
        return
    if len(sys.argv) != 2 or sys.argv[1] not in {"build", "verify", "test"}:
        raise ValueError("Expected one fixed mode: build, verify or test")
    if sys.argv[1] == "test":
        subprocess.run([sys.executable, "-B", "-m", "unittest", "discover", "-s", str(LITE / "testdata"), "-p", "test_release_artifacts.py"], check=True, env=environment())
        return
    if (LITE / "dist").is_symlink():
        raise ValueError("Refusing a symlinked dist directory")
    if sys.argv[1] == "verify":
        verify(LITE / "dist", clean_revision(), version())
        return
    (LITE / "dist").mkdir(exist_ok=True)
    lock = LITE / "dist/.release.lock"
    descriptor = os.open(lock, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "w") as locked:
        fcntl.flock(locked, fcntl.LOCK_EX | fcntl.LOCK_NB)
        build()


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        sys.exit(f"Release preparation failed: {error}")
