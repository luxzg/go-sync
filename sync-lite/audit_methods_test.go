package main

import (
	"bytes"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditA14OnlyPOSTMutates(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	for _, path := range []string{"/command/", "/v2/command/"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead} {
			payload := mustMarshal(t, makeCommitMessage([]*sync_pb.SyncEntity{makeCommitEntity("must-not-exist", makeBookmarkSpecifics())}))
			req := httptest.NewRequest(method, path, bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusMethodNotAllowed || rr.Header().Get("Allow") != "POST" {
				t.Fatalf("%s %s: status %d Allow=%q", method, path, rr.Code, rr.Header().Get("Allow"))
			}
			sqliteTestRowCount(t, h.db.db, "sync_entities", chain, 0)
		}
	}
	for _, path := range []string{"/command/", "/v2/command/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatal("unauthenticated reachability changed")
		}
	}
}
