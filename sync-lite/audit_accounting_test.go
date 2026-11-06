package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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
