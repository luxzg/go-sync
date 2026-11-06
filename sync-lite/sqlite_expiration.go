package main

import (
	"context"
	"database/sql"
	"github.com/brave/go-sync/datastore"
	"time"
)

const expirationBatchSize = 256

// Retire only the protocol's explicitly expiring History family. No write
// transaction is opened when there is no expired work. Normal tombstones stay.
func (s *SQLiteStore) retireExpired(ctx context.Context, chain string, id, tag *string) error {
	return s.retireExpiredAt(ctx, chain, id, tag, time.Now().Unix())
}

func (s *SQLiteStore) retireExpiredAt(ctx context.Context, chain string, id, tag *string, now int64) error {
	where := `client_id=? AND data_type IN (?,?) AND expiration_time>0 AND expiration_time<?`
	args := []any{chain, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID, now}
	if id != nil {
		where += ` AND (id=?`
		args = append(args, *id)
		if tag != nil {
			where += ` OR client_defined_unique_tag=?`
			args = append(args, *tag)
		}
		where += `)`
	}
	exists, err := hasRow(ctx, s.db, `SELECT 1 FROM sync_entities WHERE `+where+` LIMIT 1`, args...)
	if err != nil || !exists {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// A legacy database may not yet have a clock row. Preserve the highest
	// issued timestamp before deleting even the last expiring entity.
	if _, err := tx.ExecContext(ctx, `INSERT INTO sync_clocks (client_id,data_type,mtime)
		SELECT client_id,data_type,COALESCE(MAX(mtime),0) FROM sync_entities
		WHERE client_id=? AND data_type IN (?,?) GROUP BY client_id,data_type
		ON CONFLICT(client_id,data_type) DO UPDATE SET mtime=MAX(mtime,excluded.mtime)`, chain, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID); err != nil {
		return err
	}
	deletionArgs := append([]any{chain}, args...)
	deletionArgs = append(deletionArgs, expirationBatchSize)
	rows, err := tx.QueryContext(ctx, `DELETE FROM sync_entities WHERE client_id=? AND id IN
        (SELECT id FROM sync_entities WHERE `+where+` ORDER BY expiration_time,id LIMIT ?)
        RETURNING client_defined_unique_tag,data_type`, deletionArgs...)
	if err != nil {
		return err
	}
	tags := map[string]bool{}
	for rows.Next() {
		var tag sql.NullString
		var dt int
		if err := rows.Scan(&tag, &dt); err != nil {
			_ = rows.Close()
			return err
		}
		if tag.Valid && dt != datastore.HistoryTypeID {
			tags[tag.String] = true
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for tag := range tags {
		_, err = tx.ExecContext(ctx, `DELETE FROM unique_tags WHERE client_id=? AND id=?
            AND NOT EXISTS (SELECT 1 FROM sync_entities WHERE client_id=?
            AND client_defined_unique_tag=? AND data_type<>? AND COALESCE(deleted,0)=0
			AND NOT (data_type=? AND COALESCE(expiration_time,0)>0 AND COALESCE(expiration_time,0)<?))`,
			chain, clientTagItemPrefix+tag, chain, tag, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
