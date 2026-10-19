package main

import (
	"log"
	"sync"
	"time"

	"github.com/brave/go-sync/datastore"
)

const conflictLogInterval = 30 * time.Second
const maxConflictLogKeys = 128

type conflictLogKey struct {
	operation string
	reason    string
	dataType  int
}

type conflictDiagnostics struct {
	mu     sync.Mutex
	logger *log.Logger
	last   map[conflictLogKey]time.Time
}

func (s *SQLiteStore) logConflict(operation, reason string, entity *datastore.SyncEntity) {
	if s.conflicts == nil {
		return
	}
	dataType := 0
	if entity.DataType != nil {
		dataType = *entity.DataType
	}
	s.conflicts.record(conflictLogKey{operation: operation, reason: reason, dataType: dataType}, time.Now())
}

func (d *conflictDiagnostics) record(key conflictLogKey, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.last[key]; ok && now.Sub(last) < conflictLogInterval {
		return
	}
	// Bound both log frequency and bookkeeping, even for direct datastore callers.
	if len(d.last) >= maxConflictLogKeys {
		for oldKey, last := range d.last {
			if now.Sub(last) >= conflictLogInterval {
				delete(d.last, oldKey)
			}
		}
		if len(d.last) >= maxConflictLogKeys {
			return
		}
	}
	d.last[key] = now
	d.logger.Printf("sync conflict operation=%s reason=%s data_type=%d", key.operation, key.reason, key.dataType)
}
