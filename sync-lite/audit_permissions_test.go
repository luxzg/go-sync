package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditA11FreshDatabasePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.db")
	s := sqliteTestOpen(t, path)
	sqliteTestInsert(t, s, sqliteTestEntity("chain", "record", 100))
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("fresh database%s permissions=%#o", suffix, info.Mode().Perm())
		}
	}
}

func TestAuditA11ExistingModePreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	_ = sqliteTestOpen(t, path)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("existing operator mode changed", info, err)
	}
}
