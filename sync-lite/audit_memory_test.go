package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestAuditA15MemoryConnectionRetained(t *testing.T) {
	s := sqliteTestOpen(t, ":memory:")
	sqliteTestInsert(t, s, sqliteTestEntity("a", "keep", 100))
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var n int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities`).Scan(&n)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("pool opened a separate empty database instead of waiting", err)
	}
	_ = conn.Close()
	exists, err := s.HasItem(context.Background(), "a", "keep")
	if err != nil || !exists {
		t.Fatal("retained memory database lost state", exists, err)
	}
	other := sqliteTestOpen(t, ":memory:")
	exists, err = other.HasItem(context.Background(), "a", "keep")
	if err != nil || exists {
		t.Fatal("independent stores shared private state", exists, err)
	}
}

func TestAuditA15ConcurrentMemoryWrites(t *testing.T) {
	s := sqliteTestOpen(t, ":memory:")
	result := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			e := sqliteTestEntity("a", fmt.Sprint(i), 100)
			conflict, err := s.InsertSyncEntity(context.Background(), &e)
			if conflict {
				err = fmt.Errorf("unexpected conflict")
			}
			result <- err
		}(i)
	}
	var firstErr error
	for i := 0; i < 8; i++ {
		if err := <-result; err != nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	sqliteTestRowCount(t, s.db, "sync_entities", "a", 8)
}
