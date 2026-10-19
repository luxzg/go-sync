package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

const protocolDeviceInfoTypeID int32 = 154522

func TestProtocolDuplicateClientTagConflict(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, clientID := mustToken(t)

	first := makeCommitEntity("original", makeBookmarkSpecifics())
	first.ClientTagHash = stringPtr("synthetic-shared-tag")
	accepted := protocolCommit(t, h, token, "device-a", first)[0]
	protocolRequireCommitSuccess(t, accepted)
	before, err := h.db.GetClientItemCount(context.Background(), clientID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ItemCount != 1 {
		t.Fatalf("initial item count = %d, want 1", before.ItemCount)
	}

	duplicate := makeCommitEntity("duplicate", makeBookmarkSpecifics())
	duplicate.ClientTagHash = stringPtr(*first.ClientTagHash)
	rejected := protocolCommit(t, h, token, "device-b", duplicate)[0]
	if rejected.ResponseType == nil || *rejected.ResponseType != sync_pb.CommitResponse_CONFLICT {
		t.Fatalf("duplicate tag response = %v, want CONFLICT", rejected.ResponseType)
	}
	if id := rejected.GetIdString(); id != "" {
		exists, err := h.db.HasItem(context.Background(), clientID, id)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("conflict response advertised nonexistent server ID %q", id)
		}
	}
	after, err := h.db.GetClientItemCount(context.Background(), clientID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ItemCount != before.ItemCount || after.SumHistoryCounts() != before.SumHistoryCounts() {
		t.Fatalf("conflict changed counts: before=%+v after=%+v", before, after)
	}
	updates := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID)
	if len(updates.Entries) != 1 || updates.Entries[0].GetIdString() != accepted.GetIdString() ||
		updates.Entries[0].GetName() != "original" {
		t.Fatalf("duplicate changed stored entities: %+v", updates.Entries)
	}
}

func TestProtocolSameChainDevicesSurviveReopen(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	path := filepath.Join(t.TempDir(), "two-devices.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			if err := store.Close(); err != nil {
				t.Errorf("close store: %v", err)
			}
		}
	})
	h := commandHandler(cache.NewCache(NewMemoryRedis()), store)
	token, clientID := mustToken(t)
	type expectedEntity struct {
		id        string
		guid      string
		clientID  string
		version   int64
		specifics *sync_pb.EntitySpecifics
	}
	want := make(map[string]expectedEntity)
	commitDevice := func(guid string) {
		t.Helper()
		device := makeCommitEntity("info-"+guid, &sync_pb.EntitySpecifics{
			SpecificsVariant: &sync_pb.EntitySpecifics_DeviceInfo{DeviceInfo: &sync_pb.DeviceInfoSpecifics{
				CacheGuid: stringPtr(guid), ClientName: stringPtr("Synthetic " + guid),
			}},
		})
		device.ClientTagHash = stringPtr("device-tag-" + guid)
		bookmark := makeCommitEntity("bookmark-"+guid, makeBookmarkSpecifics())
		bookmark.Specifics.GetBookmark().Url = stringPtr("https://example.invalid/" + guid)
		entries := []*sync_pb.SyncEntity{device, bookmark}
		responses := protocolCommit(t, h, token, guid, entries...)
		for i, response := range responses {
			protocolRequireCommitSuccess(t, response)
			if _, exists := want[response.GetIdString()]; exists {
				t.Fatalf("distinct device entries reused server ID %q", response.GetIdString())
			}
			want[response.GetIdString()] = expectedEntity{
				id: response.GetIdString(), guid: guid,
				clientID: entries[i].GetIdString(), version: response.GetVersion(),
				specifics: proto.Clone(entries[i].Specifics).(*sync_pb.EntitySpecifics),
			}
		}
	}
	readDevice := func(guid string) {
		t.Helper()
		updates := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER,
			protocolDeviceInfoTypeID, testBookmarkTypeID)
		if len(updates.Entries) != len(want) {
			t.Fatalf("%s read %d entries, want %d", guid, len(updates.Entries), len(want))
		}
		seen := make(map[string]bool)
		for _, entity := range updates.Entries {
			expected, ok := want[entity.GetIdString()]
			if !ok || seen[entity.GetIdString()] {
				t.Fatalf("%s read unexpected/duplicate ID %q", guid, entity.GetIdString())
			}
			seen[entity.GetIdString()] = true
			if entity.GetOriginatorCacheGuid() != expected.guid ||
				entity.GetOriginatorClientItemId() != expected.clientID ||
				entity.GetName() != expected.clientID || entity.GetVersion() != expected.version ||
				entity.GetDeleted() || entity.GetFolder() ||
				!proto.Equal(entity.Specifics, expected.specifics) {
				t.Fatalf("%s read changed identity/payload for %q", guid, expected.id)
			}
		}
	}

	commitDevice("device-a")
	readDevice("device-b")
	commitDevice("device-b")
	readDevice("device-a")
	readDevice("device-b")
	if err := store.Close(); err != nil {
		t.Fatalf("close before reopen: %v", err)
	}
	store = nil
	store, err = NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	h = commandHandler(cache.NewCache(NewMemoryRedis()), store)
	readDevice("device-a")
	readDevice("device-b")
	counts, err := store.GetClientItemCount(context.Background(), clientID)
	if err != nil {
		t.Fatal(err)
	}
	if counts.ItemCount != len(want) || counts.SumHistoryCounts() != 0 {
		t.Fatalf("reopened counts = %+v, want %d normal, 0 history", counts, len(want))
	}
}

func TestProtocolNewClientInitializesRoots(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, _ := mustToken(t)
	wantParents := map[string]string{
		"google_chrome_nigori":    "",
		"google_chrome_bookmarks": "",
		"other_bookmarks":         "google_chrome_bookmarks",
		"synced_bookmarks":        "google_chrome_bookmarks",
		"bookmark_bar":            "google_chrome_bookmarks",
	}
	var initial map[string]*sync_pb.SyncEntity
	for attempt := 0; attempt < 2; attempt++ {
		updates := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_NEW_CLIENT,
			testNigoriTypeID, testBookmarkTypeID)
		if len(updates.EncryptionKeys) != 1 || len(updates.EncryptionKeys[0]) == 0 {
			t.Fatal("new client missing Nigori initialization encryption-key placeholder")
		}
		if len(updates.Entries) != len(wantParents) || updates.GetChangesRemaining() != 0 {
			t.Fatalf("initialization returned %d entries, remaining=%d", len(updates.Entries), updates.GetChangesRemaining())
		}
		byTag := make(map[string]*sync_pb.SyncEntity)
		ids := make(map[string]bool)
		for _, entity := range updates.Entries {
			tag := entity.GetServerDefinedUniqueTag()
			if _, ok := wantParents[tag]; !ok || byTag[tag] != nil {
				t.Fatalf("unexpected/duplicate root tag %q", tag)
			}
			if entity.GetIdString() == "" || ids[entity.GetIdString()] ||
				!entity.GetFolder() || entity.GetDeleted() || entity.GetVersion() <= 0 {
				t.Fatalf("invalid root %q: %+v", tag, entity)
			}
			ids[entity.GetIdString()] = true
			byTag[tag] = entity
		}
		for tag, parentTag := range wantParents {
			parentID := "0"
			if parentTag != "" {
				parentID = byTag[parentTag].GetIdString()
			}
			entity := byTag[tag]
			if entity.GetParentIdString() != parentID {
				t.Fatalf("root %q parent = %q, want %q", tag, entity.GetParentIdString(), parentID)
			}
			if tag == "google_chrome_nigori" {
				if entity.GetSpecifics().GetNigori() == nil {
					t.Fatal("Nigori root missing Nigori specifics")
				}
			} else if entity.GetSpecifics().GetBookmark() == nil {
				t.Fatalf("bookmark root %q missing bookmark specifics", tag)
			}
			if initial != nil && !proto.Equal(initial[tag], entity) {
				t.Fatalf("repeated NEW_CLIENT changed root %q", tag)
			}
		}
		initial = byTag
	}
}

func TestProtocolBlockedClientIDCaseInsensitive(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, clientID := mustToken(t)
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		t.Fatal("unexpected synthetic token format")
	}
	parts[2] = strings.ToUpper(parts[2])
	upperToken := base64.URLEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
	// Changing only public-key hex casing must not invalidate the signature.
	protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID)
	protocolGetUpdates(t, h, upperToken, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID)

	for _, tc := range []struct {
		name    string
		blocked string
		token   string
	}{
		{name: "upper-list-lower-token", blocked: strings.ToUpper(clientID), token: token},
		{name: "lower-list-upper-token", blocked: clientID, token: upperToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BLOCKED_CLIENT_IDS", tc.blocked)
			req := httptest.NewRequest(http.MethodPost, "/command/",
				bytes.NewReader(mustMarshal(t, makeCommitMessage(nil))))
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("blocked request status = %d, want 401", rr.Code)
			}
		})
	}
}

func protocolCommit(t *testing.T, h http.Handler, token, guid string, entries ...*sync_pb.SyncEntity) []*sync_pb.CommitResponse_EntryResponse {
	t.Helper()
	msg := makeCommitMessage(entries)
	msg.Commit.CacheGuid = stringPtr(guid)
	rsp := doAuthedRequest(t, h, token, mustMarshal(t, msg), false)
	if rsp.ErrorCode == nil || *rsp.ErrorCode != sync_pb.SyncEnums_SUCCESS ||
		rsp.Commit == nil || len(rsp.Commit.Entryresponse) != len(entries) {
		t.Fatalf("unexpected commit response: %v", rsp)
	}
	return rsp.Commit.Entryresponse
}

func protocolRequireCommitSuccess(t *testing.T, entry *sync_pb.CommitResponse_EntryResponse) {
	t.Helper()
	if entry == nil || entry.ResponseType == nil || *entry.ResponseType != sync_pb.CommitResponse_SUCCESS ||
		entry.GetIdString() == "" || entry.GetVersion() <= 0 {
		t.Fatalf("expected persisted successful commit: %v", entry)
	}
}

func protocolGetUpdates(t *testing.T, h http.Handler, token string, origin sync_pb.SyncEnums_GetUpdatesOrigin, typeIDs ...int32) *sync_pb.GetUpdatesResponse {
	t.Helper()
	contents := sync_pb.ClientToServerMessage_GET_UPDATES
	msg := &sync_pb.ClientToServerMessage{
		MessageContents: &contents,
		Share:           stringPtr(""),
		GetUpdates: &sync_pb.GetUpdatesMessage{
			FetchFolders:       boolPtr(true),
			GetUpdatesOrigin:   enumGUPtr(origin),
			FromProgressMarker: markers(typeIDs, make([]int64, len(typeIDs))),
		},
	}
	rsp := doAuthedRequest(t, h, token, mustMarshal(t, msg), false)
	if rsp.ErrorCode == nil || *rsp.ErrorCode != sync_pb.SyncEnums_SUCCESS || rsp.GetUpdates == nil {
		t.Fatalf("unexpected GetUpdates response: %v", rsp)
	}
	return rsp.GetUpdates
}
