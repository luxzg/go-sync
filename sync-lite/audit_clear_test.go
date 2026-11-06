package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestAuditA10ClearMetadataOnly(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	e := sqliteTestEntity("a", "opaque", 100)
	e.Specifics = make([]byte, 65536)
	sqliteTestInsert(t, h.db, e)
	records, err := h.db.ClearServerData(context.Background(), "a")
	if err != nil || len(records) != 1 {
		t.Fatal(len(records), err)
	}
	if records[0].ID != e.ID || records[0].ClientID != e.ClientID || records[0].DataType == nil || *records[0].DataType != *e.DataType {
		t.Fatal("lost invalidation metadata")
	}
	if len(records[0].Specifics) != 0 || len(records[0].UniquePosition) != 0 {
		t.Fatal("clear unnecessarily materialized opaque payload")
	}
}

func BenchmarkAuditA10Clear(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s, err := NewSQLiteStore(filepath.Join(b.TempDir(), "clear.db"))
		if err != nil {
			b.Fatal(err)
		}
		ctx := context.Background()
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		payload := make([]byte, 65536)
		for j := 0; j < 100; j++ {
			if _, err := tx.Exec(`INSERT INTO sync_entities (client_id,id,data_type,specifics) VALUES ('bench',?,32904,?)`, fmt.Sprint(j), payload); err != nil {
				_ = tx.Rollback()
				_ = s.Close()
				b.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			_ = s.Close()
			b.Fatal(err)
		}
		b.StartTimer()
		records, err := s.ClearServerData(ctx, "bench")
		b.StopTimer()
		_ = s.Close()
		if err != nil || len(records) != 100 {
			b.Fatal(len(records), err)
		}
	}
}
