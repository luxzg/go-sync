package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brave/go-sync/datastore"
)

const (
	sqliteCountVersion         = 2
	disabledChainID            = "disabled_chain"
	reasonDeleted              = "deleted"
	clientTagItemPrefix        = "Client#"
	serverTagItemPrefix        = "Server#"
	historyExpirationIntervalS = 14 * 24 * 60 * 60
	periodDurationSecs         = historyExpirationIntervalS / 4
)

type SQLiteStore struct {
	db          *sql.DB
	conflicts   *conflictDiagnostics
	commandGate chan struct{}
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path != ":memory:" {
		// Only initialize new files. Never chmod an existing operator-managed DB.
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			if err := file.Close(); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	dsn, err := sqliteDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(sqliteDriverName, dsn)
	if err != nil {
		return nil, err
	}
	if path == ":memory:" {
		// This DSN is private per physical connection. Retain exactly one for
		// the store lifetime, rather than silently opening independent databases.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	store := &SQLiteStore{db: db, commandGate: make(chan struct{}, 1)}
	if os.Getenv("SYNC_LOG_CONFLICTS") == "1" {
		store.conflicts = &conflictDiagnostics{logger: log.Default(), last: make(map[conflictLogKey]time.Time)}
	}
	if err := store.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.CheckpointTruncate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CheckpointTruncate(ctx context.Context) error {
	var busy, logFrames, checkpointedFrames int
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return err
	}
	if busy != 0 {
		return fmt.Errorf("sqlite checkpoint busy: %d of %d WAL frames checkpointed", checkpointedFrames, logFrames)
	}
	return nil
}

func (s *SQLiteStore) initSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sync_clocks (
			client_id TEXT NOT NULL, data_type INTEGER NOT NULL,
			mtime INTEGER NOT NULL, PRIMARY KEY(client_id, data_type)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_entities (
			client_id TEXT NOT NULL,
			id TEXT NOT NULL,
			parent_id TEXT,
			version INTEGER,
			mtime INTEGER,
			ctime INTEGER,
			name TEXT,
			non_unique_name TEXT,
			server_defined_unique_tag TEXT,
			deleted INTEGER,
			originator_cache_guid TEXT,
			originator_client_item_id TEXT,
			specifics BLOB,
			data_type INTEGER,
			folder INTEGER,
			client_defined_unique_tag TEXT,
			unique_position BLOB,
			data_type_mtime TEXT,
			expiration_time INTEGER,
			PRIMARY KEY (client_id, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_entities_updates
			ON sync_entities(client_id, data_type, mtime)`,
		`CREATE TABLE IF NOT EXISTS unique_tags (
			client_id TEXT NOT NULL,
			id TEXT NOT NULL,
			mtime INTEGER,
			ctime INTEGER,
			PRIMARY KEY (client_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS client_item_counts (
			client_id TEXT PRIMARY KEY,
			item_count INTEGER NOT NULL DEFAULT 0,
			history_item_count_period1 INTEGER NOT NULL DEFAULT 0,
			history_item_count_period2 INTEGER NOT NULL DEFAULT 0,
			history_item_count_period3 INTEGER NOT NULL DEFAULT 0,
			history_item_count_period4 INTEGER NOT NULL DEFAULT 0,
			last_period_change_time INTEGER NOT NULL DEFAULT 0,
			version INTEGER NOT NULL DEFAULT 0
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) InsertSyncEntity(ctx context.Context, entity *datastore.SyncEntity) (bool, error) {
	if entity.DataType != nil && (*entity.DataType == datastore.HistoryTypeID || *entity.DataType == datastore.HistoryDeleteDirectiveTypeID) {
		if err := s.retireExpired(ctx, entity.ClientID, &entity.ID, entity.ClientDefinedUniqueTag); err != nil {
			return false, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := stampEntity(ctx, tx, entity); err != nil {
		return false, err
	}

	if entity.ClientDefinedUniqueTag != nil && entity.DataType != nil && *entity.DataType != datastore.HistoryTypeID {
		now := time.Now().UnixMilli()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO unique_tags (client_id, id, mtime, ctime) VALUES (?, ?, ?, ?)`,
			entity.ClientID,
			clientTagItemPrefix+*entity.ClientDefinedUniqueTag,
			now,
			now,
		)
		if err != nil {
			if isConstraintErr(err) {
				s.logConflict("insert", "duplicate_client_tag", entity)
				return true, err
			}
			return false, err
		}
	}

	if err := insertSyncEntity(ctx, tx, entity); err != nil {
		if isConstraintErr(err) {
			s.logConflict("insert", "duplicate_entity_id", entity)
			return true, err
		}
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return false, nil
}

func (s *SQLiteStore) InsertSyncEntitiesWithServerTags(ctx context.Context, entities []*datastore.SyncEntity) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	for _, entity := range entities {
		if err := stampEntity(ctx, tx, entity); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO unique_tags (client_id, id, mtime, ctime) VALUES (?, ?, ?, ?)`,
			entity.ClientID,
			serverTagItemPrefix+ptrString(entity.ServerDefinedUniqueTag),
			now,
			now,
		)
		if err != nil {
			return err
		}

		if err := insertSyncEntity(ctx, tx, entity); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) UpdateSyncEntity(ctx context.Context, entity *datastore.SyncEntity, oldVersion int64) (bool, bool, error) {
	return s.updateSyncEntity(ctx, entity, oldVersion, nil)
}

func (s *SQLiteStore) updateSyncEntity(ctx context.Context, entity *datastore.SyncEntity, oldVersion int64, resurrected func()) (bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer tx.Rollback()
	if err := stampEntity(ctx, tx, entity); err != nil {
		return false, false, err
	}

	var currentVersion sql.NullInt64
	var oldDeletedRaw sql.NullInt64
	var storedType sql.NullInt64
	var storedTag sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT version, deleted, data_type, client_defined_unique_tag FROM sync_entities WHERE client_id = ? AND id = ?`,
		entity.ClientID, entity.ID,
	).Scan(&currentVersion, &oldDeletedRaw, &storedType, &storedTag)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logConflict("update", "missing_entity", entity)
			return true, false, nil
		}
		return false, false, err
	}

	// An update cannot change identity, even when the incoming type normally
	// bypasses version checking (History). Omitted tags retain stored identity.
	if !storedType.Valid || entity.DataType == nil || int64(*entity.DataType) != storedType.Int64 ||
		(entity.ClientDefinedUniqueTag != nil && (!storedTag.Valid || *entity.ClientDefinedUniqueTag != storedTag.String)) {
		return true, false, nil
	}
	if int(storedType.Int64) != datastore.HistoryTypeID {
		if !currentVersion.Valid || currentVersion.Int64 != oldVersion {
			s.logConflict("update", "version_mismatch", entity)
			return true, false, nil
		}
	}
	oldDeleted := oldDeletedRaw.Valid && oldDeletedRaw.Int64 != 0
	newDeleted := oldDeleted
	if entity.Deleted != nil {
		newDeleted = *entity.Deleted
	}
	if oldDeleted && !newDeleted && storedTag.Valid && int(storedType.Int64) != datastore.HistoryTypeID {
		var owners int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities
			WHERE client_id=? AND client_defined_unique_tag=? AND id<>?
			AND data_type<>? AND COALESCE(deleted,0)=0`, entity.ClientID, storedTag.String, entity.ID, datastore.HistoryTypeID).Scan(&owners); err != nil {
			return false, false, err
		}
		if owners != 0 {
			return true, false, nil
		}
		now := time.Now().UnixMilli()
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO unique_tags (client_id,id,mtime,ctime) VALUES (?,?,?,?)`, entity.ClientID, clientTagItemPrefix+storedTag.String, now, now); err != nil {
			return false, false, err
		}
	}

	setParts := []string{"version = ?", "mtime = ?", "specifics = ?", "data_type_mtime = ?"}
	args := []any{nullableInt64(entity.Version), nullableInt64(entity.Mtime), entity.Specifics, nullableString(entity.DataTypeMtime)}

	if entity.UniquePosition != nil {
		setParts = append(setParts, "unique_position = ?")
		args = append(args, entity.UniquePosition)
	}
	if entity.ParentID != nil {
		setParts = append(setParts, "parent_id = ?")
		args = append(args, entity.ParentID)
	}
	if entity.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, entity.Name)
	}
	if entity.NonUniqueName != nil {
		setParts = append(setParts, "non_unique_name = ?")
		args = append(args, entity.NonUniqueName)
	}
	if entity.Deleted != nil {
		setParts = append(setParts, "deleted = ?")
		args = append(args, boolToInt(*entity.Deleted))
	}
	if entity.Folder != nil {
		setParts = append(setParts, "folder = ?")
		args = append(args, boolToInt(*entity.Folder))
	}

	args = append(args, entity.ClientID, entity.ID)
	query := fmt.Sprintf(`UPDATE sync_entities SET %s WHERE client_id = ? AND id = ?`, strings.Join(setParts, ", "))
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return false, false, err
	}

	if entity.Deleted != nil && storedTag.Valid &&
		*entity.Deleted && int(storedType.Int64) != datastore.HistoryTypeID {
		_, err = tx.ExecContext(ctx,
			`DELETE FROM unique_tags WHERE client_id = ? AND id = ?
			AND NOT EXISTS (SELECT 1 FROM sync_entities WHERE client_id=?
			AND client_defined_unique_tag=? AND data_type<>? AND COALESCE(deleted,0)=0)`,
			entity.ClientID,
			clientTagItemPrefix+storedTag.String,
			entity.ClientID, storedTag.String, datastore.HistoryTypeID,
		)
		if err != nil {
			return false, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, false, err
	}

	if oldDeleted && !newDeleted && resurrected != nil {
		resurrected()
	}
	return false, !oldDeleted && newDeleted, nil
}

func (s *SQLiteStore) GetUpdatesForType(ctx context.Context, dataType int, clientToken int64, fetchFolders bool, clientID string, maxSize int) (bool, []datastore.SyncEntity, error) {
	if dataType == datastore.HistoryTypeID || dataType == datastore.HistoryDeleteDirectiveTypeID {
		if err := s.retireExpired(ctx, clientID, nil, nil); err != nil {
			return false, nil, err
		}
	}
	limit := maxSize + 1
	if limit < 1 {
		limit = 1
	}

	query := `
		SELECT
			client_id, id, parent_id, version, mtime, ctime, name, non_unique_name,
			server_defined_unique_tag, deleted, originator_cache_guid, originator_client_item_id,
			specifics, data_type, folder, client_defined_unique_tag, unique_position,
			data_type_mtime, expiration_time
		FROM sync_entities
		WHERE client_id = ?
		  AND data_type = ?
		  AND mtime > ?
		  AND (expiration_time IS NULL OR expiration_time = 0 OR expiration_time >= ?)
	`
	args := []any{clientID, dataType, clientToken, time.Now().Unix()}

	if !fetchFolders {
		query += ` AND folder = 0`
	}
	query += ` ORDER BY mtime ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	entities := make([]datastore.SyncEntity, 0, limit)
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return false, nil, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	hasChangesRemaining := len(entities) > maxSize
	if hasChangesRemaining {
		entities = entities[:maxSize]
	}
	return hasChangesRemaining, entities, nil
}

func (s *SQLiteStore) HasServerDefinedUniqueTag(ctx context.Context, clientID string, tag string) (bool, error) {
	return hasRow(ctx, s.db,
		`SELECT 1 FROM unique_tags WHERE client_id = ? AND id = ? LIMIT 1`,
		clientID,
		serverTagItemPrefix+tag,
	)
}

func (s *SQLiteStore) GetClientItemCount(ctx context.Context, clientID string) (*datastore.ClientItemCounts, error) {
	counts := &datastore.ClientItemCounts{
		ClientID: clientID,
		ID:       clientID,
	}

	var found bool
	err := s.db.QueryRowContext(ctx, `
		SELECT
			item_count,
			history_item_count_period1,
			history_item_count_period2,
			history_item_count_period3,
			history_item_count_period4,
			last_period_change_time,
			version
		FROM client_item_counts
		WHERE client_id = ?
	`, clientID).Scan(
		&counts.ItemCount,
		&counts.HistoryItemCountPeriod1,
		&counts.HistoryItemCountPeriod2,
		&counts.HistoryItemCountPeriod3,
		&counts.HistoryItemCountPeriod4,
		&counts.LastPeriodChangeTime,
		&counts.Version,
	)
	if err == nil {
		found = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if !found {
		counts.ItemCount = 0
		counts.HistoryItemCountPeriod1 = 0
		counts.HistoryItemCountPeriod2 = 0
		counts.HistoryItemCountPeriod3 = 0
		counts.HistoryItemCountPeriod4 = 0
		counts.LastPeriodChangeTime = 0
		counts.Version = 0
	}

	if err := s.initRealCountsAndUpdateHistoryCounts(ctx, counts); err != nil {
		return nil, err
	}

	return counts, nil
}

func (s *SQLiteStore) UpdateClientItemCount(ctx context.Context, counts *datastore.ClientItemCounts, newNormalItemCount int, newHistoryItemCount int) error {
	counts.HistoryItemCountPeriod4 += newHistoryItemCount
	counts.ItemCount += newNormalItemCount

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO client_item_counts (
			client_id,
			item_count,
			history_item_count_period1,
			history_item_count_period2,
			history_item_count_period3,
			history_item_count_period4,
			last_period_change_time,
			version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(client_id) DO UPDATE SET
			item_count = excluded.item_count,
			history_item_count_period1 = excluded.history_item_count_period1,
			history_item_count_period2 = excluded.history_item_count_period2,
			history_item_count_period3 = excluded.history_item_count_period3,
			history_item_count_period4 = excluded.history_item_count_period4,
			last_period_change_time = excluded.last_period_change_time,
			version = excluded.version
	`,
		counts.ClientID,
		counts.ItemCount,
		counts.HistoryItemCountPeriod1,
		counts.HistoryItemCountPeriod2,
		counts.HistoryItemCountPeriod3,
		counts.HistoryItemCountPeriod4,
		counts.LastPeriodChangeTime,
		counts.Version,
	)
	return err
}

func (s *SQLiteStore) ClearServerData(ctx context.Context, clientID string) ([]datastore.SyncEntity, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Acquire writer ownership before collecting invalidation metadata, so it
	// describes exactly the same snapshot that is cleared. Never load payloads.
	if _, err := tx.ExecContext(ctx, `DELETE FROM unique_tags WHERE client_id=?`, clientID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT client_id, id, data_type
		FROM sync_entities
		WHERE client_id = ?
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := []datastore.SyncEntity{}
	for rows.Next() {
		var entity datastore.SyncEntity
		var dataType sql.NullInt64
		if err := rows.Scan(&entity.ClientID, &entity.ID, &dataType); err != nil {
			return nil, err
		}
		if dataType.Valid {
			dt := int(dataType.Int64)
			entity.DataType = &dt
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM client_item_counts WHERE client_id = ?`,
		clientID,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM sync_entities WHERE client_id = ? AND id <> ?`,
		clientID,
		disabledChainID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *SQLiteStore) DisableSyncChain(ctx context.Context, clientID string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_entities (
			client_id, id, mtime, ctime, name
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(client_id, id) DO UPDATE SET
			mtime = excluded.mtime,
			ctime = excluded.ctime,
			name = excluded.name
	`, clientID, disabledChainID, now, now, reasonDeleted)
	return err
}

func (s *SQLiteStore) IsSyncChainDisabled(ctx context.Context, clientID string) (bool, error) {
	return hasRow(ctx, s.db,
		`SELECT 1 FROM sync_entities WHERE client_id = ? AND id = ? LIMIT 1`,
		clientID,
		disabledChainID,
	)
}

func (s *SQLiteStore) HasItem(ctx context.Context, clientID string, id string) (bool, error) {
	if err := s.retireExpired(ctx, clientID, &id, nil); err != nil {
		return false, err
	}
	return hasRow(ctx, s.db,
		`SELECT 1 FROM sync_entities WHERE client_id = ? AND id = ? LIMIT 1`,
		clientID,
		id,
	)
}

func (s *SQLiteStore) initRealCountsAndUpdateHistoryCounts(ctx context.Context, counts *datastore.ClientItemCounts) error {
	now := time.Now().Unix()
	if counts.Version < sqliteCountVersion {
		if counts.ItemCount > 0 {
			historyCount, err := countRows(ctx, s.db, `
				SELECT COUNT(1)
				FROM sync_entities
				WHERE client_id = ?
				  AND data_type IN (?, ?)
				  AND deleted = 0
				  AND (expiration_time IS NULL OR expiration_time = 0 OR expiration_time >= ?)
			`, counts.ClientID, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID, now)
			if err != nil {
				return err
			}

			normalCount, err := countRows(ctx, s.db, `
				SELECT COUNT(1)
				FROM sync_entities
				WHERE client_id = ?
				  AND data_type IS NOT NULL
				  AND data_type NOT IN (?, ?)
				  AND deleted = 0
			`, counts.ClientID, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID)
			if err != nil {
				return err
			}

			counts.HistoryItemCountPeriod1 = 0
			counts.HistoryItemCountPeriod2 = 0
			counts.HistoryItemCountPeriod3 = 0
			counts.HistoryItemCountPeriod4 = historyCount
			counts.ItemCount = normalCount
		}
		counts.LastPeriodChangeTime = now
		counts.Version = sqliteCountVersion
		return nil
	}

	timeSinceLastChange := now - counts.LastPeriodChangeTime
	if timeSinceLastChange >= periodDurationSecs {
		changeCount := int(timeSinceLastChange / periodDurationSecs)
		for i := 0; i < changeCount; i++ {
			counts.HistoryItemCountPeriod1 = counts.HistoryItemCountPeriod2
			counts.HistoryItemCountPeriod2 = counts.HistoryItemCountPeriod3
			counts.HistoryItemCountPeriod3 = counts.HistoryItemCountPeriod4
			counts.HistoryItemCountPeriod4 = 0
		}
		counts.LastPeriodChangeTime += periodDurationSecs * int64(changeCount)
	}
	return nil
}

func insertSyncEntity(ctx context.Context, tx *sql.Tx, entity *datastore.SyncEntity) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sync_entities (
			client_id, id, parent_id, version, mtime, ctime,
			name, non_unique_name, server_defined_unique_tag,
			deleted, originator_cache_guid, originator_client_item_id,
			specifics, data_type, folder, client_defined_unique_tag,
			unique_position, data_type_mtime, expiration_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entity.ClientID,
		entity.ID,
		nullableString(entity.ParentID),
		nullableInt64(entity.Version),
		nullableInt64(entity.Mtime),
		nullableInt64(entity.Ctime),
		nullableString(entity.Name),
		nullableString(entity.NonUniqueName),
		nullableString(entity.ServerDefinedUniqueTag),
		nullableBool(entity.Deleted),
		nullableString(entity.OriginatorCacheGUID),
		nullableString(entity.OriginatorClientItemID),
		entity.Specifics,
		nullableInt(entity.DataType),
		nullableBool(entity.Folder),
		nullableString(entity.ClientDefinedUniqueTag),
		entity.UniquePosition,
		nullableString(entity.DataTypeMtime),
		nullableInt64(entity.ExpirationTime),
	)
	return err
}

func scanEntity(scanner interface{ Scan(dest ...any) error }) (datastore.SyncEntity, error) {
	var entity datastore.SyncEntity
	var parentID, name, nonUniqueName, serverTag, originatorCacheGUID, originatorClientItemID, clientTag, dataTypeMtime sql.NullString
	var version, mtime, ctime, expirationTime sql.NullInt64
	var deletedRaw, folderRaw sql.NullInt64
	var specifics, uniquePosition []byte
	var dataTypeRaw sql.NullInt64

	err := scanner.Scan(
		&entity.ClientID,
		&entity.ID,
		&parentID,
		&version,
		&mtime,
		&ctime,
		&name,
		&nonUniqueName,
		&serverTag,
		&deletedRaw,
		&originatorCacheGUID,
		&originatorClientItemID,
		&specifics,
		&dataTypeRaw,
		&folderRaw,
		&clientTag,
		&uniquePosition,
		&dataTypeMtime,
		&expirationTime,
	)
	if err != nil {
		return datastore.SyncEntity{}, err
	}

	entity.ParentID = nullStringToPtr(parentID)
	entity.Version = nullInt64ToPtr(version)
	entity.Mtime = nullInt64ToPtr(mtime)
	entity.Ctime = nullInt64ToPtr(ctime)
	entity.Name = nullStringToPtr(name)
	entity.NonUniqueName = nullStringToPtr(nonUniqueName)
	entity.ServerDefinedUniqueTag = nullStringToPtr(serverTag)
	entity.Deleted = nullBoolToPtr(deletedRaw)
	entity.OriginatorCacheGUID = nullStringToPtr(originatorCacheGUID)
	entity.OriginatorClientItemID = nullStringToPtr(originatorClientItemID)
	entity.Specifics = specifics
	if dataTypeRaw.Valid {
		v := int(dataTypeRaw.Int64)
		entity.DataType = &v
	}
	entity.Folder = nullBoolToPtr(folderRaw)
	entity.ClientDefinedUniqueTag = nullStringToPtr(clientTag)
	if uniquePosition != nil {
		entity.UniquePosition = uniquePosition
	}
	entity.DataTypeMtime = nullStringToPtr(dataTypeMtime)
	entity.ExpirationTime = nullInt64ToPtr(expirationTime)

	return entity, nil
}

func countRows(ctx context.Context, db *sql.DB, query string, args ...any) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func hasRow(ctx context.Context, db *sql.DB, query string, args ...any) (bool, error) {
	var v int
	err := db.QueryRowContext(ctx, query, args...).Scan(&v)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func isConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "constraint")
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableBool(v *bool) any {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}
	return 0
}

func nullStringToPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func nullInt64ToPtr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func nullBoolToPtr(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
