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

func TestAuditA09AtomicClearSurvivesCancellation(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", auditTaggedEntity("keep-until-clear"))[0])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &commandStore{SQLiteStore: h.db, requestContext: ctx}
	if err := store.DisableSyncChain(context.Background(), chain); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := store.ClearServerData(context.Background(), chain); err != nil {
		t.Fatal("post-commit cancellation left partial clear", err)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", chain, 1)
	sqliteTestRowCount(t, h.db.db, "unique_tags", chain, 0)
	sqliteTestRowCount(t, h.db.db, "client_item_counts", chain, 0)
	disabled, err := h.db.IsSyncChainDisabled(context.Background(), chain)
	if err != nil || !disabled {
		t.Fatal("atomic clear lost terminal marker", disabled, err)
	}
}

func TestAuditA09AtomicClearRollback(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", auditTaggedEntity("keep"))[0])
	if _, err := h.db.db.Exec(`CREATE TRIGGER audit_clear_failure BEFORE DELETE ON sync_entities WHEN OLD.id<>'disabled_chain' BEGIN SELECT RAISE(ABORT,'synthetic clear failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := upgradeSnapshot(t, context.Background(), h.db.db)
	store := &commandStore{SQLiteStore: h.db, requestContext: context.Background()}
	if err := store.DisableSyncChain(context.Background(), chain); err == nil {
		t.Fatal("clear failure not reported")
	}
	if !bytes.Equal(before, upgradeSnapshot(t, context.Background(), h.db.db)) {
		t.Fatal("failed clear left disabled marker or partial deletions")
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
