package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/datastore"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

// These are characterization tests, not claims of historical recovery. Seed raw
// synthetic pre-0.4 timestamps so the ordered-write allocator cannot hide ties.
func remediationSeedLegacy(t *testing.T, s *SQLiteStore, entries ...datastore.SyncEntity) {
	t.Helper()
	ctx := sqliteTestContext(t)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := range entries {
		if err := insertSyncEntity(ctx, tx, &entries[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func remediationLegacyEntity(t *testing.T, chain, id string, stamp int64) datastore.SyncEntity {
	t.Helper()
	e := sqliteTestEntity(chain, id, stamp)
	e.ServerDefinedUniqueTag = nil
	e.Specifics = mustMarshal(t, makeBookmarkSpecifics())
	e.UniquePosition = mustMarshal(t, &sync_pb.UniquePosition{Value: []byte("synthetic-position")})
	return e
}

func remediationPoll(t *testing.T, h http.Handler, token string, stamp int64) *sync_pb.GetUpdatesResponse {
	t.Helper()
	contents := sync_pb.ClientToServerMessage_GET_UPDATES
	rsp := doAuthedRequest(t, h, token, mustMarshal(t, &sync_pb.ClientToServerMessage{
		MessageContents: &contents, Share: stringPtr(""),
		GetUpdates: &sync_pb.GetUpdatesMessage{
			FetchFolders: boolPtr(true), GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_GU_TRIGGER),
			FromProgressMarker: markers([]int32{testBookmarkTypeID}, []int64{stamp}),
		},
	}), false)
	if rsp.GetErrorCode() != sync_pb.SyncEnums_SUCCESS || rsp.GetUpdates == nil {
		t.Fatal("unexpected GetUpdates response", rsp)
	}
	return rsp.GetUpdates
}

func remediationMarker(t *testing.T, rsp *sync_pb.GetUpdatesResponse) int64 {
	t.Helper()
	if len(rsp.NewProgressMarker) != 1 || rsp.NewProgressMarker[0].GetDataTypeId() != testBookmarkTypeID {
		t.Fatal("expected one bookmark marker", rsp.NewProgressMarker)
	}
	stamp, n := binary.Varint(rsp.NewProgressMarker[0].Token)
	if n <= 0 {
		t.Fatal("invalid scalar token")
	}
	return stamp
}

func remediationExpectedLegacy(e datastore.SyncEntity) *sync_pb.SyncEntity {
	return &sync_pb.SyncEntity{
		IdString: stringPtr(e.ID), ParentIdString: e.ParentID,
		Version: e.Version, Mtime: e.Mtime, Ctime: e.Ctime,
		Name: e.Name, NonUniqueName: e.NonUniqueName, Deleted: e.Deleted,
		Folder: e.Folder, ServerDefinedUniqueTag: e.ServerDefinedUniqueTag,
		OriginatorCacheGuid: e.OriginatorCacheGUID, OriginatorClientItemId: e.OriginatorClientItemID,
		ClientTagHash: e.ClientDefinedUniqueTag, Specifics: makeBookmarkSpecifics(),
		UniquePosition: &sync_pb.UniquePosition{Value: []byte("synthetic-position")},
	}
}

func remediationRequireLegacyPage(t *testing.T, rsp *sync_pb.GetUpdatesResponse, expected map[string]*sync_pb.SyncEntity, count int) {
	t.Helper()
	if len(rsp.Entries) != count {
		t.Fatalf("legacy page has %d entries, want %d", len(rsp.Entries), count)
	}
	seen := make(map[string]bool)
	for _, e := range rsp.Entries {
		want, ok := expected[e.GetIdString()]
		if !ok || seen[e.GetIdString()] {
			t.Fatal("unknown/duplicate identity in legacy page")
		}
		seen[e.GetIdString()] = true
		if !proto.Equal(e, want) {
			t.Fatal("legacy download changed metadata, deletion/folder state, specifics or position")
		}
	}
}

func TestRemediationLegacyTokenTiedPageIsNotRepaired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-ties.db")
	s := sqliteTestOpen(t, path)
	token, chain := mustToken(t)
	entries := make([]datastore.SyncEntity, 501)
	expected := make(map[string]*sync_pb.SyncEntity, len(entries))
	for i := range entries {
		entries[i] = remediationLegacyEntity(t, chain, fmt.Sprintf("legacy-%03d", i), 100)
		if i%3 == 0 {
			entries[i].Deleted = boolPtr(true)
		}
		if i%5 == 0 {
			entries[i].Folder = boolPtr(true)
		}
		expected[entries[i].ID] = remediationExpectedLegacy(entries[i])
	}
	remediationSeedLegacy(t, s, entries...)
	before := sqliteTestChainSnapshot(t, s.db, chain)
	clocksBefore := sqliteTestQuery(t, s.db, `SELECT * FROM sync_clocks`)
	h := commandHandler(cache.NewCache(NewMemoryRedis()), s)
	first := remediationPoll(t, h, token, 0)
	if len(first.Entries) != 500 || first.GetChangesRemaining() != 1 || remediationMarker(t, first) != 100 {
		t.Fatalf("legacy first page count=%d remaining=%d", len(first.Entries), first.GetChangesRemaining())
	}
	remediationRequireLegacyPage(t, first, expected, 500)
	second := remediationPoll(t, h, token, remediationMarker(t, first))
	if len(second.Entries) != 0 || second.GetChangesRemaining() != 0 || remediationMarker(t, second) != 100 {
		t.Fatal("legacy scalar marker behavior changed; revisit the token strategy", second)
	}
	// All 501 still exist: the missing delivery is not database row loss.
	sqliteTestRowCount(t, s.db, "sync_entities", chain, 501)
	if !reflect.DeepEqual(before, sqliteTestChainSnapshot(t, s.db, chain)) || !reflect.DeepEqual(clocksBefore, sqliteTestQuery(t, s.db, `SELECT * FROM sync_clocks`)) {
		t.Fatal("ordinary legacy reads silently rewrote records or clocks")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = sqliteTestOpen(t, path)
	if !reflect.DeepEqual(before, sqliteTestChainSnapshot(t, s.db, chain)) || !reflect.DeepEqual(clocksBefore, sqliteTestQuery(t, s.db, `SELECT * FROM sync_clocks`)) {
		t.Fatal("restart silently changed legacy records or clocks")
	}
	// Restart and a token-zero scan are not a fix for a >500 tied cohort.
	h = commandHandler(cache.NewCache(NewMemoryRedis()), s)
	repeated := remediationPoll(t, h, token, 0)
	if len(repeated.Entries) != 500 || remediationMarker(t, repeated) != 100 {
		t.Fatal("unexpected restarted legacy scan", len(repeated.Entries))
	}
	remediationRequireLegacyPage(t, repeated, expected, 500)
	if !reflect.DeepEqual(before, sqliteTestChainSnapshot(t, s.db, chain)) || !reflect.DeepEqual(clocksBefore, sqliteTestQuery(t, s.db, `SELECT * FROM sync_clocks`)) {
		t.Fatal("restarted scan silently changed legacy records or clocks")
	}
	sqliteTestIntegrity(t, s.db)
}

func TestRemediationLegacyTokenSkippedUpdateAndNewWrite(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "old-token.db"))
	token, chain := mustToken(t)
	// A client already holds 200. An old server committed this 100 timestamp
	// after issuing 200; current state alone cannot identify that client's miss.
	old := []datastore.SyncEntity{
		remediationLegacyEntity(t, chain, "late-old-commit", 100),
		remediationLegacyEntity(t, chain, "previously-delivered", 200),
	}
	remediationSeedLegacy(t, s, old...)
	h := commandHandler(cache.NewCache(NewMemoryRedis()), s)
	before := sqliteTestChainSnapshot(t, s.db, chain)
	if rsp := remediationPoll(t, h, token, 200); len(rsp.Entries) != 0 || remediationMarker(t, rsp) != 200 {
		t.Fatal("old token was silently rewound", rsp)
	}
	if !reflect.DeepEqual(before, sqliteTestChainSnapshot(t, s.db, chain)) {
		t.Fatal("read attempted historical repair")
	}
	// Even a backwards proposed wall clock is clamped above all existing rows.
	fresh := remediationLegacyEntity(t, chain, "new-ordered-write", 1)
	fresh.Version = fresh.Mtime
	if conflict, err := s.InsertSyncEntity(sqliteTestContext(t), &fresh); conflict || err != nil {
		t.Fatal("new write failed", conflict, err)
	}
	rsp := remediationPoll(t, h, token, 200)
	if len(rsp.Entries) != 1 || rsp.Entries[0].GetIdString() != fresh.ID || remediationMarker(t, rsp) <= 200 {
		t.Fatal("new write was not discoverable above the old token", rsp)
	}
	expected := map[string]*sync_pb.SyncEntity{fresh.ID: remediationExpectedLegacy(fresh)}
	remediationRequireLegacyPage(t, rsp, expected, 1)
	for _, e := range old {
		expected[e.ID] = remediationExpectedLegacy(e)
	}
	all := remediationPoll(t, h, token, 0)
	remediationRequireLegacyPage(t, all, expected, 3)
	if !reflect.DeepEqual(before["sync_entities"], sqliteTestQuery(t, s.db, `SELECT * FROM sync_entities WHERE client_id=? AND id IN ('late-old-commit','previously-delivered') ORDER BY id`, chain)) {
		t.Fatal("new ordered write or download rewrote original legacy rows")
	}
}
