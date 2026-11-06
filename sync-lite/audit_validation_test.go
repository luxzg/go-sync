package main

import (
	"bytes"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditA08SemanticRequests(t *testing.T) {
	for _, missing := range []string{"get-updates", "marker-type"} {
		t.Run(missing, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, _ := mustToken(t)
			msg := &sync_pb.ClientToServerMessage{Share: stringPtr(""), MessageContents: sync_pb.ClientToServerMessage_GET_UPDATES.Enum()}
			if missing == "marker-type" {
				msg.GetUpdates = &sync_pb.GetUpdatesMessage{FromProgressMarker: []*sync_pb.DataTypeProgressMarker{{}}}
			}
			req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(mustMarshal(t, msg)))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			defer func() {
				if recover() != nil {
					t.Error("wire-valid missing semantic field panicked")
				}
			}()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("invalid semantic request returned %d", rr.Code)
			}
		})
	}
}

func TestAuditA08InvalidEntriesPreserveBatchResults(t *testing.T) {
	for _, kind := range []string{"empty-specifics", "unknown-specifics", "history-tag"} {
		t.Run(kind, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, _ := mustToken(t)
			specifics := &sync_pb.EntitySpecifics{}
			if kind == "unknown-specifics" {
				specifics.ProtoReflect().SetUnknown([]byte{0xf8, 0x7f, 1})
			}
			if kind == "history-tag" {
				specifics.SpecificsVariant = &sync_pb.EntitySpecifics_History{History: &sync_pb.HistorySpecifics{}}
			}
			bad := makeCommitEntity("bad", specifics)
			good := makeCommitEntity("good", makeBookmarkSpecifics())
			defer func() {
				if recover() != nil {
					t.Error("malformed entry panicked instead of INVALID_MESSAGE")
				}
			}()
			rsp := doAuthedRequest(t, h, token, mustMarshal(t, makeCommitMessage([]*sync_pb.SyncEntity{bad, good})), false)
			if len(rsp.GetCommit().GetEntryresponse()) != 2 || rsp.Commit.Entryresponse[0].GetResponseType() != sync_pb.CommitResponse_INVALID_MESSAGE || rsp.Commit.Entryresponse[1].GetResponseType() != sync_pb.CommitResponse_SUCCESS {
				t.Fatalf("incorrect per-entry outcomes: %v", rsp)
			}
			sqliteTestRowCount(t, h.db.db, "sync_entities", "", 1)
		})
	}
}
