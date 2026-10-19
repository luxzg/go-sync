package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/datastore"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

const reconciliationSessionType = 50119

const reconciliationColumns = `client_id, id, parent_id, version, mtime, ctime,
	name, non_unique_name, server_defined_unique_tag, deleted, originator_cache_guid,
	originator_client_item_id, specifics, data_type, folder, client_defined_unique_tag,
	unique_position, data_type_mtime, expiration_time`

// These experiments are deliberately outside the service and ordinary test
// package. Only scripts/test-session-reconciliation.sh injects this file.
func TestSessionReconciliationExperimentRetry(t *testing.T) {
	for _, retryState := range []string{"live", "omitted", "deleted"} {
		t.Run(retryState, func(t *testing.T) {
			deleteOnRetry := retryState == "deleted"
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			entries := reconciliationSeed(t, h, token, chain)
			before := reconciliationOriginals(t, h.db)
			metadata := reconciliationMetadata(t, h.db)
			for _, response := range protocolCommit(t, h, token, "synthetic-phone", entries...) {
				if response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatal("missing positive-version IDs must conflict before reconciliation")
				}
			}
			if !bytes.Equal(before, reconciliationOriginals(t, h.db)) {
				t.Fatal("conflicting retries modified original entity rows")
			}
			if err := reconciliationStage(context.Background(), h.db, chain, entries); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(metadata, reconciliationMetadata(t, h.db)) {
				t.Fatal("staging changed reservations or non-normal-count metadata")
			}
			for i, oldRows := range []int{5, 3} {
				var stagedBytes, donorBytes []byte
				if err := h.db.db.QueryRow(`SELECT specifics FROM sync_entities WHERE client_id = ? AND id = ?`,
					chain, entries[i].GetIdString()).Scan(&stagedBytes); err != nil {
					t.Fatal(err)
				}
				if err := h.db.db.QueryRow(`SELECT specifics FROM sync_entities WHERE client_id = ? AND id = ?`,
					chain, fmt.Sprintf("synthetic-old-%d-%d", i, oldRows-1)).Scan(&donorBytes); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(stagedBytes, donorBytes) {
					t.Fatal("staging did not preserve the latest donor's exact opaque bytes")
				}
			}
			counts, err := h.db.GetClientItemCount(context.Background(), chain)
			if err != nil || counts.ItemCount != 3 {
				t.Fatalf("staged identities must be counted: %+v, %v", counts, err)
			}
			// Simulate a service restart after the offline transaction. The helper
			// owns this entire disposable DB, never an installed service's file.
			var path string
			if err := h.db.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
				t.Fatal(err)
			}
			if err := h.db.Close(); err != nil {
				t.Fatal(err)
			}
			h.db, err = NewSQLiteStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer h.db.Close()
			h.handler = commandHandler(cache.NewCache(NewMemoryRedis()), h.db)
			for _, marker := range []int64{math.MinInt64, -1, 0, 1000} {
				_, rows, err := h.db.GetUpdatesForType(context.Background(), reconciliationSessionType, marker, true, chain, 500)
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range rows {
					if row.ID == entries[0].GetIdString() || row.ID == entries[1].GetIdString() {
						t.Fatal("staged contents escaped into downloads before an authenticated retry")
					}
				}
			}
			initial := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, reconciliationSessionType)
			if len(initial.Entries) != 8 {
				t.Fatal("initial protocol download must contain only the original tombstones")
			}
			for _, fetchFolders := range []*bool{nil, boolPtr(false), boolPtr(true)} {
				h.handler = commandHandler(cache.NewCache(NewMemoryRedis()), h.db)
				contents := sync_pb.ClientToServerMessage_GET_UPDATES
				poll := &sync_pb.ClientToServerMessage{
					MessageContents: &contents, Share: stringPtr(""),
					GetUpdates: &sync_pb.GetUpdatesMessage{
						FetchFolders: fetchFolders, GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_PERIODIC),
						FromProgressMarker: initial.NewProgressMarker,
					},
				}
				got := doAuthedRequest(t, h, token, mustMarshal(t, poll), false).GetUpdates
				if got == nil || len(got.Entries) != 0 {
					t.Fatal("pre-retry incremental protocol poll exposed staged rows")
				}
			}
			if err := reconciliationStage(context.Background(), h.db, chain, entries); err == nil {
				t.Fatal("repeated staging must refuse existing IDs")
			}
			staged := upgradeSnapshot(t, context.Background(), h.db.db)
			for _, entry := range entries {
				wrongBase := proto.Clone(entry).(*sync_pb.SyncEntity)
				wrongBase.Version = int64Ptr(entry.GetVersion() + 1)
				if response := protocolCommit(t, h, token, "synthetic-phone", wrongBase)[0]; response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatal("wrong base version must conflict even after staging")
				}
				var modifiedTags int
				if err := h.db.db.QueryRow(`SELECT COUNT(*) FROM unique_tags
					WHERE client_id = ? AND id = ? AND mtime = 1 AND ctime = 1`, chain,
					clientTagItemPrefix+entry.GetClientTagHash()).Scan(&modifiedTags); err != nil || modifiedTags != 1 {
					t.Fatal("staging modified the original tag reservation")
				}
			}
			if !bytes.Equal(staged, upgradeSnapshot(t, context.Background(), h.db.db)) {
				t.Fatal("wrong-base conflicts changed staged rows or counts")
			}
			for _, entry := range entries {
				entry.Deleted = boolPtr(deleteOnRetry)
				if retryState == "omitted" {
					entry.Deleted = nil
				}
			}
			responses := protocolCommit(t, h, token, "synthetic-phone", entries...)
			for i, response := range responses {
				protocolRequireCommitSuccess(t, response)
				if response.GetIdString() != entries[i].GetIdString() || response.GetVersion() <= entries[i].GetVersion() {
					t.Fatal("normal retry must preserve the client identity and advance its version")
				}
			}
			contents := sync_pb.ClientToServerMessage_GET_UPDATES
			poll := &sync_pb.ClientToServerMessage{
				MessageContents: &contents, Share: stringPtr(""),
				GetUpdates: &sync_pb.GetUpdatesMessage{
					FetchFolders: boolPtr(true), GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_PERIODIC),
					FromProgressMarker: initial.NewProgressMarker,
				},
			}
			updates := doAuthedRequest(t, h, token, mustMarshal(t, poll), false).GetUpdates
			if updates == nil || len(updates.Entries) != 2 {
				t.Fatal("peer must receive exactly the two retried identities incrementally")
			}
			byID := make(map[string]*sync_pb.SyncEntity)
			for _, entry := range entries {
				byID[entry.GetIdString()] = entry
			}
			for _, got := range updates.Entries {
				want := byID[got.GetIdString()]
				if want == nil || !proto.Equal(got.Specifics, want.Specifics) || got.GetDeleted() != deleteOnRetry {
					t.Fatal("retry did not replace staged specifics with the actual opaque client payload")
				}
			}
			for _, response := range protocolCommit(t, h, token, "synthetic-phone", entries...) {
				if response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatal("ordinary stale-version checks must remain effective")
				}
			}
			if !bytes.Equal(before, reconciliationOriginals(t, h.db)) {
				t.Fatal("reconciliation or retry modified old tombstones or unrelated entity rows")
			}
			if !deleteOnRetry {
				if !bytes.Equal(metadata, reconciliationMetadata(t, h.db)) {
					t.Fatal("live retry changed reservations or non-normal-count metadata")
				}
				duplicate := makeCommitEntity("synthetic-competing-id", entries[0].Specifics)
				duplicate.ClientTagHash = entries[0].ClientTagHash
				stable := upgradeSnapshot(t, context.Background(), h.db.db)
				if response := protocolCommit(t, h, token, "synthetic-peer", duplicate)[0]; response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatal("same-tag competitor must still conflict after live retry")
				}
				if !bytes.Equal(stable, upgradeSnapshot(t, context.Background(), h.db.db)) {
					t.Fatal("same-tag competitor changed rows or counts")
				}
			}
			counts, err = h.db.GetClientItemCount(context.Background(), chain)
			wantCount := 3
			if deleteOnRetry {
				wantCount = 1
			}
			if err != nil || counts.ItemCount != wantCount || counts.SumHistoryCounts() != 10 {
				t.Fatalf("retry changed unrelated history counts or has wrong normal count: %+v, %v", counts, err)
			}
			if !deleteOnRetry {
				for i, entry := range entries {
					entry.Version = int64Ptr(responses[i].GetVersion())
					entry.Deleted = boolPtr(true)
					updateTestWaitAfter(t, responses[i].GetVersion())
					protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "synthetic-phone", entry)[0])
				}
			}
			counts, err = h.db.GetClientItemCount(context.Background(), chain)
			if err != nil || counts.ItemCount != 1 {
				t.Fatalf("normal cleanup must remove exactly two live counts: %+v, %v", counts, err)
			}
			if !bytes.Equal(before, reconciliationOriginals(t, h.db)) {
				t.Fatal("cleanup modified original tombstones or unrelated entity rows")
			}
			for _, entry := range entries {
				var remaining int
				if err := h.db.db.QueryRow(`SELECT COUNT(*) FROM unique_tags WHERE client_id = ? AND id = ?`,
					chain, clientTagItemPrefix+entry.GetClientTagHash()).Scan(&remaining); err != nil || remaining != 0 {
					t.Fatalf("normal deletion must release the reservation: %d, %v", remaining, err)
				}
			}
			var integrity string
			if err := h.db.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("integrity=%s, %v", integrity, err)
			}
		})
	}
}

func TestSessionReconciliationExperimentInheritedTagRelease(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	entries := reconciliationSeed(t, h, token, chain)
	if err := reconciliationStage(context.Background(), h.db, chain, entries); err != nil {
		t.Fatal(err)
	}
	// Characterize an inherited limitation, not a repair guarantee: deleting
	// another same-tag tombstone can release the active identity's reservation.
	oldDeletion := makeCommitEntity("synthetic-old-0-4", entries[0].Specifics)
	oldDeletion.Version = int64Ptr(204)
	oldDeletion.Deleted = boolPtr(true)
	oldDeletion.ClientTagHash = entries[0].ClientTagHash
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "synthetic-peer", oldDeletion)[0])
	var reservationCount, activeCount int
	if err := h.db.db.QueryRow(`SELECT COUNT(*) FROM unique_tags WHERE client_id = ? AND id = ?`,
		chain, clientTagItemPrefix+entries[0].GetClientTagHash()).Scan(&reservationCount); err != nil {
		t.Fatal(err)
	}
	if err := h.db.db.QueryRow(`SELECT COUNT(*) FROM sync_entities WHERE client_id = ? AND id = ? AND deleted = 0`,
		chain, entries[0].GetIdString()).Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if reservationCount != 0 || activeCount != 1 {
		t.Fatal("inherited reservation ownership behavior changed; re-review recovery assumptions")
	}
}

func TestSessionReconciliationExperimentKnownCountDrift(t *testing.T) {
	for _, stored := range []int{0, 1, 9} {
		t.Run(fmt.Sprintf("stored=%d", stored), func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			entries := reconciliationSeed(t, h, token, chain)
			// Real new-client initialization adds five internal roots without
			// incrementing the normal counter. All identities here are synthetic.
			protocolGetUpdates(t, h, token, sync_pb.SyncEnums_NEW_CLIENT, testBookmarkTypeID, testNigoriTypeID)
			if _, err := h.db.db.Exec(`UPDATE client_item_counts SET item_count = ? WHERE client_id = ?`, stored, chain); err != nil {
				t.Fatal(err)
			}
			baseline := reconciliationExpectedCounts{Stored: stored, Live: 6}
			originals := reconciliationOriginals(t, h.db)
			metadata := reconciliationMetadata(t, h.db)
			snapshot := upgradeSnapshot(t, context.Background(), h.db.db)
			if err := reconciliationStage(context.Background(), h.db, chain, entries); err == nil {
				t.Fatal("unknown count drift must still be refused")
			}
			for _, wrong := range []reconciliationExpectedCounts{
				{Stored: stored + 1, Live: 6},
				{Stored: stored, Live: 7},
				{Stored: stored + 1, Live: 7},
			} {
				if err := reconciliationStageAtCounts(context.Background(), h.db, chain, entries, &wrong); err == nil {
					t.Fatal("changed baseline must be refused, even if its difference is unchanged")
				}
			}
			if !bytes.Equal(snapshot, upgradeSnapshot(t, context.Background(), h.db.db)) {
				t.Fatal("baseline refusals changed stored data")
			}
			if err := reconciliationStageAtCounts(context.Background(), h.db, chain, entries, &baseline); err != nil {
				t.Fatal(err)
			}
			assertDelta := func() {
				t.Helper()
				var counted, observed int
				if err := h.db.db.QueryRow(`SELECT item_count FROM client_item_counts WHERE client_id = ?`, chain).Scan(&counted); err != nil {
					t.Fatal(err)
				}
				if err := h.db.db.QueryRow(`SELECT COUNT(*) FROM sync_entities WHERE client_id = ? AND deleted = 0
					AND data_type IS NOT NULL AND data_type NOT IN (?, ?)`, chain,
					datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID).Scan(&observed); err != nil {
					t.Fatal(err)
				}
				if counted != stored+2 || observed != 8 || observed-counted != baseline.Live-baseline.Stored {
					t.Fatalf("must preserve the existing difference, not recount: stored=%d live=%d", counted, observed)
				}
				if !bytes.Equal(originals, reconciliationOriginals(t, h.db)) || !bytes.Equal(metadata, reconciliationMetadata(t, h.db)) {
					t.Fatal("Sessions staging/retry changed original rows or unrelated metadata")
				}
			}
			assertDelta()
			for _, response := range protocolCommit(t, h, token, "synthetic-phone", entries...) {
				protocolRequireCommitSuccess(t, response)
			}
			assertDelta()
		})
	}
}

func TestSessionReconciliationExperimentRefusals(t *testing.T) {
	for _, scenario := range []string{"existing-id", "live-tag", "wrong-type", "null-deleted", "no-tag", "no-reservation", "wrong-chain", "counts-differ", "disabled", "duplicate-plan", "second-invalid", "folder", "expiry", "bad-specifics", "unique-position"} {
		t.Run(scenario, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			entries := reconciliationSeed(t, h, token, chain)
			var query string
			switch scenario {
			case "existing-id":
				entries[0].IdString = stringPtr("synthetic-old-0-0")
			case "live-tag":
				query = `UPDATE sync_entities SET deleted = 0 WHERE id = 'synthetic-old-0-0'`
			case "wrong-type":
				query = `UPDATE sync_entities SET data_type = 32904 WHERE id = 'synthetic-old-0-0'`
			case "null-deleted":
				query = `UPDATE sync_entities SET deleted = NULL WHERE id = 'synthetic-old-0-0'`
			case "no-tag":
				entries[0].ClientTagHash = stringPtr("synthetic-absent-tag")
			case "no-reservation":
				query = `DELETE FROM unique_tags`
			case "wrong-chain":
				chain = "synthetic-other-chain"
			case "counts-differ":
				query = `UPDATE client_item_counts SET item_count = item_count + 1`
			case "disabled":
				if err := h.db.DisableSyncChain(context.Background(), chain); err != nil {
					t.Fatal(err)
				}
			case "duplicate-plan":
				entries[1] = entries[0]
			case "second-invalid":
				entries[1].Version = int64Ptr(0)
			case "folder":
				query = `UPDATE sync_entities SET folder = 1 WHERE id = 'synthetic-old-0-0'`
			case "expiry":
				query = `UPDATE sync_entities SET expiration_time = 1 WHERE id = 'synthetic-old-0-0'`
			case "bad-specifics":
				query = `UPDATE sync_entities SET specifics = X'FF' WHERE id = 'synthetic-old-0-0'`
			case "unique-position":
				query = `UPDATE sync_entities SET unique_position = X'FF' WHERE id = 'synthetic-old-0-0'`
			}
			if query != "" {
				if _, err := h.db.db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "live-tag" {
				if _, err := h.db.db.Exec(`UPDATE client_item_counts SET item_count = item_count + 1 WHERE client_id = ?`, chain); err != nil {
					t.Fatal(err)
				}
			}
			before := upgradeSnapshot(t, context.Background(), h.db.db)
			err := reconciliationStage(context.Background(), h.db, chain, entries)
			if err == nil {
				t.Fatal("unsafe/ambiguous candidate was not refused")
			}
			if scenario == "live-tag" && !strings.Contains(err.Error(), "only deleted") {
				t.Fatalf("expected live donor refusal, not an unrelated guard: %v", err)
			}
			if !bytes.Equal(before, upgradeSnapshot(t, context.Background(), h.db.db)) {
				t.Fatal("refusal changed the database; all candidate writes must roll back")
			}
		})
	}
}

func reconciliationSeed(t *testing.T, h *testHandler, token, chain string) []*sync_pb.SyncEntity {
	t.Helper()
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "synthetic-desktop",
		makeCommitEntity("synthetic-unrelated-bookmark", makeBookmarkSpecifics()))[0])
	entries := make([]*sync_pb.SyncEntity, 2)
	for i, oldRows := range []int{5, 3} {
		tag := fmt.Sprintf("synthetic-session-tag-%d", i)
		entries[i] = makeCommitEntity(fmt.Sprintf("synthetic-missing-%d", i),
			updateTestEncryptedSpecifics(&sync_pb.EntitySpecifics{
				SpecificsVariant: &sync_pb.EntitySpecifics_Session{Session: &sync_pb.SessionSpecifics{}},
			}, fmt.Sprintf("synthetic-opaque-client-payload-%d", i)))
		entries[i].Version = int64Ptr(int64(100 + i))
		entries[i].Ctime = int64Ptr(int64(50 + i))
		entries[i].ClientTagHash = stringPtr(tag)
		for j := 0; j < oldRows; j++ {
			if _, err := h.db.db.Exec(`INSERT INTO sync_entities
				(client_id, id, version, mtime, ctime, deleted, folder, data_type, client_defined_unique_tag, specifics)
				VALUES (?, ?, ?, ?, 1, 1, 0, 50119, ?, ?)`, chain, fmt.Sprintf("synthetic-old-%d-%d", i, j),
				int64(200+j), int64(200+j), tag, mustMarshal(t, updateTestEncryptedSpecifics(&sync_pb.EntitySpecifics{
					SpecificsVariant: &sync_pb.EntitySpecifics_Session{Session: &sync_pb.SessionSpecifics{}},
				}, fmt.Sprintf("synthetic-opaque-donor-%d-%d", i, j)))); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := h.db.db.Exec(`INSERT INTO unique_tags (client_id, id, mtime, ctime) VALUES (?, ?, 1, 1)`,
			chain, clientTagItemPrefix+tag); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.db.db.Exec(`INSERT INTO sync_entities
		(client_id, id, version, mtime, deleted, folder, data_type, specifics)
		VALUES ('synthetic-other-chain', 'synthetic-unrelated', 1, 1, 0, 0, 32904, ?)`,
		mustMarshal(t, makeBookmarkSpecifics())); err != nil {
		t.Fatal(err)
	}
	// History counters are a rolling estimate, not a direct count of retained
	// history rows. They must not be recalculated by a Sessions-only experiment.
	if _, err := h.db.db.Exec(`UPDATE client_item_counts SET history_item_count_period1 = 1,
		history_item_count_period2 = 2, history_item_count_period3 = 3, history_item_count_period4 = 4
		WHERE client_id = ?`, chain); err != nil {
		t.Fatal(err)
	}
	return entries
}

// This is an experiment, NOT a callable production repair function. It operates
// only on the test-owned store. MinInt64 is below or equal to every possible
// progress token, so strict mtime > token cannot advertise staged contents.
func reconciliationStage(ctx context.Context, store *SQLiteStore, chain string, entries []*sync_pb.SyncEntity) error {
	return reconciliationStageAtCounts(ctx, store, chain, entries, nil)
}

type reconciliationExpectedCounts struct {
	Stored int
	Live   int
}

func reconciliationStageAtCounts(ctx context.Context, store *SQLiteStore, chain string, entries []*sync_pb.SyncEntity, expected *reconciliationExpectedCounts) error {
	if len(entries) == 0 || len(entries) > 2 || chain == "" {
		return errors.New("invalid experiment scope")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var disabled int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities WHERE client_id = ? AND id = ?`, chain, disabledChainID).Scan(&disabled); err != nil || disabled != 0 {
		return errors.New("disabled or unreadable chain")
	}
	var count, countVersion, live int
	if err := tx.QueryRowContext(ctx, `SELECT item_count, version FROM client_item_counts WHERE client_id = ?`, chain).Scan(&count, &countVersion); err != nil || countVersion != sqliteCountVersion || count < 0 {
		return errors.New("missing or unsupported counts")
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities WHERE client_id = ? AND deleted = 0
		AND data_type IS NOT NULL AND data_type NOT IN (?, ?)`, chain, datastore.HistoryTypeID, datastore.HistoryDeleteDirectiveTypeID).Scan(&live); err != nil {
		return err
	}
	if expected == nil && live != count {
		return errors.New("unreviewed count mismatch needs separate investigation")
	}
	if expected != nil && (count != expected.Stored || live != expected.Live) {
		return errors.New("reviewed count baseline changed")
	}
	seenID, seenTag := make(map[string]bool), make(map[string]bool)
	for _, entry := range entries {
		if entry == nil {
			return errors.New("missing planned identity")
		}
		id, tag := entry.GetIdString(), entry.GetClientTagHash()
		if id == "" || tag == "" || seenID[id] || seenTag[tag] || entry.GetVersion() <= 0 || entry.GetCtime() <= 0 || entry.GetDeleted() || entry.GetFolder() || entry.Specifics.GetSession() == nil {
			return errors.New("invalid or repeated identity")
		}
		seenID[id], seenTag[tag] = true, true
		var found int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_entities WHERE client_id = ? AND id = ?`, chain, id).Scan(&found); err != nil || found != 0 {
			return errors.New("target identity already exists")
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM unique_tags WHERE client_id = ? AND id = ?`, chain, clientTagItemPrefix+tag).Scan(&found); err != nil || found != 1 {
			return errors.New("missing tag reservation")
		}
		rows, err := tx.QueryContext(ctx, `SELECT `+reconciliationColumns+` FROM sync_entities
			WHERE client_id = ? AND client_defined_unique_tag = ? ORDER BY mtime DESC, id`, chain, tag)
		if err != nil {
			return err
		}
		var template *datastore.SyncEntity
		for rows.Next() {
			row, scanErr := scanEntity(rows)
			if scanErr != nil || row.DataType == nil || *row.DataType != reconciliationSessionType ||
				row.Deleted == nil || !*row.Deleted || row.Folder == nil || *row.Folder || row.ServerDefinedUniqueTag != nil ||
				(row.ExpirationTime != nil && *row.ExpirationTime != 0) || row.UniquePosition != nil {
				rows.Close()
				return errors.New("tag must match only deleted non-folder Sessions rows")
			}
			var specifics sync_pb.EntitySpecifics
			if err := proto.Unmarshal(row.Specifics, &specifics); err != nil || specifics.GetSession() == nil {
				rows.Close()
				return errors.New("invalid Sessions donor specifics")
			}
			if template == nil {
				template = &row
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil || template == nil {
			return errors.New("no unambiguous tombstone history")
		}
		template.ID = id
		template.Version = int64Ptr(entry.GetVersion())
		template.Ctime = int64Ptr(entry.GetCtime())
		template.Mtime = int64Ptr(math.MinInt64)
		template.DataTypeMtime = stringPtr(fmt.Sprintf("50119#%d", int64(math.MinInt64)))
		template.Deleted = boolPtr(false)
		// Original originator metadata for the missing ID is unknown. Preserve
		// that absence rather than attributing it to a different tombstone ID.
		template.OriginatorCacheGUID, template.OriginatorClientItemID = nil, nil
		template.ExpirationTime = nil
		if err := insertSyncEntity(ctx, tx, template); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE client_item_counts SET item_count = item_count + ? WHERE client_id = ?`, len(entries), chain)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return errors.New("expected exactly one count row")
	}
	return tx.Commit()
}

func reconciliationMetadata(t *testing.T, store *SQLiteStore) []byte {
	t.Helper()
	type reservation struct {
		Chain, ID    string
		Mtime, Ctime sql.NullInt64
	}
	rows, err := store.db.Query(`SELECT client_id, id, mtime, ctime FROM unique_tags ORDER BY client_id, id`)
	if err != nil {
		t.Fatal(err)
	}
	var reservations []reservation
	for rows.Next() {
		var row reservation
		if err := rows.Scan(&row.Chain, &row.ID, &row.Mtime, &row.Ctime); err != nil {
			t.Fatal(err)
		}
		reservations = append(reservations, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	var history [6]int64
	if err := store.db.QueryRow(`SELECT history_item_count_period1, history_item_count_period2,
		history_item_count_period3, history_item_count_period4, last_period_change_time, version
		FROM client_item_counts`).Scan(&history[0], &history[1], &history[2], &history[3], &history[4], &history[5]); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(struct {
		Reservations []reservation
		History      [6]int64
	}{reservations, history})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func reconciliationOriginals(t *testing.T, store *SQLiteStore) []byte {
	t.Helper()
	rows, err := store.db.Query(`SELECT ` + reconciliationColumns + ` FROM sync_entities
		WHERE id NOT IN ('synthetic-missing-0', 'synthetic-missing-1') ORDER BY client_id, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var all []datastore.SyncEntity
	for rows.Next() {
		row, err := scanEntity(rows)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
