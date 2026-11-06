"""Synthetic packaging regressions; never builds, deploys or reads user state."""

import importlib.util
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "release_artifacts", Path(__file__).resolve().parent.parent / "scripts/release_artifacts.py"
)
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ReleaseArtifactTests(unittest.TestCase):
    def test_build_snapshot_excludes_ignored_and_index_hidden_changes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repository, snapshot = root / "repository", root / "snapshot"
            repository.mkdir()
            release.run(["git", "init", "--quiet", repository], cwd=root)
            (repository / ".gitignore").write_text("local_only.go\n")
            original = b"package main\n// committed synthetic source\n"
            (repository / "main.go").write_bytes(original)
            release.run(["git", "add", "-A"], cwd=repository)
            release.run(["git", "-c", "user.name=Synthetic Test", "-c", "user.email=test@example.invalid", "-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "synthetic fixture"], cwd=repository)
            revision = release.run(["git", "rev-parse", "HEAD"], cwd=repository).strip()
            (repository / "local_only.go").write_text("package main\n// ignored input\n")
            release.run(["git", "update-index", "--skip-worktree", "main.go"], cwd=repository)
            (repository / "main.go").write_text("package main\n// hidden local change\n")
            self.assertEqual(release.run(["git", "status", "--porcelain"], cwd=repository), "")
            with patch.object(release, "ROOT", repository):
                release.source_snapshot(snapshot, revision)
            self.assertEqual((snapshot / "main.go").read_bytes(), original)
            self.assertFalse((snapshot / "local_only.go").exists())

    def test_unsafe_paths(self):
        for name in ("", "/absolute", "../escape", "prefix/../../escape", "prefix\\escape"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                release.safe_name(name)

    def test_regular_file_rejects_links_and_directories(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            file = root / "regular"
            file.write_bytes(b"synthetic")
            self.assertEqual(release.regular(file), file)
            link = root / "link"
            link.symlink_to(file)
            for path in (link, root, root / "absent"):
                with self.subTest(path=path.name), self.assertRaises(ValueError):
                    release.regular(path)

    def test_archive_bytes_modes_and_no_owner_information(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            entries = [("package/sync-lite", b"synthetic-binary\x00\xff", 0o755),
                       ("package/README.md", b"synthetic documentation", 0o644)]
            release.write_archive(root / "first.tar.gz", entries)
            release.write_archive(root / "second.tar.gz", list(reversed(entries)))
            self.assertEqual(release.sha(root / "first.tar.gz"), release.sha(root / "second.tar.gz"))
            with tarfile.open(root / "first.tar.gz", "r:gz") as archive:
                self.assertEqual(len(archive.getmembers()), 2)
                for name, content, mode in entries:
                    member = archive.getmember(name)
                    self.assertEqual((member.uid, member.gid, member.uname, member.gname, member.mtime), (0, 0, "", "", 0))
                    self.assertEqual(member.mode, mode)
                    with archive.extractfile(member) as stream:
                        self.assertEqual(stream.read(), content)

    def test_json_stream(self):
        self.assertEqual(list(release.json_stream(' {"Path":"first"}\n{"Path":"second"}\n')), [{"Path": "first"}, {"Path": "second"}])

    def test_dirty_checkout_rejected(self):
        with patch.object(release, "run", return_value=" M sync-lite/README.md\n"):
            with self.assertRaisesRegex(ValueError, "clean checkout"):
                release.clean_revision()

    def test_wrong_revision_and_checksum_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest = root / "release-manifest.json"
            manifest.write_text('{"revision":"synthetic-commit","version":"0.4","files":["binary"]}')
            with self.assertRaisesRegex(ValueError, "current committed"):
                release.verify(root, "another-commit", "0.4")
            (root / "binary").write_bytes(b"synthetic")
            (root / "SHA256SUMS").write_text("0" * 64 + "  binary\n")
            with self.assertRaisesRegex(ValueError, "Checksum mismatch"):
                release.verify(root, "synthetic-commit", "0.4")


if __name__ == "__main__":
    unittest.main()
