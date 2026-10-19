package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scripts/test-upgrade.sh copies this test into the archived v0.2 sources and
// runs each compiled test binary against the same disposable SQLite file.
func TestUpgradeFixture(t *testing.T) {
	dir := os.Getenv("SYNC_LITE_UPGRADE_TEST_DIR")
	if dir == "" {
		t.Skip("run scripts/test-upgrade.sh for the cross-version/driver matrix")
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".synthetic-upgrade-test"))
	if err != nil || string(marker) != "sync-lite disposable upgrade fixture\n" {
		t.Fatal("refusing a directory not owned by the synthetic upgrade helper")
	}
	stage := os.Getenv("SYNC_LITE_UPGRADE_STAGE")
	switch stage {
	case "seed", "modernc", "mattn", "old-version":
	default:
		t.Fatalf("unknown upgrade stage %q", stage)
	}
	path := filepath.Join(dir, "fixture.db")
	if stage == "seed" {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("refusing to overwrite an existing fixture")
		}
	}
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var engine string
	if err := s.db.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&engine); err != nil {
		t.Fatal(err)
	}
	t.Logf("upgrade stage %s: SQLite %s", stage, engine)
	var synchronous int
	if err := s.db.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&synchronous); err != nil || synchronous != 2 {
		t.Fatalf("synchronous=%d, want FULL(2): %v", synchronous, err)
	}
	expectedPath := filepath.Join(dir, "expected.json")
	if stage == "seed" {
		fixture, err := os.ReadFile("testdata/v0.2.sql")
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, line := range strings.Split(string(fixture), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		// Use v0.2's real initializer, then add only the synthetic fixture rows.
		for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
			if strings.HasPrefix(strings.TrimSpace(statement), "INSERT ") {
				if _, err := s.db.ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
		}
	} else {
		expected, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := upgradeSnapshot(t, ctx, s.db); !bytes.Equal(got, expected) {
			for i := 0; i < len(got) && i < len(expected); i++ {
				if got[i] != expected[i] {
					t.Fatalf("synthetic snapshot differs at byte %d: got %q, expected %q", i,
						got[max(0, i-30):min(len(got), i+70)], expected[max(0, i-30):min(len(expected), i+70)])
				}
			}
			t.Fatalf("synthetic snapshot length changed: got %d, expected %d", len(got), len(expected))
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO sync_entities
			(client_id, id, version, mtime, deleted, specifics, data_type, folder)
			VALUES ('upgrade-writes', ?, 1, 1000, 0, ?, 32904, 0)`, stage,
			[]byte{0, 0xff, 0x80, 0, 0xa5}); err != nil {
			t.Fatalf("write with %s: %v", stage, err)
		}
	}
	var integrity string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check=%q: %v", integrity, err)
	}
	if err := os.WriteFile(expectedPath, upgradeSnapshot(t, ctx, s.db), 0600); err != nil {
		t.Fatal(err)
	}
	if stage == "seed" && os.Getenv("SYNC_LITE_UPGRADE_LEAVE_WAL") == "1" {
		// The helper invokes the test executable directly. Exit this owned child
		// without closing SQLite to exercise recovery of committed v0.2 WAL data.
		os.Exit(0)
	}
	if err := s.CheckpointTruncate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func upgradeSnapshot(t *testing.T, ctx context.Context, db *sql.DB) []byte {
	t.Helper()
	queries := []string{
		`SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`,
		`SELECT * FROM sync_entities ORDER BY client_id, id`,
		`SELECT * FROM unique_tags ORDER BY client_id, id`,
		`SELECT * FROM client_item_counts ORDER BY client_id`,
	}
	all := make([][][]any, 0, len(queries))
	for _, query := range queries {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		var table [][]any
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			// A zero-length BLOB may be a nil or empty []byte depending on the
			// driver. Keep its SQL type distinct from NULL and compare exact bytes.
			for i, value := range values {
				if blob, ok := value.([]byte); ok {
					values[i] = map[string]string{"blob": hex.EncodeToString(blob)}
				}
			}
			table = append(table, values)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, table)
	}
	out, err := json.Marshal(all)
	if err != nil {
		t.Fatal(fmt.Errorf("marshal synthetic database snapshot: %w", err))
	}
	return out
}
