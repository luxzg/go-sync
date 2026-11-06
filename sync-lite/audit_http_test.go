package main

import (
	"bytes"
	"compress/gzip"
	"google.golang.org/protobuf/encoding/protowire"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func auditPaddedRequest(t *testing.T, size int) []byte {
	t.Helper()
	base := mustMarshal(t, makeCommitMessage(nil))
	padding := size - len(base) - 8
	for {
		out := protowire.AppendTag(append([]byte(nil), base...), 2047, protowire.BytesType)
		out = protowire.AppendBytes(out, make([]byte, padding))
		if len(out) == size {
			return out
		}
		padding += size - len(out)
	}
}

func TestAuditA04BodyLimits(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, _ := mustToken(t)
	for _, compressed := range []bool{false, true} {
		for _, size := range []int{payloadLimit10MB - 1, payloadLimit10MB, payloadLimit10MB + 1} {
			data := auditPaddedRequest(t, size)
			if compressed {
				var b bytes.Buffer
				z := gzip.NewWriter(&b)
				_, _ = z.Write(data)
				_ = z.Close()
				data = b.Bytes()
			}
			req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(data))
			req.Header.Set("Authorization", "Bearer "+token)
			if compressed {
				req.Header.Set("Content-Encoding", "gzip")
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			want := http.StatusOK
			if size > payloadLimit10MB {
				want = http.StatusRequestEntityTooLarge
			}
			if rr.Code != want {
				t.Fatalf("gzip=%v size=%d: got %d want %d", compressed, size, rr.Code, want)
			}
		}
	}
}

func TestAuditA04ValidPrefixAndCorruptGzip(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, _ := mustToken(t)
	for _, compressed := range []bool{false, true} {
		prefix := auditPaddedRequest(t, payloadLimit10MB)
		data := protowire.AppendTag(prefix, 2046, protowire.VarintType)
		data = protowire.AppendVarint(data, 1)
		if compressed {
			var b bytes.Buffer
			z := gzip.NewWriter(&b)
			_, _ = z.Write(data)
			_ = z.Close()
			data = b.Bytes()
		}
		req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		if compressed {
			req.Header.Set("Content-Encoding", "gzip")
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("valid prefix accepted: %d", rr.Code)
		}
	}
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	_, _ = z.Write(mustMarshal(t, makeCommitMessage(nil)))
	_ = z.Close()
	data := b.Bytes()
	data[len(data)-8] ^= 0xff
	req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("corrupt gzip: %d", rr.Code)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", "", 0)
}

func TestAuditA04ServerDeadlines(t *testing.T) {
	s := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Fatal("unbounded server deadline")
	}
}

func TestAuditA04SlowClients(t *testing.T) {
	for _, header := range []bool{true, false} {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		readErr := make(chan error, 1)
		s := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, err := io.ReadAll(r.Body); readErr <- err }))
		s.ReadHeaderTimeout = 30 * time.Millisecond
		s.ReadTimeout = 60 * time.Millisecond
		done := make(chan error, 1)
		go func() { done <- s.Serve(l) }()
		conn, err := net.DialTimeout("tcp", l.Addr().String(), time.Second)
		if err != nil {
			_ = s.Close()
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if header {
			_, err = io.WriteString(conn, "POST / HTTP/1.1\r\nHost: local")
		} else {
			_, err = io.WriteString(conn, "POST / HTTP/1.1\r\nHost: local\r\nContent-Length: 100\r\n\r\nx")
		}
		if err != nil {
			_ = conn.Close()
			_ = s.Close()
			t.Fatal(err)
		}
		_, err = io.ReadAll(conn)
		_ = conn.Close()
		_ = s.Close()
		<-done
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("server did not bound slow client")
		}
		if !header {
			select {
			case err := <-readErr:
				if err == nil {
					t.Fatal("slow body read unexpectedly succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("body handler stuck")
			}
		}
	}
}
