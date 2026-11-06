package main

import (
	"context"
	"github.com/brave/go-sync/datastore"
)

// Request-owned bookkeeping bridges the unchanged upstream deleted-only return
// contract. Never keep resurrection deltas on the shared store or in globals.
type commandStore struct {
	*SQLiteStore
	normalResurrections  int
	historyResurrections int
}

func (s *commandStore) UpdateSyncEntity(ctx context.Context, e *datastore.SyncEntity, oldVersion int64) (bool, bool, error) {
	return s.SQLiteStore.updateSyncEntity(ctx, e, oldVersion, func() {
		if e.DataType != nil && (*e.DataType == datastore.HistoryTypeID || *e.DataType == datastore.HistoryDeleteDirectiveTypeID) {
			s.historyResurrections++
		} else {
			s.normalResurrections++
		}
	})
}

func (s *commandStore) UpdateClientItemCount(ctx context.Context, counts *datastore.ClientItemCounts, normal, history int) error {
	return s.SQLiteStore.UpdateClientItemCount(ctx, counts, normal+s.normalResurrections, history+s.historyResurrections)
}
