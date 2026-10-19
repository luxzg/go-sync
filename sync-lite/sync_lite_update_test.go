package main

import (
	"context"
	"testing"
	"time"

	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
)

func TestProtocolRemoteBookmarkEdit(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		name := "plain"
		if encrypted {
			name = "opaque-encrypted"
		}
		t.Run(name, func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, clientID := mustToken(t)
			original := makeCommitEntity("desktop-bookmark", makeBookmarkSpecifics())
			original.Specifics.GetBookmark().Url = stringPtr("https://example.invalid/test")
			if encrypted {
				original.Specifics = updateTestEncryptedSpecifics(makeBookmarkSpecifics(), "synthetic-original")
			}
			created := protocolCommit(t, h, token, "desktop", original)[0]
			protocolRequireCommitSuccess(t, created)
			initial := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID)
			if len(initial.Entries) != 1 {
				t.Fatalf("phone received %d bookmarks, want 1", len(initial.Entries))
			}
			received := initial.Entries[0]
			if received.GetIdString() != created.GetIdString() || received.GetVersion() != created.GetVersion() {
				t.Fatal("phone received a different bookmark identity or version")
			}
			rename := proto.Clone(received).(*sync_pb.SyncEntity)
			rename.Name = stringPtr("renamed-on-phone")
			if encrypted {
				rename.Specifics = updateTestEncryptedSpecifics(makeBookmarkSpecifics(), "synthetic-renamed")
			}
			// Ensure the subsequent update lies after the initial progress marker.
			updateTestWaitAfter(t, received.GetMtime())
			updated := protocolCommit(t, h, token, "phone", rename)[0]
			protocolRequireCommitSuccess(t, updated)
			if updated.GetIdString() != created.GetIdString() || updated.GetVersion() <= created.GetVersion() {
				t.Fatal("rename changed identity or did not advance the stored version")
			}
			contents := sync_pb.ClientToServerMessage_GET_UPDATES
			next := &sync_pb.ClientToServerMessage{
				MessageContents: &contents, Share: stringPtr(""),
				GetUpdates: &sync_pb.GetUpdatesMessage{
					FetchFolders: boolPtr(true), GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_PERIODIC),
					FromProgressMarker: initial.NewProgressMarker,
				},
			}
			polled := doAuthedRequest(t, h, token, mustMarshal(t, next), false)
			if polled.GetErrorCode() != sync_pb.SyncEnums_SUCCESS || polled.GetUpdates == nil || len(polled.GetUpdates.Entries) != 1 {
				t.Fatalf("desktop did not receive the rename through incremental polling: %v", polled)
			}
			got := polled.GetUpdates.Entries[0]
			if got.GetIdString() != created.GetIdString() || got.GetName() != "renamed-on-phone" ||
				got.GetVersion() != updated.GetVersion() || !proto.Equal(got.Specifics, rename.Specifics) {
				t.Fatal("desktop received different renamed bookmark contents")
			}
			counts, err := h.db.GetClientItemCount(context.Background(), clientID)
			if err != nil || counts.ItemCount != 1 {
				t.Fatalf("rename changed the number of records: counts=%v err=%v", counts, err)
			}

			stale := proto.Clone(received).(*sync_pb.SyncEntity)
			stale.Name = stringPtr("stale-desktop-edit")
			rejected := protocolCommit(t, h, token, "desktop", stale)[0]
			if rejected.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
				t.Fatalf("stale version response = %v, want CONFLICT", rejected.GetResponseType())
			}
			retry := proto.Clone(got).(*sync_pb.SyncEntity)
			retry.Name = stringPtr("desktop-edit-after-refresh")
			updateTestWaitAfter(t, got.GetMtime())
			protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "desktop", retry)[0])
			final := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID)
			if len(final.Entries) != 1 || final.Entries[0].GetName() != retry.GetName() {
				t.Fatal("fresh-version retry did not recover from the conflict")
			}
		})
	}
}

func TestProtocolFreshSavedGroupsFromBothDevices(t *testing.T) {
	const groupTypeID int32 = 1004874
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, _ := mustToken(t)
	want := make(map[string]*sync_pb.EntitySpecifics)
	for _, device := range []string{"desktop", "phone"} {
		group := &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_SavedTabGroup{
			SavedTabGroup: &sync_pb.SavedTabGroupSpecifics{
				Guid:   stringPtr(device + "-group"),
				Entity: &sync_pb.SavedTabGroupSpecifics_Group{Group: &sync_pb.SavedTabGroup{Title: stringPtr(device + " test group")}},
			},
		}}
		tab := &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_SavedTabGroup{
			SavedTabGroup: &sync_pb.SavedTabGroupSpecifics{
				Guid: stringPtr(device + "-tab"),
				Entity: &sync_pb.SavedTabGroupSpecifics_Tab{Tab: &sync_pb.SavedTabGroupTab{
					GroupGuid: stringPtr(device + "-group"), Url: stringPtr("https://example.invalid/" + device),
				}},
			},
		}}
		for i, specifics := range []*sync_pb.EntitySpecifics{group, tab} {
			id := device + "-group"
			if i == 1 {
				id = device + "-tab"
			}
			entry := makeCommitEntity(id, specifics)
			entry.ClientTagHash = stringPtr("synthetic-tag-" + id)
			response := protocolCommit(t, h, token, device, entry)[0]
			protocolRequireCommitSuccess(t, response)
			want[response.GetIdString()] = specifics
		}
	}
	for _, device := range []string{"desktop", "phone"} {
		got := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, groupTypeID)
		if len(got.Entries) != len(want) {
			t.Fatalf("%s received %d saved group/tab records, want %d", device, len(got.Entries), len(want))
		}
		for _, entry := range got.Entries {
			if !proto.Equal(entry.Specifics, want[entry.GetIdString()]) {
				t.Fatalf("%s received unexpected group/tab contents", device)
			}
		}
	}
}

func updateTestEncryptedSpecifics(specifics *sync_pb.EntitySpecifics, blob string) *sync_pb.EntitySpecifics {
	specifics.Encrypted = &sync_pb.EncryptedData{KeyName: stringPtr("synthetic-key"), Blob: stringPtr(blob)}
	return specifics
}

func updateTestWaitAfter(t *testing.T, mtime int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().UnixMilli() <= mtime {
		if time.Now().After(deadline) {
			t.Fatal("clock did not advance past the stored update time")
		}
		time.Sleep(time.Millisecond)
	}
}
