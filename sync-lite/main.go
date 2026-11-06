package main

import (
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/brave/go-sync/auth"
	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/command"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
)

const (
	payloadLimit10MB = 10 * 1024 * 1024
	version          = "0.4"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and build information, then exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("sync-lite %s (sqlite: %s; %s/%s; %s)\n", version, sqliteBackend, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	log.Printf("sync-lite %s (sqlite backend: %s)", version, sqliteBackend)
	command.LoadHighDeviceLimitClientIDsFromEnv()
	listenAddr := getenvDefault("LISTEN_ADDR", ":8295")
	sqlitePath := getenvDefault("SQLITE_PATH", "./sync-lite.db")
	tlsCertFile := os.Getenv("TLS_CERT_FILE")
	tlsKeyFile := os.Getenv("TLS_KEY_FILE")

	db, err := NewSQLiteStore(sqlitePath)
	if err != nil {
		log.Fatalf("init sqlite store failed: %v", err)
	}
	if db.conflicts != nil {
		log.Print("sync conflict diagnostics enabled (rate-limited; data type and reason only)")
	}

	c := cache.NewCache(NewMemoryRedis())

	handler := commandHandler(c, db)
	mux := http.NewServeMux()
	mux.HandleFunc("/command/", handler)
	mux.HandleFunc("/v2/command/", handler)

	if (tlsCertFile == "") != (tlsKeyFile == "") {
		log.Fatal("TLS config invalid: set both TLS_CERT_FILE and TLS_KEY_FILE, or neither")
	}

	srv := newHTTPServer(listenAddr, mux)

	shutdownDone := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer close(shutdownDone)
		sig := <-sigCh
		log.Printf("received signal %s, shutting down server", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("http shutdown error: %v", err)
		}

		cpCtx, cpCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cpCancel()
		if err := db.CheckpointTruncate(cpCtx); err != nil {
			log.Printf("sqlite checkpoint error during shutdown: %v", err)
		}
		if err := db.Close(); err != nil {
			log.Printf("sqlite close error during shutdown: %v", err)
		}
	}()

	if tlsCertFile != "" && tlsKeyFile != "" {
		log.Printf("sync-lite listening on %s with HTTPS (sqlite: %s, cert: %s, key: %s)", listenAddr, sqlitePath, tlsCertFile, tlsKeyFile)
		if err := srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server stopped: %v", err)
		}
	} else {
		log.Printf("sync-lite listening on %s with HTTP (sqlite: %s)", listenAddr, sqlitePath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server stopped: %v", err)
		}
	}

	<-shutdownDone
	log.Printf("sync-lite stopped cleanly")
}

func getenvDefault(key string, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
}

func commandHandler(c *cache.Cache, db *SQLiteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, err := auth.Authorize(r)
		if err != nil || clientID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if err := db.acquireCommand(r.Context()); err != nil {
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		}
		defer db.releaseCommand()

		disabled, err := db.IsSyncChainDisabled(r.Context(), clientID)
		if err != nil {
			http.Error(w, "unable to complete request", http.StatusInternalServerError)
			return
		}
		if disabled {
			errCode := sync_pb.SyncEnums_DISABLED_BY_ADMIN
			csRsp := sync_pb.ClientToServerResponse{ErrorCode: &errCode}
			out, err := proto.Marshal(&csRsp)
			if err != nil {
				http.Error(w, "Marshal Error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(out)
			return
		}

		// Bound wire bytes independently from decoded bytes (including gzip).
		reader := io.Reader(http.MaxBytesReader(w, r.Body, payloadLimit10MB))
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(reader)
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			reader = gr
		}

		msg, err := io.ReadAll(io.LimitReader(reader, payloadLimit10MB+1))
		var maxBytes *http.MaxBytesError
		if len(msg) > payloadLimit10MB || errors.As(err, &maxBytes) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		pb := &sync_pb.ClientToServerMessage{}
		if err := proto.Unmarshal(msg, pb); err != nil {
			http.Error(w, "Unmarshal error", http.StatusInternalServerError)
			return
		}
		if err := prepareCommand(pb); err != nil {
			http.Error(w, "Invalid command payload", http.StatusBadRequest)
			return
		}

		pbRsp := &sync_pb.ClientToServerResponse{}
		if err := command.HandleClientToServerMessage(r.Context(), c, pb, pbRsp, &commandStore{SQLiteStore: db}, clientID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out, err := proto.Marshal(pbRsp)
		if err != nil {
			http.Error(w, "Marshal Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)
	}
}
