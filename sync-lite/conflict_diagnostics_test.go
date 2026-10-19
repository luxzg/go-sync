package main

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConflictDiagnosticsStartup(t *testing.T) {
	for _, value := range []string{"", "0", "true", "1"} {
		t.Run("env="+value, func(t *testing.T) {
			t.Setenv("SYNC_LOG_CONFLICTS", value)
			s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "startup.db"))
			if (s.conflicts != nil) != (value == "1") {
				t.Fatalf("diagnostics enabled=%v for setting %q", s.conflicts != nil, value)
			}
			if s.conflicts != nil && (s.conflicts.logger == nil || s.conflicts.last == nil || len(s.conflicts.last) != 0) {
				t.Fatal("enabled diagnostics not initialized empty with a logger")
			}
		})
	}
}

func TestConflictDiagnosticsRejectedWrites(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			setting := "0"
			if enabled {
				setting = "1"
			}
			t.Setenv("SYNC_LOG_CONFLICTS", setting)
			s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "rejections.db"))
			var output conflictTestBuffer
			if enabled {
				s.conflicts.logger = log.New(&output, "", 0)
			}
			seed := sqliteTestEntity("PRIVATE-CHAIN-KEY", "PRIVATE-ENTITY-ID", 100)
			seed.Name = sqliteTestPtr("PRIVATE-BOOKMARK-NAME")
			seed.NonUniqueName = sqliteTestPtr("https://private.example.invalid/PRIVATE-URL")
			seed.Specifics = []byte("PRIVATE-PAYLOAD-KEY")
			sqliteTestInsert(t, s, seed)
			before := sqliteTestSnapshot(t, s.db)
			var want strings.Builder
			for _, reason := range []string{"duplicate_client_tag", "duplicate_entity_id", "missing_entity", "version_mismatch"} {
				t.Run(reason, func(t *testing.T) {
					entity := seed
					entity.Name = sqliteTestPtr("PRIVATE-REJECTED-NAME")
					entity.Specifics = []byte("PRIVATE-REJECTED-PAYLOAD")
					operation := "insert"
					switch reason {
					case "duplicate_client_tag":
						entity.ID = "PRIVATE-NEW-ID"
					case "duplicate_entity_id":
						entity.ClientDefinedUniqueTag = sqliteTestPtr("PRIVATE-NEW-TAG")
					case "missing_entity", "version_mismatch":
						operation = "update"
						if reason == "missing_entity" {
							entity.ID = "PRIVATE-MISSING-ID"
						}
					}
					if operation == "insert" {
						conflict, err := s.InsertSyncEntity(sqliteTestContext(t), &entity)
						if !conflict || err == nil || !isConstraintErr(err) {
							t.Fatalf("insert returned conflict=%v err=%v; want constraint conflict", conflict, err)
						}
					} else {
						conflict, deleted, err := s.UpdateSyncEntity(sqliteTestContext(t), &entity, 0)
						if !conflict || deleted || err != nil {
							t.Fatalf("update returned conflict=%v deleted=%v err=%v", conflict, deleted, err)
						}
					}
					if !reflect.DeepEqual(sqliteTestSnapshot(t, s.db), before) {
						t.Fatal("rejected write changed exact database contents or schema")
					}
					if enabled {
						fmt.Fprintf(&want, "sync conflict operation=%s reason=%s data_type=%d\n", operation, reason, *seed.DataType)
					}
					if output.String() != want.String() {
						t.Fatalf("unexpected diagnostic output: %q", output.String())
					}
				})
			}
			for _, marker := range []string{seed.ClientID, seed.ID, *seed.Name, *seed.NonUniqueName, string(seed.Specifics), *seed.ClientDefinedUniqueTag, "PRIVATE-", "https://"} {
				if strings.Contains(output.String(), marker) {
					t.Fatalf("diagnostics exposed synthetic private marker %q", marker)
				}
			}
		})
	}
}

func TestConflictDiagnosticsRateLimit(t *testing.T) {
	var output conflictTestBuffer
	d := &conflictDiagnostics{logger: log.New(&output, "", 0), last: make(map[conflictLogKey]time.Time)}
	now := time.Unix(1_700_000_000, 0)
	key := conflictLogKey{"update", "missing_entity", 32904}
	d.record(key, now)
	d.record(key, now.Add(30*time.Second-time.Nanosecond))
	for _, independent := range []conflictLogKey{
		{"update", "missing_entity", 47745},
		{"update", "version_mismatch", 32904},
		{"insert", "missing_entity", 32904},
	} {
		d.record(independent, now)
	}
	if strings.Count(output.String(), "\n") != 4 || d.last[key] != now {
		t.Fatal("suppression failed, extended the interval, or suppressed an independent key")
	}
	d.record(key, now.Add(30*time.Second))
	if strings.Count(output.String(), "\n") != 5 {
		t.Fatal("logging did not resume at exactly 30 seconds")
	}
}

func TestConflictDiagnosticsCapacity(t *testing.T) {
	var output conflictTestBuffer
	d := &conflictDiagnostics{logger: log.New(&output, "", 0), last: make(map[conflictLogKey]time.Time)}
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < 128; i++ {
		d.record(conflictLogKey{"update", "missing_entity", i}, now)
	}
	newKey := conflictLogKey{"update", "missing_entity", 128}
	d.record(newKey, now.Add(30*time.Second-time.Nanosecond))
	if len(d.last) != 128 || strings.Count(output.String(), "\n") != 128 {
		t.Fatal("capacity was exceeded or an unexpired key evicted")
	}
	if _, exists := d.last[newKey]; exists {
		t.Fatal("suppressed key was retained at capacity")
	}
	d.record(newKey, now.Add(30*time.Second))
	if len(d.last) != 1 || strings.Count(output.String(), "\n") != 129 {
		t.Fatal("expired keys not reclaimed or logging did not resume")
	}
}

func TestConflictDiagnosticsConcurrent(t *testing.T) {
	var output conflictTestBuffer
	d := &conflictDiagnostics{logger: log.New(&output, "", 0), last: make(map[conflictLogKey]time.Time)}
	now := time.Unix(1_700_000_000, 0)
	var wg sync.WaitGroup
	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func(dataType int) {
			defer wg.Done()
			d.record(conflictLogKey{"update", "missing_entity", dataType}, now)
			_ = output.String()
		}(i % 4)
	}
	wg.Wait()
	if len(d.last) != 4 || strings.Count(output.String(), "\n") != 4 {
		t.Fatal("concurrent repeated keys were lost or emitted more than once")
	}
}

type conflictTestBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *conflictTestBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *conflictTestBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}
