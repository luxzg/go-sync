package main

import (
	"context"
	"fmt"
	"github.com/brave/go-sync/datastore"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"testing"
)

func TestAuditA05ExpiredHistoryRecommit(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	e := makeCommitEntity("history", &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_History{History: &sync_pb.HistorySpecifics{}}})
	e.ClientTagHash = stringPtr("synthetic-history-tag")
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", e)[0])
	if _, err := h.db.db.Exec(`UPDATE sync_entities SET expiration_time=1 WHERE client_id=? AND id=?`, chain, e.GetClientTagHash()); err != nil {
		t.Fatal(err)
	}
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", e)[0])
	_, entries, err := h.db.GetUpdatesForType(context.Background(), datastore.HistoryTypeID, 0, true, chain, 500)
	if err != nil || len(entries) != 1 {
		t.Fatalf("acknowledged History remained expired: %v %v", entries, err)
	}
}

func TestAuditA05PhysicalRetirement(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := sqliteTestContext(t)
	for _, chain := range []string{"a", "b"} {
		for _, dt := range []int{datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID, sqliteTestBookmarkType} {
			e := sqliteTestEntity(chain, fmt.Sprint(dt), 100)
			e.DataType = &dt
			e.ExpirationTime = int64Ptr(1)
			sqliteTestInsert(t, h.db, e)
		}
	}
	for _, dt := range []int{datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID} {
		_, _, err := h.db.GetUpdatesForType(ctx, dt, 0, true, "a", 500)
		if err != nil {
			t.Fatal(err)
		}
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", "a", 1)
	sqliteTestRowCount(t, h.db.db, "sync_entities", "b", 3)
}

func TestAuditA05BoundariesAndBoundedWork(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := sqliteTestContext(t)
	tx, err := h.db.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < expirationBatchSize+1; i++ {
		e := sqliteTestEntity("a", fmt.Sprint(i), int64(i+1))
		e.DataType = sqliteTestPtr(datastore.HistoryTypeID)
		e.ExpirationTime = int64Ptr(99)
		if err := insertSyncEntity(ctx, tx, &e); err != nil {
			t.Fatal(err)
		}
	}
	for i, expiry := range []int64{0, 100, 101} {
		e := sqliteTestEntity("a", fmt.Sprintf("live-%d", i), 1000)
		e.DataType = sqliteTestPtr(datastore.HistoryTypeID)
		e.ExpirationTime = &expiry
		if err := insertSyncEntity(ctx, tx, &e); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := h.db.retireExpiredAt(ctx, "a", nil, nil, 100); err != nil {
		t.Fatal(err)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", "a", 4)
	if err := h.db.retireExpiredAt(ctx, "a", nil, nil, 100); err != nil {
		t.Fatal(err)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", "a", 3)
	before := sqliteTestQuery(t, h.db.db, `PRAGMA data_version`)
	if err := h.db.retireExpiredAt(ctx, "a", nil, nil, 100); err != nil {
		t.Fatal(err)
	}
	after := sqliteTestQuery(t, h.db.db, `PRAGMA data_version`)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatal("no-expiry read unexpectedly changed database")
	}
}

func TestAuditA05ExpiredDirectiveTagReuse(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	e := sqliteTestEntity("a", "old", 100)
	e.DataType = sqliteTestPtr(datastore.HistoryDeleteDirectiveTypeID)
	e.ExpirationTime = int64Ptr(1)
	sqliteTestInsert(t, h.db, e)
	e.ID = "new"
	e.ExpirationTime = int64Ptr(4102444800)
	sqliteTestInsert(t, h.db, e)
	sqliteTestRowCount(t, h.db.db, "sync_entities", "a", 1)
	sqliteTestRowCount(t, h.db.db, "unique_tags", "a", 1)
}

func TestAuditA05LegacyRetirementPreservesClock(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := sqliteTestContext(t)
	e := sqliteTestEntity("a", "old", 9000)
	e.DataType = sqliteTestPtr(datastore.HistoryTypeID)
	e.ExpirationTime = int64Ptr(1)
	tx, err := h.db.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := insertSyncEntity(ctx, tx, &e); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if exists, err := h.db.HasItem(ctx, "a", "old"); exists || err != nil {
		t.Fatal(exists, err)
	}
	e.ID = "new"
	e.Mtime = int64Ptr(100)
	e.ExpirationTime = int64Ptr(4102444800)
	sqliteTestInsert(t, h.db, e)
	_, entries, err := h.db.GetUpdatesForType(ctx, datastore.HistoryTypeID, 9000, true, "a", 500)
	if err != nil || len(entries) != 1 {
		t.Fatal("expiry lost legacy watermark", entries, err)
	}
}

func TestAuditA05NullableExpiryPreservesLiveOwner(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := sqliteTestContext(t)
	old := sqliteTestEntity("a", "expired-tombstone", 100)
	old.DataType = sqliteTestPtr(datastore.HistoryDeleteDirectiveTypeID)
	old.ExpirationTime = int64Ptr(1)
	old.Deleted = boolPtr(true)
	live := old
	live.ID = "live-owner"
	live.ExpirationTime = nil
	live.Deleted = boolPtr(false)
	tx, err := h.db.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, e := range []datastore.SyncEntity{old, live} {
		if err := insertSyncEntity(ctx, tx, &e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO unique_tags (client_id,id,mtime,ctime) VALUES ('a',?,1,1)`, clientTagItemPrefix+*live.ClientDefinedUniqueTag); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := h.db.retireExpiredAt(ctx, "a", nil, nil, 100); err != nil {
		t.Fatal(err)
	}
	sqliteTestRowCount(t, h.db.db, "unique_tags", "a", 1)
	sqliteTestRowCount(t, h.db.db, "sync_entities", "a", 1)
}
