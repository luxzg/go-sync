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
	requestContext       context.Context
	admissionPending     bool
}

func (s *commandStore) context(fallback context.Context) context.Context {
	if s.requestContext != nil {
		return s.requestContext
	}
	return fallback
}

func (s *commandStore) InsertSyncEntity(ctx context.Context, e *datastore.SyncEntity) (bool, error) {
	return s.SQLiteStore.InsertSyncEntity(s.context(ctx), e)
}
func (s *commandStore) InsertSyncEntitiesWithServerTags(ctx context.Context, e []*datastore.SyncEntity) error {
	return s.SQLiteStore.InsertSyncEntitiesWithServerTags(s.context(ctx), e)
}
func (s *commandStore) GetUpdatesForType(ctx context.Context, d int, token int64, folders bool, chain string, size int) (bool, []datastore.SyncEntity, error) {
	if s.admissionPending {
		s.admissionPending = false
		if d == 154522 && token == 0 && !folders && size == 500 {
			return s.deviceAdmissionSample(s.context(ctx), chain)
		}
	}
	return s.SQLiteStore.GetUpdatesForType(s.context(ctx), d, token, folders, chain, size)
}
func (s *commandStore) HasServerDefinedUniqueTag(ctx context.Context, chain, tag string) (bool, error) {
	return s.SQLiteStore.HasServerDefinedUniqueTag(s.context(ctx), chain, tag)
}
func (s *commandStore) GetClientItemCount(ctx context.Context, chain string) (*datastore.ClientItemCounts, error) {
	return s.SQLiteStore.GetClientItemCount(s.context(ctx), chain)
}
func (s *commandStore) ClearServerData(ctx context.Context, chain string) ([]datastore.SyncEntity, error) {
	return s.SQLiteStore.ClearServerData(s.context(ctx), chain)
}
func (s *commandStore) DisableSyncChain(ctx context.Context, chain string) error {
	return s.SQLiteStore.DisableSyncChain(s.context(ctx), chain)
}
func (s *commandStore) IsSyncChainDisabled(ctx context.Context, chain string) (bool, error) {
	return s.SQLiteStore.IsSyncChainDisabled(s.context(ctx), chain)
}
func (s *commandStore) HasItem(ctx context.Context, chain, id string) (bool, error) {
	return s.SQLiteStore.HasItem(s.context(ctx), chain, id)
}

func (s *commandStore) UpdateSyncEntity(ctx context.Context, e *datastore.SyncEntity, oldVersion int64) (bool, bool, error) {
	return s.SQLiteStore.updateSyncEntity(s.context(ctx), e, oldVersion, func() {
		if e.DataType != nil && (*e.DataType == datastore.HistoryTypeID || *e.DataType == datastore.HistoryDeleteDirectiveTypeID) {
			s.historyResurrections++
		} else {
			s.normalResurrections++
		}
	})
}

func (s *commandStore) UpdateClientItemCount(ctx context.Context, counts *datastore.ClientItemCounts, normal, history int) error {
	return s.SQLiteStore.UpdateClientItemCount(s.context(ctx), counts, normal+s.normalResurrections, history+s.historyResurrections)
}

var _ datastore.Datastore = (*commandStore)(nil)
