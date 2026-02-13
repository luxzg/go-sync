package main

import (
	"compress/gzip"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/brave/go-sync/auth"
	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/command"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
)

const payloadLimit10MB = 10 * 1024 * 1024

func main() {
	listenAddr := getenvDefault("LISTEN_ADDR", ":8295")
	sqlitePath := getenvDefault("SQLITE_PATH", "./sync-lite.db")
	tlsCertFile := os.Getenv("TLS_CERT_FILE")
	tlsKeyFile := os.Getenv("TLS_KEY_FILE")

	db, err := NewSQLiteStore(sqlitePath)
	if err != nil {
		log.Fatalf("init sqlite store failed: %v", err)
	}

	c := cache.NewCache(NewMemoryRedis())

	handler := commandHandler(c, db)
	mux := http.NewServeMux()
	mux.HandleFunc("/command/", handler)
	mux.HandleFunc("/v2/command/", handler)

	if (tlsCertFile == "") != (tlsKeyFile == "") {
		log.Fatal("TLS config invalid: set both TLS_CERT_FILE and TLS_KEY_FILE, or neither")
	}

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

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

func commandHandler(c *cache.Cache, db *SQLiteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, err := auth.Authorize(r)
		if err != nil || clientID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

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

		reader := r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Create gzip reader failed", http.StatusInternalServerError)
				return
			}
			defer gr.Close()
			reader = gr
		}

		msg, err := io.ReadAll(io.LimitReader(reader, payloadLimit10MB))
		if err != nil {
			http.Error(w, "Read request body error", http.StatusInternalServerError)
			return
		}

		pb := &sync_pb.ClientToServerMessage{}
		if err := proto.Unmarshal(msg, pb); err != nil {
			http.Error(w, "Unmarshal error", http.StatusInternalServerError)
			return
		}

		pbRsp := &sync_pb.ClientToServerResponse{}
		if err := command.HandleClientToServerMessage(r.Context(), c, pb, pbRsp, db, clientID); err != nil {
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
