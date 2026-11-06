package main

import (
	"bytes"
	"context"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type auditCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func TestAuditA09RefusesCleanupUnderStuckHandler(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	tracker := newHandlerTracker(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release }))
	go func() {
		defer close(finished)
		tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-started
	srv := newHTTPServer("", tracker)
	err := stopServer(srv, tracker, h.db, 20*time.Millisecond, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "cleanup skipped") {
		close(release)
		<-finished
		t.Fatal("stuck handler was reported clean", err)
	}
	if err := h.db.db.Ping(); err != nil {
		close(release)
		<-finished
		t.Fatal("database closed beneath handler", err)
	}
	rr := httptest.NewRecorder()
	tracker.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusServiceUnavailable {
		close(release)
		<-finished
		t.Fatal("new handler admitted during shutdown")
	}
	close(release)
	<-finished
	if err := stopServer(srv, tracker, h.db, time.Second, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := h.db.db.Ping(); err == nil {
		t.Fatal("drained database not closed")
	}
}

func TestAuditA09ForceCancelsAndDrains(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	started := make(chan struct{})
	tracker := newHandlerTracker(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	srv := httptest.NewServer(tracker)
	defer srv.Close()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		rsp, err := srv.Client().Get(srv.URL)
		if err == nil {
			_ = rsp.Body.Close()
		}
	}()
	<-started
	err := stopServer(srv.Config, tracker, h.db, 20*time.Millisecond, time.Second)
	if err == nil {
		t.Fatal("forced shutdown misreported as clean")
	}
	<-clientDone
	if err := h.db.db.Ping(); err == nil {
		t.Fatal("forced/drained database not closed")
	}
}

func (r auditCancelReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.cancel()
	}
	return n, err
}

func TestAuditA09RequestCancellation(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "clear"}[clear], func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			msg := makeCommitMessage([]*sync_pb.SyncEntity{makeCommitEntity("new", makeBookmarkSpecifics())})
			expected := 0
			if clear {
				protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", makeCommitEntity("keep", makeBookmarkSpecifics()))[0])
				expected = 1
				msg = &sync_pb.ClientToServerMessage{Share: stringPtr(""), MessageContents: sync_pb.ClientToServerMessage_CLEAR_SERVER_DATA.Enum(), ClearServerData: &sync_pb.ClearServerDataMessage{}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := auditCancelReader{bytes.NewReader(mustMarshal(t, msg)), cancel}
			req := httptest.NewRequest(http.MethodPost, "/command/", reader).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+token)
			h.ServeHTTP(httptest.NewRecorder(), req)
			sqliteTestRowCount(t, h.db.db, "sync_entities", chain, expected)
			disabled, err := h.db.IsSyncChainDisabled(context.Background(), chain)
			if err != nil || disabled {
				t.Fatal("cancelled command disabled chain", disabled, err)
			}
		})
	}
}
