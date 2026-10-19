package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brave/go-sync/datastore"
)

const sqliteTestBookmarkType = 32904

func TestSQLiteStoreConnectionPragmas(t *testing.T) {
	t.Logf("SQLite backend: %s (driver %s)", sqliteBackend, sqliteDriverName)
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "connections.db"))
	ctx := sqliteTestContext(t)
	s.db.SetMaxOpenConns(4)
	// Keep every connection checked out to force new physical connections,
	// rather than repeatedly testing the connection used by initSchema.
	for i := 0; i < 3; i++ {
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		var journal string
		var timeout, synchronous int
		if err := conn.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&synchronous); err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(journal, "wal") || timeout != 5000 || synchronous != 2 {
			t.Errorf("connection %d: journal_mode=%q busy_timeout=%d synchronous=%d, want wal/5000/FULL", i, journal, timeout, synchronous)
		}
	}
}

func TestSQLiteStoreMemoryPath(t *testing.T) {
	s := sqliteTestOpen(t, ":memory:")
	var journal string
	if err := s.db.QueryRowContext(sqliteTestContext(t), `PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if journal != "memory" {
		t.Fatalf(":memory: became a disk database with journal mode %q", journal)
	}
}

func TestSQLiteStoreFirstRunSchema(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "new.db"))
	ctx := sqliteTestContext(t)
	for table, columns := range map[string][]string{
		"sync_entities":      {"client_id", "id", "parent_id", "version", "mtime", "ctime", "name", "non_unique_name", "server_defined_unique_tag", "deleted", "originator_cache_guid", "originator_client_item_id", "specifics", "data_type", "folder", "client_defined_unique_tag", "unique_position", "data_type_mtime", "expiration_time"},
		"unique_tags":        {"client_id", "id", "mtime", "ctime"},
		"client_item_counts": {"client_id", "item_count", "history_item_count_period1", "history_item_count_period2", "history_item_count_period3", "history_item_count_period4", "last_period_change_time", "version"},
	} {
		rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var cid, notNull, pk int
			var name, columnType string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			got = append(got, name)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, columns) {
			t.Errorf("%s columns = %v, want %v", table, got, columns)
		}
		sqliteTestRowCount(t, s.db, table, "", 0)
	}
	var indexColumns []string
	for _, row := range sqliteTestQuery(t, s.db, `PRAGMA index_info(idx_sync_entities_updates)`) {
		indexColumns = append(indexColumns, row[2].(string))
	}
	if !reflect.DeepEqual(indexColumns, []string{"client_id", "data_type", "mtime"}) {
		t.Errorf("updates index columns = %v", indexColumns)
	}
	counts, err := s.GetClientItemCount(ctx, "new-chain")
	if err != nil {
		t.Fatal(err)
	}
	if counts.ClientID != "new-chain" || counts.ID != "new-chain" || counts.ItemCount != 0 || counts.HistoryItemCountPeriod4 != 0 || counts.Version != sqliteCountVersion {
		t.Fatalf("unexpected initial counts: %+v", counts)
	}
	sqliteTestDisabled(t, s, "new-chain", false)
	sqliteTestIntegrity(t, s.db)
}

func TestSQLiteStoreReopenPreservesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preserved.db")
	s := sqliteTestOpen(t, path)
	ctx := sqliteTestContext(t)
	full := sqliteTestEntity("chain-a", "same-id", 100)
	other := sqliteTestEntity("chain-b", "same-id", 200)
	other.Specifics = []byte{0x91, 0, 0xff, 0x80}
	sqliteTestInsert(t, s, full)
	sqliteTestInsert(t, s, other)
	wantCounts := datastore.ClientItemCounts{
		ClientID: "chain-a", ID: "chain-a", ItemCount: 17,
		HistoryItemCountPeriod1: 2, HistoryItemCountPeriod2: 3,
		HistoryItemCountPeriod3: 4, HistoryItemCountPeriod4: 5,
		LastPeriodChangeTime: time.Now().Unix() + periodDurationSecs, Version: sqliteCountVersion,
	}
	if err := s.UpdateClientItemCount(ctx, &wantCounts, 2, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.DisableSyncChain(ctx, "chain-b"); err != nil {
		t.Fatal(err)
	}
	before := sqliteTestSnapshot(t, s.db)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = sqliteTestOpen(t, path)
	if got := sqliteTestSnapshot(t, s.db); !reflect.DeepEqual(got, before) {
		t.Fatal("reopen changed stored logical contents")
	}
	sqliteTestUpdatesEqual(t, s, "chain-a", []datastore.SyncEntity{full})
	sqliteTestUpdatesEqual(t, s, "chain-b", []datastore.SyncEntity{other})
	gotCounts, err := s.GetClientItemCount(ctx, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*gotCounts, wantCounts) {
		t.Fatalf("counts = %+v, want %+v", gotCounts, wantCounts)
	}
	sqliteTestDisabled(t, s, "chain-a", false)
	sqliteTestDisabled(t, s, "chain-b", true)
	sqliteTestIntegrity(t, s.db)
}

func TestSQLiteStoreInsertConflictsAreAtomicAndChainScoped(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "conflicts.db"))
	ctx := sqliteTestContext(t)
	original := sqliteTestEntity("chain-a", "same-id", 100)
	sqliteTestInsert(t, s, original)
	sqliteTestInsert(t, s, sqliteTestEntity("chain-b", "same-id", 100))
	for _, tc := range []struct {
		name, id, tag string
	}{
		{"duplicate-id", original.ID, "uncommitted-tag"},
		{"duplicate-tag", "uncommitted-id", *original.ClientDefinedUniqueTag},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := sqliteTestSnapshot(t, s.db)
			duplicate := sqliteTestEntity("chain-a", tc.id, 200)
			duplicate.ClientDefinedUniqueTag = &tc.tag
			conflict, err := s.InsertSyncEntity(ctx, &duplicate)
			// command.Commit's insertion path only checks conflicts when err != nil.
			if !conflict || err == nil {
				t.Fatalf("insert conflict = (%v, %v), want true and non-nil error", conflict, err)
			}
			if got := sqliteTestSnapshot(t, s.db); !reflect.DeepEqual(got, before) {
				t.Fatal("conflicting insert changed entities or leaked a tag")
			}
		})
	}
	probe := sqliteTestEntity("chain-a", "uncommitted-id", 300)
	probe.ClientDefinedUniqueTag = sqliteTestPtr("uncommitted-tag")
	sqliteTestInsert(t, s, probe)
	sqliteTestUpdatesEqual(t, s, "chain-b", []datastore.SyncEntity{sqliteTestEntity("chain-b", "same-id", 100)})
}

func TestSQLiteStoreUpdateAndDeletion(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "updates.db"))
	ctx := sqliteTestContext(t)
	original := sqliteTestEntity("chain-a", "item", 100)
	sqliteTestInsert(t, s, original)
	other := sqliteTestEntity("chain-b", "item", 100)
	sqliteTestInsert(t, s, other)
	update := datastore.SyncEntity{
		ClientID: original.ClientID, ID: original.ID, DataType: original.DataType,
		Version: sqliteTestPtr(int64(2)), Mtime: sqliteTestPtr(int64(200)),
		Specifics: []byte{0, 0xfe, 0x80, 0x01}, DataTypeMtime: sqliteTestPtr("32904#200"),
		ParentID: sqliteTestPtr("new-parent"), Name: sqliteTestPtr("renamed"),
		Folder: sqliteTestPtr(true), UniquePosition: []byte{0xff, 0, 0x81},
	}
	before := sqliteTestSnapshot(t, s.db)
	conflict, deleted, err := s.UpdateSyncEntity(ctx, &update, 0)
	if err != nil || !conflict || deleted {
		t.Fatalf("stale update = (%v, %v, %v), want true, false, nil", conflict, deleted, err)
	}
	if !reflect.DeepEqual(sqliteTestSnapshot(t, s.db), before) {
		t.Fatal("stale update changed stored contents")
	}
	missing := update
	missing.ID = "missing"
	conflict, deleted, err = s.UpdateSyncEntity(ctx, &missing, 1)
	if err != nil || !conflict || deleted {
		t.Fatalf("missing update = (%v, %v, %v), want true, false, nil", conflict, deleted, err)
	}
	conflict, deleted, err = s.UpdateSyncEntity(ctx, &update, 1)
	if err != nil || conflict || deleted {
		t.Fatalf("valid update = (%v, %v, %v)", conflict, deleted, err)
	}
	want := original
	want.Version, want.Mtime, want.Specifics, want.DataTypeMtime = update.Version, update.Mtime, update.Specifics, update.DataTypeMtime
	want.ParentID, want.Name, want.Folder, want.UniquePosition = update.ParentID, update.Name, update.Folder, update.UniquePosition
	sqliteTestUpdatesEqual(t, s, "chain-a", []datastore.SyncEntity{want})
	update.Version, update.Mtime = sqliteTestPtr(int64(3)), sqliteTestPtr(int64(300))
	update.DataTypeMtime, update.Deleted = sqliteTestPtr("32904#300"), sqliteTestPtr(true)
	update.ClientDefinedUniqueTag = original.ClientDefinedUniqueTag
	conflict, deleted, err = s.UpdateSyncEntity(ctx, &update, 2)
	if err != nil || conflict || !deleted {
		t.Fatalf("delete = (%v, %v, %v), want false, true, nil", conflict, deleted, err)
	}
	want.Version, want.Mtime, want.DataTypeMtime, want.Deleted = update.Version, update.Mtime, update.DataTypeMtime, update.Deleted
	sqliteTestUpdatesEqual(t, s, "chain-a", []datastore.SyncEntity{want})
	conflict, deleted, err = s.UpdateSyncEntity(ctx, &update, 3)
	if err != nil || conflict || deleted {
		t.Fatalf("repeated delete = (%v, %v, %v), want false, false, nil", conflict, deleted, err)
	}
	// The tombstone stays fetchable, while its tag is available for a new item.
	replacement := sqliteTestEntity("chain-a", "replacement", 400)
	replacement.ClientDefinedUniqueTag = original.ClientDefinedUniqueTag
	sqliteTestInsert(t, s, replacement)
	sqliteTestUpdatesEqual(t, s, "chain-b", []datastore.SyncEntity{other})
}

func TestSQLiteStorePaginationAndFilters(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "pages.db"))
	ctx := sqliteTestContext(t)
	// Deliberately insert out of order and place filtered records between pages.
	for _, mtime := range []int64{300, 100, 200, 150, 175} {
		entity := sqliteTestEntity("chain-a", fmt.Sprintf("item-%d", mtime), mtime)
		if mtime == 150 {
			entity.Folder = sqliteTestPtr(true)
		}
		if mtime == 175 {
			entity.ExpirationTime = sqliteTestPtr(int64(1))
		}
		if mtime == 300 {
			entity.Deleted = sqliteTestPtr(true)
		}
		sqliteTestInsert(t, s, entity)
	}
	sqliteTestInsert(t, s, sqliteTestEntity("chain-b", "foreign", 125))
	otherType := sqliteTestEntity("chain-a", "other-type", 125)
	otherType.DataType = sqliteTestPtr(47745)
	sqliteTestInsert(t, s, otherType)
	for _, tc := range []struct {
		name         string
		token        int64
		fetchFolders bool
		limit        int
		more         bool
		ids          []string
	}{
		{"first", 0, false, 2, true, []string{"item-100", "item-200"}},
		{"second", 200, false, 2, false, []string{"item-300"}},
		{"exact-boundary", 100, false, 2, false, []string{"item-200", "item-300"}},
		{"last-token-exclusive", 300, false, 2, false, []string{}},
		{"with-folders", 0, true, 10, false, []string{"item-100", "item-150", "item-200", "item-300"}},
		{"zero-limit", 0, true, 0, true, []string{}},
		{"empty-chain", 1000, true, 1, false, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			more, entities, err := s.GetUpdatesForType(ctx, sqliteTestBookmarkType, tc.token, tc.fetchFolders, "chain-a", tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(entities))
			for _, entity := range entities {
				ids = append(ids, entity.ID)
			}
			if more != tc.more || !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("got more=%v ids=%v, want more=%v ids=%v", more, ids, tc.more, tc.ids)
			}
		})
	}
}

func TestSQLiteStoreServerTagBatchRollback(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "batch.db"))
	ctx := sqliteTestContext(t)
	root := sqliteTestEntity("chain-a", "root", 100)
	root.ServerDefinedUniqueTag = sqliteTestPtr("root-tag")
	if err := s.InsertSyncEntitiesWithServerTags(ctx, []*datastore.SyncEntity{&root}); err != nil {
		t.Fatal(err)
	}
	before := sqliteTestSnapshot(t, s.db)
	first := sqliteTestEntity("chain-a", "new-root", 200)
	duplicate := sqliteTestEntity("chain-a", "duplicate-root", 300)
	duplicate.ServerDefinedUniqueTag = root.ServerDefinedUniqueTag
	if err := s.InsertSyncEntitiesWithServerTags(ctx, []*datastore.SyncEntity{&first, &duplicate}); err == nil {
		t.Fatal("expected duplicate server tag to roll back the entire batch")
	}
	if !reflect.DeepEqual(sqliteTestSnapshot(t, s.db), before) {
		t.Fatal("failed batch left partial entities or tags")
	}
	root.ClientID = "chain-b"
	if err := s.InsertSyncEntitiesWithServerTags(ctx, []*datastore.SyncEntity{&root}); err != nil {
		t.Fatalf("same server tag in another chain: %v", err)
	}
}

func TestSQLiteStoreClearServerDataIsolation(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "clear.db"))
	ctx := sqliteTestContext(t)
	for _, chain := range []string{"chain-a", "chain-b"} {
		entity := sqliteTestEntity(chain, "same-id", 100)
		if err := s.InsertSyncEntitiesWithServerTags(ctx, []*datastore.SyncEntity{&entity}); err != nil {
			t.Fatal(err)
		}
		clientEntity := sqliteTestEntity(chain, "client-tagged", 200)
		sqliteTestInsert(t, s, clientEntity)
		counts := &datastore.ClientItemCounts{ClientID: chain, Version: sqliteCountVersion, LastPeriodChangeTime: time.Now().Unix()}
		if err := s.UpdateClientItemCount(ctx, counts, 2, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DisableSyncChain(ctx, "chain-a"); err != nil {
		t.Fatal(err)
	}
	foreignBefore := sqliteTestChainSnapshot(t, s.db, "chain-b")
	removed, err := s.ClearServerData(ctx, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, entity := range removed {
		if entity.ClientID != "chain-a" {
			t.Fatalf("clear returned another chain's entity: %q", entity.ClientID)
		}
		ids[entity.ID] = true
	}
	if !ids["same-id"] || !ids["client-tagged"] {
		t.Fatalf("clear did not return deleted records: %v", ids)
	}
	sqliteTestRowCount(t, s.db, "sync_entities", "chain-a", 1)
	sqliteTestRowCount(t, s.db, "unique_tags", "chain-a", 0)
	sqliteTestRowCount(t, s.db, "client_item_counts", "chain-a", 0)
	sqliteTestDisabled(t, s, "chain-a", true)
	if !reflect.DeepEqual(sqliteTestChainSnapshot(t, s.db, "chain-b"), foreignBefore) {
		t.Fatal("clear changed the other chain's entities, tags, or counts")
	}
	if _, err := s.ClearServerData(ctx, "unknown-chain"); err != nil {
		t.Fatalf("clear unknown chain: %v", err)
	}
	sqliteTestIntegrity(t, s.db)
}

func TestSQLiteStoreV02Compatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v0.2.db")
	// Seed without NewSQLiteStore/initSchema so this fixture can detect an
	// incompatible initializer, rather than regenerating the new schema.
	dsn, err := sqliteDSN(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open(sqliteDriverName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	fixture, err := os.ReadFile("testdata/v0.2.sql")
	if err != nil {
		t.Fatal(err)
	}
	ctx := sqliteTestContext(t)
	// The fixture has no semicolons in SQL literals. Execute one statement at
	// a time because drivers differ in their multi-statement Exec behavior.
	for _, statement := range strings.Split(string(fixture), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := raw.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed v0.2 fixture: %v", err)
		}
	}
	sqliteTestIntegrity(t, raw)
	before := sqliteTestSnapshot(t, raw)
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	s := sqliteTestOpen(t, path)
	if !reflect.DeepEqual(sqliteTestSnapshot(t, s.db), before) {
		t.Fatal("opening v0.2 changed schema or logical table contents")
	}
	more, records, err := s.GetUpdatesForType(ctx, sqliteTestBookmarkType, 0, true, "legacy-active", 10)
	if err != nil || more || len(records) != 3 {
		t.Fatalf("legacy updates: more=%v count=%d err=%v", more, len(records), err)
	}
	want := datastore.SyncEntity{
		ClientID: "legacy-active", ID: "full-record", ParentID: sqliteTestPtr("parent-0"),
		Version: sqliteTestPtr(int64(7)), Mtime: sqliteTestPtr(int64(100)), Ctime: sqliteTestPtr(int64(90)),
		Name: sqliteTestPtr("synthetic name"), NonUniqueName: sqliteTestPtr(""),
		ServerDefinedUniqueTag: sqliteTestPtr("legacy-root"), Deleted: sqliteTestPtr(false),
		OriginatorCacheGUID: sqliteTestPtr("synthetic-device"), OriginatorClientItemID: sqliteTestPtr("client-item-0"),
		Specifics: []byte{0, 0xff, 0x80, 0xa5, 1, 0x7f, 0, 0x0d, 0x0a},
		DataType:  sqliteTestPtr(sqliteTestBookmarkType), Folder: sqliteTestPtr(false), ClientDefinedUniqueTag: sqliteTestPtr("legacy-tag"),
		UniquePosition: []byte{0xff, 0, 0x80, 0x7f}, DataTypeMtime: sqliteTestPtr("32904#100"), ExpirationTime: sqliteTestPtr(int64(0)),
	}
	if !reflect.DeepEqual(records[0], want) {
		t.Fatalf("legacy full record = %+v, want %+v", records[0], want)
	}
	if records[1].ID != "nullable-record" || records[1].Name != nil || records[1].Ctime != nil || records[1].Deleted != nil || records[1].Specifics != nil || records[1].Folder != nil {
		t.Fatalf("legacy NULL fields changed: %+v", records[1])
	}
	if records[2].ID != "tombstone" || records[2].Deleted == nil || !*records[2].Deleted || !bytes.Equal(records[2].Specifics, []byte{}) {
		t.Fatalf("legacy tombstone changed: %+v", records[2])
	}
	more, history, err := s.GetUpdatesForType(ctx, datastore.HistoryTypeID, 0, true, "legacy-active", 10)
	if err != nil || more || len(history) != 1 || history[0].ID != "live-history" || !bytes.Equal(history[0].Specifics, []byte{0x80, 0xff, 0}) {
		t.Fatalf("legacy history filtering: more=%v records=%+v err=%v", more, history, err)
	}
	counts, err := s.GetClientItemCount(ctx, "legacy-active")
	wantCounts := datastore.ClientItemCounts{ClientID: "legacy-active", ID: "legacy-active", ItemCount: 2, HistoryItemCountPeriod1: 3, HistoryItemCountPeriod2: 4, HistoryItemCountPeriod3: 5, HistoryItemCountPeriod4: 6, LastPeriodChangeTime: 4102444800, Version: 2}
	if err != nil || counts == nil || !reflect.DeepEqual(*counts, wantCounts) {
		t.Fatalf("legacy counts = %+v, err=%v, want %+v", counts, err, wantCounts)
	}
	sqliteTestDisabled(t, s, "legacy-active", false)
	sqliteTestDisabled(t, s, "legacy-disabled", true)
	hasTag, err := s.HasServerDefinedUniqueTag(ctx, "legacy-active", "legacy-root")
	if err != nil || !hasTag {
		t.Fatalf("legacy server tag = %v, err=%v", hasTag, err)
	}
	if !reflect.DeepEqual(sqliteTestSnapshot(t, s.db), before) {
		t.Fatal("reading v0.2 records changed stored logical contents")
	}
	// Exercise writes and another restart without altering the legacy chains.
	sqliteTestInsert(t, s, sqliteTestEntity("new-chain", "new-item", 900))
	if err := s.CheckpointTruncate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = sqliteTestOpen(t, path)
	if !reflect.DeepEqual(sqliteTestChainSnapshot(t, s.db, "legacy-active"), before["legacy-active"]) || !reflect.DeepEqual(sqliteTestChainSnapshot(t, s.db, "legacy-disabled"), before["legacy-disabled"]) {
		t.Fatal("write/restart changed legacy chains")
	}
	sqliteTestUpdatesEqual(t, s, "new-chain", []datastore.SyncEntity{sqliteTestEntity("new-chain", "new-item", 900)})
	sqliteTestIntegrity(t, s.db)
}

func TestSQLiteStoreCheckpointReportsBusy(t *testing.T) {
	s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "checkpoint.db"))
	ctx := sqliteTestContext(t)
	s.db.SetMaxOpenConns(2)
	sqliteTestInsert(t, s, sqliteTestEntity("chain-a", "before-reader", 100))
	reader, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	sqliteTestInsert(t, s, sqliteTestEntity("chain-a", "after-reader", 200))
	writer, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Only the idle writer can run the checkpoint. Shorten its timeout so a
	// real SQLITE_BUSY result is deterministic without a five-second test delay.
	if _, err := writer.ExecContext(ctx, `PRAGMA busy_timeout = 1`); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckpointTruncate(ctx); err == nil || !strings.Contains(strings.ToLower(err.Error()), "busy") {
		t.Fatalf("checkpoint with an older reader snapshot = %v, want busy error", err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("checkpoint exhausted the context instead of reporting busy: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckpointTruncate(ctx); err != nil {
		t.Fatalf("checkpoint after releasing reader: %v", err)
	}
	sqliteTestIntegrity(t, s.db)
}

func TestSQLiteStoreEscapedFilePaths(t *testing.T) {
	for _, filename := range []string{"with spaces.db", "question?mode=memory.db", "fragment#name.db", "percent%23name.db"} {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), filename)
			s := sqliteTestOpen(t, path)
			entity := sqliteTestEntity("path-chain", "item", 100)
			sqliteTestInsert(t, s, entity)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("database was not created at the literal path: %v", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s = sqliteTestOpen(t, path)
			sqliteTestUpdatesEqual(t, s, "path-chain", []datastore.SyncEntity{entity})
		})
	}
}

func sqliteTestPtr[T any](v T) *T { return &v }

func sqliteTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func sqliteTestOpen(t *testing.T, path string) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open synthetic database: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close synthetic database: %v", err)
		}
	})
	return s
}

func sqliteTestEntity(chain, id string, mtime int64) datastore.SyncEntity {
	// Opaque, encrypted-looking fixtures exercise BLOB bytes without relying
	// on protobuf decoding, real keys, user records, or randomness.
	return datastore.SyncEntity{
		ClientID: chain, ID: id, ParentID: sqliteTestPtr("parent-0"),
		Version: sqliteTestPtr(int64(1)), Mtime: &mtime, Ctime: sqliteTestPtr(int64(90)),
		Name: sqliteTestPtr("synthetic name"), NonUniqueName: sqliteTestPtr(""),
		ServerDefinedUniqueTag: sqliteTestPtr("server-" + id), Deleted: sqliteTestPtr(false),
		OriginatorCacheGUID: sqliteTestPtr("synthetic-device"), OriginatorClientItemID: sqliteTestPtr("local-" + id),
		Specifics: []byte{0, 0xff, 0x80, 0xa5, 1, 0x7f, 0, 0x0d, 0x0a},
		DataType:  sqliteTestPtr(sqliteTestBookmarkType), Folder: sqliteTestPtr(false),
		ClientDefinedUniqueTag: sqliteTestPtr("client-" + id), UniquePosition: []byte{0xff, 0, 0x80, 0x7f},
		DataTypeMtime: sqliteTestPtr(fmt.Sprintf("32904#%d", mtime)), ExpirationTime: sqliteTestPtr(int64(0)),
	}
}

func sqliteTestInsert(t *testing.T, s *SQLiteStore, entity datastore.SyncEntity) {
	t.Helper()
	conflict, err := s.InsertSyncEntity(sqliteTestContext(t), &entity)
	if err != nil || conflict {
		t.Fatalf("insert %s/%s: conflict=%v err=%v", entity.ClientID, entity.ID, conflict, err)
	}
}

func sqliteTestUpdatesEqual(t *testing.T, s *SQLiteStore, chain string, want []datastore.SyncEntity) {
	t.Helper()
	more, got, err := s.GetUpdatesForType(sqliteTestContext(t), sqliteTestBookmarkType, 0, true, chain, 100)
	if err != nil || more || !reflect.DeepEqual(got, want) {
		t.Fatalf("updates for %s: more=%v err=%v\ngot:  %+v\nwant: %+v", chain, more, err, got, want)
	}
}

func sqliteTestDisabled(t *testing.T, s *SQLiteStore, chain string, want bool) {
	t.Helper()
	got, err := s.IsSyncChainDisabled(sqliteTestContext(t), chain)
	if err != nil || got != want {
		t.Fatalf("disabled %s = %v, err=%v, want %v", chain, got, err, want)
	}
}

func sqliteTestRowCount(t *testing.T, db *sql.DB, table, chain string, want int) {
	t.Helper()
	query := `SELECT COUNT(*) FROM ` + table
	var args []any
	if chain != "" {
		query += ` WHERE client_id = ?`
		args = append(args, chain)
	}
	var got int
	if err := db.QueryRowContext(sqliteTestContext(t), query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("%s/%s row count = %d, want %d", table, chain, got, want)
	}
}

func sqliteTestIntegrity(t *testing.T, db *sql.DB) {
	t.Helper()
	got := sqliteTestQuery(t, db, `PRAGMA integrity_check`)
	if !reflect.DeepEqual(got, [][]any{{"ok"}}) {
		t.Fatalf("integrity_check = %v, want ok", got)
	}
}

func sqliteTestQuery(t *testing.T, db *sql.DB, query string, args ...any) [][]any {
	t.Helper()
	rows, err := db.QueryContext(sqliteTestContext(t), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	result := make([][]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		for i, value := range values {
			if blob, ok := value.([]byte); ok {
				values[i] = bytes.Clone(blob)
			}
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func sqliteTestChainSnapshot(t *testing.T, db *sql.DB, chain string) map[string][][]any {
	t.Helper()
	return map[string][][]any{
		"sync_entities":      sqliteTestQuery(t, db, `SELECT * FROM sync_entities WHERE client_id = ? ORDER BY id`, chain),
		"unique_tags":        sqliteTestQuery(t, db, `SELECT * FROM unique_tags WHERE client_id = ? ORDER BY id`, chain),
		"client_item_counts": sqliteTestQuery(t, db, `SELECT * FROM client_item_counts WHERE client_id = ?`, chain),
	}
}

func sqliteTestSnapshot(t *testing.T, db *sql.DB) map[string]any {
	t.Helper()
	result := map[string]any{
		"schema": sqliteTestQuery(t, db, `SELECT type, name, tbl_name, sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`),
	}
	for _, row := range sqliteTestQuery(t, db, `SELECT client_id FROM sync_entities UNION SELECT client_id FROM unique_tags UNION SELECT client_id FROM client_item_counts ORDER BY client_id`) {
		chain := row[0].(string)
		result[chain] = sqliteTestChainSnapshot(t, db, chain)
	}
	return result
}
