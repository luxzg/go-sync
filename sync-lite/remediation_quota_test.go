package main

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/datastore"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

// Quota fixtures deliberately decouple counters from row counts. This tests the
// accepted soft-quota contract without generating tens of thousands of records.
func remediationSeedCounts(t *testing.T, s *SQLiteStore, chain string, normal int, history [4]int, anchor int64, version int) {
	t.Helper()
	counts := &datastore.ClientItemCounts{
		ClientID: chain, ID: chain, ItemCount: normal,
		HistoryItemCountPeriod1: history[0], HistoryItemCountPeriod2: history[1],
		HistoryItemCountPeriod3: history[2], HistoryItemCountPeriod4: history[3],
		LastPeriodChangeTime: anchor, Version: version,
	}
	if err := s.UpdateClientItemCount(sqliteTestContext(t), counts, 0, 0); err != nil {
		t.Fatal(err)
	}
}

func remediationHistoryEntity(id string, directive bool) *sync_pb.SyncEntity {
	specifics := &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_History{History: &sync_pb.HistorySpecifics{}}}
	if directive {
		specifics = &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_HistoryDeleteDirective{HistoryDeleteDirective: &sync_pb.HistoryDeleteDirectiveSpecifics{}}}
	}
	e := makeCommitEntity(id, specifics)
	e.ClientTagHash = stringPtr("synthetic-history-" + id)
	return e
}

func remediationRequireCounts(t *testing.T, s *SQLiteStore, chain string, normal, history int) *datastore.ClientItemCounts {
	t.Helper()
	counts, err := s.GetClientItemCount(sqliteTestContext(t), chain)
	if err != nil {
		t.Fatal(err)
	}
	if counts.ItemCount != normal || counts.SumHistoryCounts() != history || counts.Version != sqliteCountVersion {
		t.Fatalf("counts=%+v, want normal=%d history=%d version=%d", counts, normal, history, sqliteCountVersion)
	}
	return counts
}

func TestRemediationQuotaInitializationAndReopen(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	path := filepath.Join(t.TempDir(), "quota.db")
	s := sqliteTestOpen(t, path)
	h := commandHandler(cache.NewCache(NewMemoryRedis()), s)
	token, chain := mustToken(t)
	firstRoots := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_NEW_CLIENT, testNigoriTypeID, testBookmarkTypeID)
	if len(firstRoots.Entries) != 5 {
		t.Fatal("expected five initial server roots")
	}
	remediationRequireCounts(t, s, chain, 0, 0)
	responses := protocolCommit(t, h, token, "device-a", makeCommitEntity("normal", makeBookmarkSpecifics()), remediationHistoryEntity("visit", false))
	for _, r := range responses {
		protocolRequireCommitSuccess(t, r)
	}
	remediationRequireCounts(t, s, chain, 1, 1)
	sqliteTestRowCount(t, s.db, "sync_entities", chain, 7)
	before := sqliteTestChainSnapshot(t, s.db, chain)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = sqliteTestOpen(t, path)
	h = commandHandler(cache.NewCache(NewMemoryRedis()), s)
	again := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_NEW_CLIENT, testNigoriTypeID, testBookmarkTypeID)
	seen := make(map[string]*sync_pb.SyncEntity)
	for _, e := range again.Entries {
		if seen[e.GetIdString()] != nil {
			t.Fatal("duplicate root/item identity after reopen")
		}
		seen[e.GetIdString()] = e
	}
	for _, root := range firstRoots.Entries {
		if !proto.Equal(root, seen[root.GetIdString()]) {
			t.Fatal("repeated initialization changed a root")
		}
	}
	remediationRequireCounts(t, s, chain, 1, 1)
	if !reflect.DeepEqual(before, sqliteTestChainSnapshot(t, s.db, chain)) {
		t.Fatal("reopen/initialization recounted or rewrote existing records")
	}
}

func TestRemediationQuotaGlobalAndLegacyBoost(t *testing.T) {
	for _, tc := range []struct {
		name            string
		normal, history int
	}{
		{"normal-limit", 49999, 0},
		{"legacy-history-boost", 19999, 35000},
		{"legacy-boost-cap", 9999, 60000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			remediationSeedCounts(t, h.db, chain, tc.normal, [4]int{0, 0, 0, tc.history}, time.Now().Unix(), sqliteCountVersion)
			first := makeCommitEntity("last-slot", makeBookmarkSpecifics())
			first.ClientTagHash = stringPtr("synthetic-last-slot")
			second := makeCommitEntity("over-quota", makeBookmarkSpecifics())
			second.ClientTagHash = stringPtr("synthetic-rejected-slot")
			rsp := protocolCommit(t, h, token, "device-a", first, second)
			protocolRequireCommitSuccess(t, rsp[0])
			if rsp[1].GetResponseType() != sync_pb.CommitResponse_OVER_QUOTA {
				t.Fatal("batch did not enforce boundary after accepting the last slot", rsp[1])
			}
			remediationRequireCounts(t, h.db, chain, tc.normal+1, tc.history)
			sqliteTestRowCount(t, h.db.db, "sync_entities", chain, 1)
			sqliteTestRowCount(t, h.db.db, "unique_tags", chain, 1)
			edit := proto.Clone(first).(*sync_pb.SyncEntity)
			edit.IdString, edit.Version = rsp[0].IdString, rsp[0].Version
			edit.Name = stringPtr("updated-at-capacity")
			updated := protocolCommit(t, h, token, "device-b", edit)[0]
			protocolRequireCommitSuccess(t, updated)
			remediationRequireCounts(t, h.db, chain, tc.normal+1, tc.history)
			edit.Version, edit.Deleted = updated.Version, boolPtr(true)
			deleted := protocolCommit(t, h, token, "device-b", edit)[0]
			protocolRequireCommitSuccess(t, deleted)
			remediationRequireCounts(t, h.db, chain, tc.normal, tc.history)
			// Deleting again must not create another free quota slot.
			edit.Version = deleted.Version
			protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device-b", edit)[0])
			remediationRequireCounts(t, h.db, chain, tc.normal, tc.history)
			protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device-a", second)[0])
			remediationRequireCounts(t, h.db, chain, tc.normal+1, tc.history)
			sqliteTestIntegrity(t, h.db.db)
		})
	}
}

func TestRemediationQuotaHistoryBoundary(t *testing.T) {
	for _, directive := range []bool{false, true} {
		t.Run(fmt.Sprintf("directive=%v", directive), func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			remediationSeedCounts(t, h.db, chain, 0, [4]int{0, 0, 0, 29999}, time.Now().Unix(), sqliteCountVersion)
			first, second := remediationHistoryEntity("last-slot", directive), remediationHistoryEntity("overflow", directive)
			rsp := protocolCommit(t, h, token, "device-a", first, second, makeCommitEntity("normal-still-accepted", makeBookmarkSpecifics()))
			for _, r := range rsp {
				protocolRequireCommitSuccess(t, r)
			}
			// Upstream deliberately acknowledges, but does not store, History
			// above its soft cap; do not confuse SUCCESS with persistence here.
			if exists, err := h.db.HasItem(context.Background(), chain, rsp[1].GetIdString()); err != nil || exists {
				t.Fatalf("history overflow exists=%v err=%v, want acknowledged but absent", exists, err)
			}
			sqliteTestRowCount(t, h.db.db, "sync_entities", chain, 2)
			remediationRequireCounts(t, h.db, chain, 1, 30000)
			edit := proto.Clone(first).(*sync_pb.SyncEntity)
			edit.IdString, edit.Version = rsp[0].IdString, rsp[0].Version
			updated := protocolCommit(t, h, token, "device-b", edit)[0]
			protocolRequireCommitSuccess(t, updated)
			remediationRequireCounts(t, h.db, chain, 1, 30000)
			edit.Version, edit.Deleted = updated.Version, boolPtr(true)
			deleted := protocolCommit(t, h, token, "device-b", edit)[0]
			protocolRequireCommitSuccess(t, deleted)
			remediationRequireCounts(t, h.db, chain, 1, 29999)
			accepted := protocolCommit(t, h, token, "device-a", second)[0]
			protocolRequireCommitSuccess(t, accepted)
			if exists, err := h.db.HasItem(context.Background(), chain, accepted.GetIdString()); err != nil || !exists {
				t.Fatal("history slot was not reusable after deletion", exists, err)
			}
			remediationRequireCounts(t, h.db, chain, 1, 30000)
		})
	}
	// The combined cap is evaluated before the special History acknowledgement.
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	remediationSeedCounts(t, h.db, chain, 20000, [4]int{0, 0, 0, 30000}, time.Now().Unix(), sqliteCountVersion)
	r := protocolCommit(t, h, token, "device-a", remediationHistoryEntity("full-total", false))[0]
	if r.GetResponseType() != sync_pb.CommitResponse_OVER_QUOTA {
		t.Fatal("global quota should take precedence over history acknowledgement", r)
	}
	sqliteTestRowCount(t, h.db.db, "sync_entities", chain, 0)
	remediationRequireCounts(t, h.db, chain, 20000, 30000)
}

func TestRemediationQuotaHistoryPeriodRotation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		periods    int64
		beforeEdge bool
		want       [4]int
	}{
		{"before-first-boundary", 1, true, [4]int{1, 2, 3, 5}},
		{"one-period", 1, false, [4]int{2, 3, 4, 1}},
		{"two-periods", 2, false, [4]int{3, 4, 0, 1}},
		{"four-periods", 4, false, [4]int{0, 0, 0, 1}},
		{"long-downtime", 40, false, [4]int{0, 0, 0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			anchor := time.Now().Unix() - tc.periods*periodDurationSecs - 60
			rotations := tc.periods
			if tc.beforeEdge {
				anchor += 120
				rotations--
			}
			remediationSeedCounts(t, h.db, chain, 17, [4]int{1, 2, 3, 4}, anchor, sqliteCountVersion)
			protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device-a", remediationHistoryEntity("new-period", false))[0])
			counts, err := h.db.GetClientItemCount(context.Background(), chain)
			if err != nil {
				t.Fatal(err)
			}
			got := [4]int{counts.HistoryItemCountPeriod1, counts.HistoryItemCountPeriod2, counts.HistoryItemCountPeriod3, counts.HistoryItemCountPeriod4}
			if got != tc.want || counts.ItemCount != 17 || counts.LastPeriodChangeTime != anchor+rotations*periodDurationSecs {
				t.Fatalf("rotated counts=%+v buckets=%v, want %v; normal offset must stay 17", counts, got, tc.want)
			}
			stored := sqliteTestQuery(t, h.db.db, `SELECT history_item_count_period1,history_item_count_period2,history_item_count_period3,history_item_count_period4,last_period_change_time FROM client_item_counts WHERE client_id=?`, chain)
			if !reflect.DeepEqual(stored, [][]any{{int64(got[0]), int64(got[1]), int64(got[2]), int64(got[3]), counts.LastPeriodChangeTime}}) {
				t.Fatal("command did not persist rotated buckets", stored)
			}
		})
	}
}

func TestRemediationQuotaLegacyCountVersions(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		version, normal       int
		wantNormal, wantTotal int
	}{
		{"positive-v1-is-recounted", 1, 42, 2, 2},
		{"zero-v1-keeps-bootstrap-contract", 1, 0, 1, 4},
		{"v2-offset-is-not-recounted", sqliteCountVersion, 42, 43, 4},
		{"future-version-is-not-downgraded", sqliteCountVersion + 1, 42, 43, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			tx, err := h.db.db.BeginTx(sqliteTestContext(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			for i := 0; i < 6; i++ {
				e := sqliteTestEntity(chain, fmt.Sprint(i), int64(100+i))
				if i > 0 {
					e.DataType = sqliteTestPtr(datastore.HistoryTypeID)
				}
				if i == 2 {
					e.DataType = sqliteTestPtr(datastore.HistoryDeleteDirectiveTypeID)
				}
				if i == 3 {
					e.Deleted = boolPtr(true)
				}
				if i == 4 {
					e.ExpirationTime = int64Ptr(1)
				}
				if i == 5 {
					e.DataType = sqliteTestPtr(sqliteTestBookmarkType)
					e.Deleted = boolPtr(true)
				}
				if err := insertSyncEntity(sqliteTestContext(t), tx, &e); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			// A future anchor also characterizes no backwards clock rotation.
			anchor := time.Now().Unix() + periodDurationSecs
			remediationSeedCounts(t, h.db, chain, tc.normal, [4]int{0, 0, 0, 4}, anchor, tc.version)
			before := sqliteTestQuery(t, h.db.db, `SELECT * FROM sync_entities WHERE client_id=? ORDER BY id`, chain)
			earliestMigration := time.Now().Unix()
			protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device-a", makeCommitEntity("after-legacy", makeBookmarkSpecifics()))[0])
			latestMigration := time.Now().Unix()
			// Assert durable values before the reader can normalize a legacy row.
			stored := sqliteTestQuery(t, h.db.db, `SELECT item_count,history_item_count_period1,history_item_count_period2,history_item_count_period3,history_item_count_period4,last_period_change_time,version FROM client_item_counts WHERE client_id=?`, chain)
			if len(stored) != 1 {
				t.Fatal("missing persisted counter row")
			}
			storedAnchor, ok := stored[0][5].(int64)
			if !ok {
				t.Fatal("invalid persisted counter anchor")
			}
			if tc.version < sqliteCountVersion {
				if storedAnchor < earliestMigration || storedAnchor > latestMigration {
					t.Fatal("legacy migration anchor was not saved", storedAnchor)
				}
			} else if storedAnchor != anchor {
				t.Fatal("current/future version anchor was rewritten")
			}
			wantStored := [][]any{{int64(tc.wantNormal), int64(0), int64(0), int64(0), int64(tc.wantTotal), storedAnchor, int64(max(tc.version, sqliteCountVersion))}}
			if !reflect.DeepEqual(stored, wantStored) {
				t.Fatalf("durable counters=%v, want %v", stored, wantStored)
			}
			counts, err := h.db.GetClientItemCount(context.Background(), chain)
			if err != nil {
				t.Fatal(err)
			}
			if counts.ItemCount != tc.wantNormal || counts.SumHistoryCounts() != tc.wantTotal || counts.Version != max(tc.version, sqliteCountVersion) {
				t.Fatalf("legacy counters=%+v, want normal=%d history=%d version=%d", counts, tc.wantNormal, tc.wantTotal, max(tc.version, sqliteCountVersion))
			}
			if tc.version >= sqliteCountVersion && counts.LastPeriodChangeTime != anchor {
				t.Fatal("future anchor was rewritten")
			}
			if !reflect.DeepEqual(before, sqliteTestQuery(t, h.db.db, `SELECT * FROM sync_entities WHERE client_id=? AND id IN ('0','1','2','3','4','5') ORDER BY id`, chain)) {
				t.Fatal("legacy count handling rewrote old entity data")
			}
		})
	}
}
