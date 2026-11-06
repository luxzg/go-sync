package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestAuditA01CommitOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.db")
	s := sqliteTestOpen(t, path)
	ctx := sqliteTestContext(t)
	earlier := sqliteTestEntity("chain", "prepared-first", 100)
	later := sqliteTestEntity("chain", "committed-first", 101)
	sqliteTestInsert(t, s, later)
	_, first, err := s.GetUpdatesForType(ctx, sqliteTestBookmarkType, 0, true, "chain", 500)
	if err != nil {
		t.Fatal(err)
	}
	token := *first[0].Mtime
	sqliteTestInsert(t, s, earlier)
	_, next, err := s.GetUpdatesForType(ctx, sqliteTestBookmarkType, token, true, "chain", 500)
	if err != nil || len(next) != 1 || next[0].ID != earlier.ID {
		t.Fatalf("late commit missed: %v %v", next, err)
	}
	if *next[0].Ctime != *earlier.Ctime {
		t.Fatal("clock altered creation time")
	}
	token = *next[0].Mtime
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = sqliteTestOpen(t, path)
	e := sqliteTestEntity("chain", "after-restart", 99)
	sqliteTestInsert(t, s, e)
	_, next, err = s.GetUpdatesForType(ctx, sqliteTestBookmarkType, token, true, "chain", 500)
	if err != nil || len(next) != 1 {
		t.Fatalf("clock regressed across reopen: %v %v", next, err)
	}
}

func TestAuditA01FrozenClockPagination(t *testing.T) {
	for _, count := range []int{499, 500, 501} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "pages.db"))
			ctx := sqliteTestContext(t)
			for i := 0; i < count; i++ {
				e := sqliteTestEntity("chain", fmt.Sprint(i), 100)
				if i%3 == 0 {
					e.Deleted = sqliteTestPtr(true)
				}
				sqliteTestInsert(t, s, e)
			}
			token := int64(0)
			seen := map[string]bool{}
			for pages := 0; pages < 3; pages++ {
				more, entries, err := s.GetUpdatesForType(ctx, sqliteTestBookmarkType, token, true, "chain", 500)
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range entries {
					if seen[e.ID] || *e.Mtime <= token {
						t.Fatal("duplicate or non-advancing timestamp")
					}
					seen[e.ID] = true
					token = *e.Mtime
				}
				if !more {
					break
				}
			}
			if len(seen) != count {
				t.Fatalf("got %d of %d identities", len(seen), count)
			}
			// Existing rows, not only live entities, define the allocation floor.
			e := sqliteTestEntity("chain", "new", 1)
			e.Version = sqliteTestPtr(int64(1))
			sqliteTestInsert(t, s, e)
		})
	}
}

func TestAuditA01RollbackAndSchema(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "rollback.db"))
	ctx := sqliteTestContext(t)
	columns := sqliteTestQuery(t, s.db, `PRAGMA table_info(sync_clocks)`)
	if len(columns) != 3 || columns[0][1] != "client_id" || columns[1][1] != "data_type" || columns[2][1] != "mtime" {
		t.Fatal("unexpected additive clock schema")
	}
	original := sqliteTestEntity("chain", "original", 100)
	sqliteTestInsert(t, s, original)
	failed := sqliteTestEntity("chain", "original", 10000)
	if conflict, err := s.InsertSyncEntity(ctx, &failed); !conflict || err == nil {
		t.Fatal("expected rollback")
	}
	var stamp int64
	if err := s.db.QueryRow(`SELECT mtime FROM sync_clocks WHERE client_id='chain'`).Scan(&stamp); err != nil || stamp != 100 {
		t.Fatalf("failed write advanced clock: %d %v", stamp, err)
	}
	update := original
	update.ServerDefinedUniqueTag = nil
	update.Version = int64Ptr(99)
	update.Mtime = update.Version
	update.Ctime = update.Mtime
	if conflict, _, err := s.UpdateSyncEntity(ctx, &update, 1); conflict || err != nil {
		t.Fatal(conflict, err)
	}
	if *update.Ctime != 99 || *update.Mtime != *update.Version || *update.Mtime != 101 {
		t.Fatal("version/timestamp aliasing changed creation time")
	}
}

func TestAuditA01IndependentHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writers.db")
	a := sqliteTestOpen(t, path)
	b := sqliteTestOpen(t, path)
	ctx := sqliteTestContext(t)
	result := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			s := a
			if i%2 != 0 {
				s = b
			}
			e := sqliteTestEntity("chain", fmt.Sprint(i), 100)
			e.ServerDefinedUniqueTag = nil
			e.Version = int64Ptr(100)
			conflict, err := s.InsertSyncEntity(ctx, &e)
			if conflict {
				err = fmt.Errorf("unexpected conflict")
			}
			result <- err
		}(i)
	}
	for i := 0; i < 8; i++ {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	_, entities, err := a.GetUpdatesForType(ctx, sqliteTestBookmarkType, 0, true, "chain", 100)
	if err != nil || len(entities) != 8 {
		t.Fatal(len(entities), err)
	}
	var previous int64
	for _, e := range entities {
		if *e.Mtime <= previous || *e.Version != *e.Mtime {
			t.Fatal("non-monotonic concurrent writes")
		}
		previous = *e.Mtime
	}
}
