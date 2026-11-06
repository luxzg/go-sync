package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAuditA06ConcurrentCommands(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	const count = 24
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		msg := makeCommitMessage(nil)
		msg.Commit.Entries = append(msg.Commit.Entries, makeCommitEntity(fmt.Sprint(i), makeBookmarkSpecifics()))
		payload := mustMarshal(t, msg)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != 200 {
				results <- fmt.Errorf("status %d", rr.Code)
			} else {
				results <- nil
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	counts, err := h.db.GetClientItemCount(context.Background(), chain)
	if err != nil || counts.ItemCount != count {
		t.Fatalf("concurrent counts=%+v err=%v want=%d", counts, err, count)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", chain, count)
}

type auditBlockedReader struct {
	io.Reader
	started, release chan struct{}
	once             sync.Once
}

func (r *auditBlockedReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	return r.Reader.Read(p)
}

type auditBlockedWriter struct {
	*httptest.ResponseRecorder
	started, release chan struct{}
	once             sync.Once
}

func (w *auditBlockedWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return w.ResponseRecorder.Write(p)
}

func TestAuditA06NetworkIODoesNotHoldGate(t *testing.T) {
	for _, upload := range []bool{true, false} {
		t.Run(fmt.Sprint(upload), func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, _ := mustToken(t)
			payload := mustMarshal(t, makeCommitMessage(nil))
			started, release := make(chan struct{}), make(chan struct{})
			firstDone, secondDone := make(chan struct{}), make(chan struct{})
			var reader io.Reader = bytes.NewReader(payload)
			var writer http.ResponseWriter = httptest.NewRecorder()
			if upload {
				reader = &auditBlockedReader{Reader: reader, started: started, release: release}
			} else {
				writer = &auditBlockedWriter{ResponseRecorder: httptest.NewRecorder(), started: started, release: release}
			}
			req := httptest.NewRequest(http.MethodPost, "/command/", reader)
			req.Header.Set("Authorization", "Bearer "+token)
			go func() { defer close(firstDone); h.ServeHTTP(writer, req) }()
			<-started
			second := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(payload))
			second.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			go func() { defer close(secondDone); h.ServeHTTP(rr, second) }()
			select {
			case <-secondDone:
				if rr.Code != 200 {
					t.Error(rr.Code)
				}
			case <-time.After(time.Second):
				t.Error("network IO blocked unrelated command")
			}
			close(release)
			<-firstDone
			<-secondDone
		})
	}
}

func TestAuditA06CancelledWaiter(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	if err := h.db.acquireCommand(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.db.acquireCommand(ctx); err != context.Canceled {
		t.Fatal("cancelled gate waiter was admitted", err)
	}
	h.db.releaseCommand()
	if err := h.db.acquireCommand(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.db.releaseCommand()
}
