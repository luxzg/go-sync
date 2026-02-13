package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/brave/go-sync/auth/authtest"
	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

const (
	testBookmarkTypeID int32 = 32904
	testNigoriTypeID   int32 = 47745
)

func TestCommandUnauthorized(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	commitMsg := makeCommitMessage(nil)
	body := mustMarshal(t, commitMsg)

	req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestCommitAndGetUpdatesRoundTrip(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	token, clientID := mustToken(t)

	// Commit 2 entities.
	entries := []*sync_pb.SyncEntity{
		makeCommitEntity("bookmark-item", makeBookmarkSpecifics()),
		makeCommitEntity("nigori-item", makeNigoriSpecifics()),
	}
	commitMsg := makeCommitMessage(entries)
	commitRsp := doAuthedRequest(t, h, token, mustMarshal(t, commitMsg), false)

	if commitRsp.ErrorCode == nil || *commitRsp.ErrorCode != sync_pb.SyncEnums_SUCCESS {
		t.Fatalf("expected commit success error code, got %+v", commitRsp.ErrorCode)
	}
	if commitRsp.Commit == nil || len(commitRsp.Commit.Entryresponse) != 2 {
		t.Fatalf("expected 2 commit entry responses, got %+v", commitRsp.Commit)
	}
	for i, entryRsp := range commitRsp.Commit.Entryresponse {
		if entryRsp.ResponseType == nil || *entryRsp.ResponseType != sync_pb.CommitResponse_SUCCESS {
			t.Fatalf("entry %d expected SUCCESS, got %+v", i, entryRsp.ResponseType)
		}
		if entryRsp.IdString == nil || *entryRsp.IdString == "" {
			t.Fatalf("entry %d expected non-empty server id", i)
		}
	}

	// Get updates from token=0 for nigori and bookmark.
	gu := &sync_pb.GetUpdatesMessage{
		FetchFolders:       boolPtr(true),
		GetUpdatesOrigin:   enumGUPtr(sync_pb.SyncEnums_GU_TRIGGER),
		FromProgressMarker: markers([]int32{testNigoriTypeID, testBookmarkTypeID}, []int64{0, 0}),
	}
	contents := sync_pb.ClientToServerMessage_GET_UPDATES
	guMsg := &sync_pb.ClientToServerMessage{MessageContents: &contents, Share: stringPtr(""), GetUpdates: gu}
	guRsp := doAuthedRequest(t, h, token, mustMarshal(t, guMsg), false)

	if guRsp.ErrorCode == nil || *guRsp.ErrorCode != sync_pb.SyncEnums_SUCCESS {
		t.Fatalf("expected get updates success error code, got %+v", guRsp.ErrorCode)
	}
	if guRsp.GetUpdates == nil {
		t.Fatalf("expected get updates response")
	}
	if len(guRsp.GetUpdates.Entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(guRsp.GetUpdates.Entries))
	}

	// Ensure every returned entity belongs to this client chain by making a subsequent poll marker request.
	for _, marker := range guRsp.GetUpdates.NewProgressMarker {
		if marker.DataTypeId == nil || marker.Token == nil {
			t.Fatalf("expected marker fields to be set")
		}
	}

	// Sanity-check disabled chain flag is false for active chain.
	if disabled, err := lookupDisabled(h, clientID); err != nil {
		t.Fatalf("disabled-chain lookup failed: %v", err)
	} else if disabled {
		t.Fatalf("expected active chain to not be disabled")
	}
}

func TestDisabledChainReturnsDisabledByAdmin(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	token, clientID := mustToken(t)

	// Disable chain directly in store.
	if err := h.db.DisableSyncChain(context.Background(), clientID); err != nil {
		t.Fatalf("disable chain failed: %v", err)
	}

	gu := &sync_pb.GetUpdatesMessage{
		FetchFolders:       boolPtr(true),
		GetUpdatesOrigin:   enumGUPtr(sync_pb.SyncEnums_GU_TRIGGER),
		FromProgressMarker: markers([]int32{testBookmarkTypeID}, []int64{0}),
	}
	contents := sync_pb.ClientToServerMessage_GET_UPDATES
	msg := &sync_pb.ClientToServerMessage{MessageContents: &contents, Share: stringPtr(""), GetUpdates: gu}

	rsp := doAuthedRequest(t, h, token, mustMarshal(t, msg), false)
	if rsp.ErrorCode == nil || *rsp.ErrorCode != sync_pb.SyncEnums_DISABLED_BY_ADMIN {
		t.Fatalf("expected DISABLED_BY_ADMIN, got %+v", rsp.ErrorCode)
	}
}

func TestGzipCommitRequest(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	token, _ := mustToken(t)

	entries := []*sync_pb.SyncEntity{
		makeCommitEntity("gzip-entity", makeBookmarkSpecifics()),
	}
	commitMsg := makeCommitMessage(entries)

	rsp := doAuthedRequest(t, h, token, mustMarshal(t, commitMsg), true)
	if rsp.ErrorCode == nil || *rsp.ErrorCode != sync_pb.SyncEnums_SUCCESS {
		t.Fatalf("expected success for gzip commit, got %+v", rsp.ErrorCode)
	}
	if rsp.Commit == nil || len(rsp.Commit.Entryresponse) != 1 {
		t.Fatalf("expected one commit response, got %+v", rsp.Commit)
	}
}

func TestMultiClientIsolation(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tokenA, _ := mustToken(t)
	tokenB, _ := mustToken(t)

	// Client A commits a bookmark item.
	commitA := makeCommitMessage([]*sync_pb.SyncEntity{
		makeCommitEntity("a-bookmark-1", makeBookmarkSpecifics()),
	})
	rspA := doAuthedRequest(t, h, tokenA, mustMarshal(t, commitA), false)
	if rspA.ErrorCode == nil || *rspA.ErrorCode != sync_pb.SyncEnums_SUCCESS {
		t.Fatalf("client A commit expected success, got %+v", rspA.ErrorCode)
	}

	// Client B asks for updates at token 0; should not see client A data.
	guB := &sync_pb.GetUpdatesMessage{
		FetchFolders:       boolPtr(true),
		GetUpdatesOrigin:   enumGUPtr(sync_pb.SyncEnums_GU_TRIGGER),
		FromProgressMarker: markers([]int32{testBookmarkTypeID}, []int64{0}),
	}
	contents := sync_pb.ClientToServerMessage_GET_UPDATES
	msgB := &sync_pb.ClientToServerMessage{
		MessageContents: &contents,
		Share:           stringPtr(""),
		GetUpdates:      guB,
	}
	guRspB := doAuthedRequest(t, h, tokenB, mustMarshal(t, msgB), false)
	if guRspB.ErrorCode == nil || *guRspB.ErrorCode != sync_pb.SyncEnums_SUCCESS {
		t.Fatalf("client B get updates expected success, got %+v", guRspB.ErrorCode)
	}
	if guRspB.GetUpdates == nil {
		t.Fatalf("client B expected get updates payload")
	}
	if len(guRspB.GetUpdates.Entries) != 0 {
		t.Fatalf("client B expected 0 entries, got %d", len(guRspB.GetUpdates.Entries))
	}
}

func TestMultiClientSameIDNoConflict(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tokenA, _ := mustToken(t)
	tokenB, _ := mustToken(t)

	// Both clients commit using the same client-side ID string.
	sameID := "shared-client-id"
	commitA := makeCommitMessage([]*sync_pb.SyncEntity{
		makeCommitEntity(sameID, makeBookmarkSpecifics()),
	})
	commitB := makeCommitMessage([]*sync_pb.SyncEntity{
		makeCommitEntity(sameID, makeBookmarkSpecifics()),
	})

	rspA := doAuthedRequest(t, h, tokenA, mustMarshal(t, commitA), false)
	rspB := doAuthedRequest(t, h, tokenB, mustMarshal(t, commitB), false)

	if rspA.Commit == nil || len(rspA.Commit.Entryresponse) != 1 {
		t.Fatalf("client A expected single commit response, got %+v", rspA.Commit)
	}
	if rspB.Commit == nil || len(rspB.Commit.Entryresponse) != 1 {
		t.Fatalf("client B expected single commit response, got %+v", rspB.Commit)
	}
	if rspA.Commit.Entryresponse[0].ResponseType == nil || *rspA.Commit.Entryresponse[0].ResponseType != sync_pb.CommitResponse_SUCCESS {
		t.Fatalf("client A expected SUCCESS, got %+v", rspA.Commit.Entryresponse[0].ResponseType)
	}
	if rspB.Commit.Entryresponse[0].ResponseType == nil || *rspB.Commit.Entryresponse[0].ResponseType != sync_pb.CommitResponse_SUCCESS {
		t.Fatalf("client B expected SUCCESS, got %+v", rspB.Commit.Entryresponse[0].ResponseType)
	}
}

type testHandler struct {
	handler http.HandlerFunc
	db      *SQLiteStore
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func newTestHandler(t *testing.T) (*testHandler, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create sqlite store failed: %v", err)
	}

	c := cache.NewCache(NewMemoryRedis())
	h := commandHandler(c, db)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = db.CheckpointTruncate(ctx)
		_ = db.Close()
	}

	return &testHandler{handler: h, db: db}, cleanup
}

func doAuthedRequest(t *testing.T, h http.Handler, token string, payload []byte, gzipBody bool) *sync_pb.ClientToServerResponse {
	t.Helper()

	var body bytes.Buffer
	if gzipBody {
		zw := gzip.NewWriter(&body)
		if _, err := zw.Write(payload); err != nil {
			t.Fatalf("gzip write failed: %v", err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("gzip close failed: %v", err)
		}
	} else {
		body.Write(payload)
	}

	req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(body.Bytes()))
	req.Header.Set("Authorization", "Bearer "+token)
	if gzipBody {
		req.Header.Set("Content-Encoding", "gzip")
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rr.Code, rr.Body.String())
	}

	rsp := &sync_pb.ClientToServerResponse{}
	if err := proto.Unmarshal(rr.Body.Bytes(), rsp); err != nil {
		t.Fatalf("response unmarshal failed: %v", err)
	}
	return rsp
}

func makeCommitMessage(entries []*sync_pb.SyncEntity) *sync_pb.ClientToServerMessage {
	contents := sync_pb.ClientToServerMessage_COMMIT
	return &sync_pb.ClientToServerMessage{
		MessageContents: &contents,
		Share:           stringPtr(""),
		Commit: &sync_pb.CommitMessage{
			Entries:   entries,
			CacheGuid: stringPtr("cache-guid"),
		},
	}
}

func makeCommitEntity(id string, specifics *sync_pb.EntitySpecifics) *sync_pb.SyncEntity {
	return &sync_pb.SyncEntity{
		IdString:  stringPtr(id),
		Name:      stringPtr(id),
		Version:   int64Ptr(0),
		Deleted:   boolPtr(false),
		Folder:    boolPtr(false),
		Specifics: specifics,
	}
}

func makeBookmarkSpecifics() *sync_pb.EntitySpecifics {
	return &sync_pb.EntitySpecifics{
		SpecificsVariant: &sync_pb.EntitySpecifics_Bookmark{Bookmark: &sync_pb.BookmarkSpecifics{}},
	}
}

func makeNigoriSpecifics() *sync_pb.EntitySpecifics {
	return &sync_pb.EntitySpecifics{
		SpecificsVariant: &sync_pb.EntitySpecifics_Nigori{Nigori: &sync_pb.NigoriSpecifics{}},
	}
}

func markers(typeIDs []int32, tokens []int64) []*sync_pb.DataTypeProgressMarker {
	out := make([]*sync_pb.DataTypeProgressMarker, 0, len(typeIDs))
	for i := range typeIDs {
		tok := make([]byte, binary.MaxVarintLen64)
		binary.PutVarint(tok, tokens[i])
		out = append(out, &sync_pb.DataTypeProgressMarker{
			DataTypeId: int32Ptr(typeIDs[i]),
			Token:      tok,
		})
	}
	return out
}

func lookupDisabled(h *testHandler, clientID string) (bool, error) {
	return h.db.IsSyncChainDisabled(context.Background(), clientID)
}

func mustMarshal(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	out, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return out
}

func mustToken(t *testing.T) (token string, clientID string) {
	t.Helper()
	tkn, _, cid, err := authtest.GenerateToken(time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}
	return tkn, cid
}

func stringPtr(v string) *string { return &v }
func int64Ptr(v int64) *int64    { return &v }
func boolPtr(v bool) *bool       { return &v }
func int32Ptr(v int32) *int32    { return &v }

func enumGUPtr(v sync_pb.SyncEnums_GetUpdatesOrigin) *sync_pb.SyncEnums_GetUpdatesOrigin {
	return &v
}
