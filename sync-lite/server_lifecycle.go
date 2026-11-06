package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type handlerTracker struct {
	handler http.Handler
	mu      sync.Mutex
	active  int
	closing bool
	drained chan struct{}
}

func newHandlerTracker(h http.Handler) *handlerTracker {
	return &handlerTracker{handler: h, drained: make(chan struct{})}
}

func (t *handlerTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	if t.closing {
		t.mu.Unlock()
		http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
		return
	}
	t.active++
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.active--
		if t.closing && t.active == 0 {
			close(t.drained)
		}
	}()
	t.handler.ServeHTTP(w, r)
}

func (t *handlerTracker) beginShutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closing {
		return
	}
	t.closing = true
	if t.active == 0 {
		close(t.drained)
	}
}

func stopServer(srv *http.Server, t *handlerTracker, db *SQLiteStore, grace, forced time.Duration) error {
	t.beginShutdown()
	ctx, cancel := context.WithTimeout(context.Background(), grace)
	shutdownErr := srv.Shutdown(ctx)
	cancel()
	var closeErr error
	if shutdownErr != nil {
		closeErr = srv.Close()
	}
	timer := time.NewTimer(forced)
	defer timer.Stop()
	select {
	case <-t.drained:
	case <-timer.C:
		// Do not close/checkpoint beneath a handler that did not stop. The
		// caller exits with failure; committed WAL is recovered on next open.
		return errors.Join(shutdownErr, closeErr, fmt.Errorf("active handlers did not drain; SQLite cleanup skipped"))
	}
	cpCtx, cpCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cpCancel()
	checkpointErr := db.CheckpointTruncate(cpCtx)
	dbErr := db.Close()
	return errors.Join(shutdownErr, closeErr, checkpointErr, dbErr)
}
