package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/brave/go-sync/datastore"
	"math"
)

// The first write owns SQLite's writer lock before reading the allocation floor.
// The clock and entity commit/rollback together, including across store handles.
func stampEntity(ctx context.Context, tx *sql.Tx, e *datastore.SyncEntity, preserveRootVersion bool) error {
	if e.DataType == nil || e.Mtime == nil {
		return fmt.Errorf("missing entity type or timestamp")
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO sync_clocks (client_id,data_type,mtime) VALUES (?,?,0)`, e.ClientID, *e.DataType); err != nil {
		return err
	}
	var previous int64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(mtime, COALESCE((SELECT MAX(mtime) FROM sync_entities WHERE client_id=? AND data_type=?),0)) FROM sync_clocks WHERE client_id=? AND data_type=?`, e.ClientID, *e.DataType, e.ClientID, *e.DataType).Scan(&previous); err != nil {
		return err
	}
	if previous == math.MaxInt64 {
		return fmt.Errorf("sync timestamp exhausted")
	}
	proposed := *e.Mtime
	stamp := max(proposed, previous+1)
	if _, err := tx.ExecContext(ctx, `UPDATE sync_clocks SET mtime=? WHERE client_id=? AND data_type=?`, stamp, e.ClientID, *e.DataType); err != nil {
		return err
	}
	// Parent conversions can alias Ctime and Mtime. Replace pointers, not values.
	if e.Version != nil && *e.Version == proposed && !preserveRootVersion {
		e.Version = &stamp
	}
	e.Mtime = &stamp
	key := fmt.Sprintf("%d#%d", *e.DataType, stamp)
	e.DataTypeMtime = &key
	return nil
}
