package main

import (
	"context"
	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/command"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
)

// Serialize only command state, never slow uploads or response socket writes.
func dispatchCommand(ctx context.Context, c *cache.Cache, db *SQLiteStore, pb *sync_pb.ClientToServerMessage, chain string) (*sync_pb.ClientToServerResponse, error) {
	if err := db.acquireCommand(ctx); err != nil {
		return nil, err
	}
	defer db.releaseCommand()
	disabled, err := db.IsSyncChainDisabled(ctx, chain)
	if err != nil {
		return nil, err
	}
	if disabled {
		return &sync_pb.ClientToServerResponse{ErrorCode: sync_pb.SyncEnums_DISABLED_BY_ADMIN.Enum()}, nil
	}
	rsp := &sync_pb.ClientToServerResponse{}
	store := &commandStore{SQLiteStore: db, requestContext: ctx, admissionPending: pb.GetMessageContents() == sync_pb.ClientToServerMessage_GET_UPDATES && pb.GetGetUpdates().GetGetUpdatesOrigin() == sync_pb.SyncEnums_NEW_CLIENT}
	err = command.HandleClientToServerMessage(ctx, c, pb, rsp, store, chain)
	return rsp, err
}
