package main

import (
	"context"
	"github.com/brave/go-sync/datastore"
	"time"
)

// The parent admits fewer than 50 devices, or fewer than 100 for opted-in
// chains. Sampling 100 live rows suffices for either decision. Only the first
// NEW_CLIENT setup call uses this path; ordinary downloads still paginate.
func (s *commandStore) deviceAdmissionSample(ctx context.Context, chain string) (bool, []datastore.SyncEntity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT client_id,id FROM sync_entities
        WHERE client_id=? AND data_type=154522 AND mtime>0 AND folder=0
        AND COALESCE(deleted,0)=0
        AND (expiration_time IS NULL OR expiration_time=0 OR expiration_time>=?)
        ORDER BY mtime,id LIMIT 100`, chain, time.Now().Unix())
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()
	entities := make([]datastore.SyncEntity, 0, 100)
	for rows.Next() {
		var e datastore.SyncEntity
		if err := rows.Scan(&e.ClientID, &e.ID); err != nil {
			return false, nil, err
		}
		live := false
		e.Deleted = &live
		entities = append(entities, e)
	}
	return false, entities, rows.Err()
}
