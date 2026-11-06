package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/brave/go-sync/cache"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const remediationAIChatTypeID int32 = 10000001

func TestRemediationCommitBatchParentIDMapping(t *testing.T) {
	t.Setenv("BLOCKED_CLIENT_IDS", "")
	h, reopen := remediationProtocolHandler(t)
	token, _ := mustToken(t)
	roots := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_NEW_CLIENT, testNigoriTypeID, testBookmarkTypeID)
	want := make(map[string]*sync_pb.SyncEntity)
	var bookmarkBarID string
	for _, entity := range roots.Entries {
		if entity.GetSpecifics().GetBookmark() != nil {
			want[entity.GetIdString()] = proto.Clone(entity).(*sync_pb.SyncEntity)
		}
		if entity.GetServerDefinedUniqueTag() == "bookmark_bar" {
			bookmarkBarID = entity.GetIdString()
		}
	}
	if bookmarkBarID == "" || len(want) != 4 {
		t.Fatal("initialization did not provide the four bookmark roots")
	}
	entries := []*sync_pb.SyncEntity{
		makeCommitEntity("temporary-parent", makeBookmarkSpecifics()),
		makeCommitEntity("temporary-sibling", makeBookmarkSpecifics()),
		makeCommitEntity("temporary-nested-folder", makeBookmarkSpecifics()),
		makeCommitEntity("temporary-nested-child", makeBookmarkSpecifics()),
		makeCommitEntity("temporary-direct-child", makeBookmarkSpecifics()),
	}
	parents := []string{bookmarkBarID, bookmarkBarID, "temporary-parent", "temporary-nested-folder", "temporary-parent"}
	for i, entity := range entries {
		entity.ParentIdString = stringPtr(parents[i])
		entity.Folder = boolPtr(i == 0 || i == 2)
		entity.Ctime = int64Ptr(1700000000000 + int64(i))
		if !entity.GetFolder() {
			entity.Specifics.GetBookmark().Url = stringPtr("https://example.invalid/" + entity.GetIdString())
		}
	}
	// The upstream contract resolves only earlier entries, not forward references.
	responses := protocolCommit(t, h, token, "synthetic-desktop", entries...)
	ids := make(map[string]string)
	for i, response := range responses {
		protocolRequireCommitSuccess(t, response)
		id := response.GetIdString()
		if id == entries[i].GetIdString() || want[id] != nil {
			t.Fatal("new entry did not receive a distinct server identity")
		}
		expected := remediationProtocolCreated(entries[i], response, "synthetic-desktop")
		if mappedParent, ok := ids[parents[i]]; ok {
			expected.ParentIdString = stringPtr(mappedParent)
		}
		ids[entries[i].GetIdString()] = id
		want[id] = expected
	}
	remediationProtocolRequireEntries(t,
		protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID), want)

	// No parent appears in this later batch: the existing server ID must survive.
	later := makeCommitEntity("temporary-later-child", makeBookmarkSpecifics())
	later.ParentIdString = stringPtr(ids["temporary-nested-folder"])
	later.Ctime = int64Ptr(1700000000010)
	later.Specifics.GetBookmark().Url = stringPtr("https://example.invalid/later")
	laterResponse := protocolCommit(t, h, token, "synthetic-phone", later)[0]
	protocolRequireCommitSuccess(t, laterResponse)
	if laterResponse.GetIdString() == later.GetIdString() || want[laterResponse.GetIdString()] != nil {
		t.Fatal("later child reused a temporary or existing identity")
	}
	want[laterResponse.GetIdString()] = remediationProtocolCreated(later, laterResponse, "synthetic-phone")
	remediationProtocolRequireEntries(t,
		protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID), want)
	reopen()
	remediationProtocolRequireEntries(t,
		protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, testBookmarkTypeID), want)
}

func TestRemediationAIChatRoundTripLifecycle(t *testing.T) {
	field := (&sync_pb.EntitySpecifics{}).ProtoReflect().Descriptor().Fields().ByName("ai_chat_conversation")
	if field == nil || field.Number() != 10000001 {
		t.Fatal("AI Chat specifics no longer use the expected protocol field")
	}
	for _, encrypted := range []bool{false, true} {
		name := "conversation-and-entry"
		if encrypted {
			name = "opaque-encrypted-conversation-and-entry"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("BLOCKED_CLIENT_IDS", "")
			h, reopen := remediationProtocolHandler(t)
			token, _ := mustToken(t)
			original := remediationAIChatSpecifics(t, "original", encrypted)
			entries := make([]*sync_pb.SyncEntity, len(original))
			for i, id := range []string{"c:synthetic-conversation", "e:synthetic-entry"} {
				entries[i] = makeCommitEntity(id, original[i])
				entries[i].ClientTagHash = stringPtr("synthetic-ai-tag-" + id)
				entries[i].Ctime = int64Ptr(1700000000000 + int64(i))
			}
			responses := protocolCommit(t, h, token, "synthetic-desktop", entries...)
			want := make(map[string]*sync_pb.SyncEntity)
			ids := make([]string, len(entries))
			for i, response := range responses {
				protocolRequireCommitSuccess(t, response)
				ids[i] = response.GetIdString()
				if ids[i] == entries[i].GetIdString() || want[ids[i]] != nil {
					t.Fatal("distinct AI Chat records reused a temporary or existing identity")
				}
				want[ids[i]] = remediationProtocolCreated(entries[i], response, "synthetic-desktop")
			}
			initial := protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, remediationAIChatTypeID)
			remediationProtocolRequireEntries(t, initial, want)
			reopen()
			remediationProtocolRequireEntries(t,
				protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, remediationAIChatTypeID), want)

			updatedSpecifics := remediationAIChatSpecifics(t, "remote-update", encrypted)
			staleSpecifics := remediationAIChatSpecifics(t, "rejected-stale-edit", encrypted)
			updates := make([]*sync_pb.SyncEntity, len(entries))
			stale := make([]*sync_pb.SyncEntity, len(entries))
			for i, id := range ids {
				updates[i] = proto.Clone(want[id]).(*sync_pb.SyncEntity)
				updates[i].Specifics = updatedSpecifics[i]
				updates[i].Name = stringPtr("synthetic-remote-edit")
				stale[i] = proto.Clone(want[id]).(*sync_pb.SyncEntity)
				stale[i].Specifics = staleSpecifics[i]
				stale[i].Name = stringPtr("must-not-be-stored")
			}
			responses = protocolCommit(t, h, token, "synthetic-phone", updates...)
			for i, response := range responses {
				remediationProtocolAdvance(t, want[ids[i]], updates[i], response)
			}
			polled := remediationProtocolPoll(t, h, token, initial.NewProgressMarker)
			remediationProtocolRequireEntries(t, polled, want)
			for _, response := range protocolCommit(t, h, token, "synthetic-desktop", stale...) {
				if response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatalf("stale AI Chat update = %v, want CONFLICT", response.GetResponseType())
				}
			}
			remediationProtocolRequireEntries(t,
				protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, remediationAIChatTypeID), want)
			remediationProtocolRequireEntries(t,
				remediationProtocolPoll(t, h, token, polled.NewProgressMarker), nil)

			tombstones := make([]*sync_pb.SyncEntity, len(entries))
			for i, id := range ids {
				tombstones[i] = proto.Clone(want[id]).(*sync_pb.SyncEntity)
				tombstones[i].Deleted = boolPtr(true)
			}
			responses = protocolCommit(t, h, token, "synthetic-phone", tombstones...)
			for i, response := range responses {
				remediationProtocolAdvance(t, want[ids[i]], tombstones[i], response)
			}
			remediationProtocolRequireEntries(t,
				remediationProtocolPoll(t, h, token, polled.NewProgressMarker), want)
			reopen()
			remediationProtocolRequireEntries(t,
				protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, remediationAIChatTypeID), want)
			for _, response := range protocolCommit(t, h, token, "synthetic-desktop", updates...) {
				if response.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
					t.Fatal("stale live edit resurrected a deleted AI Chat record")
				}
			}
			remediationProtocolRequireEntries(t,
				protocolGetUpdates(t, h, token, sync_pb.SyncEnums_GU_TRIGGER, remediationAIChatTypeID), want)
		})
	}
}

func remediationProtocolHandler(t *testing.T) (*testHandler, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "remediation-protocol.db")
	h := &testHandler{}
	open := func() {
		t.Helper()
		store, err := NewSQLiteStore(path)
		if err != nil {
			t.Fatalf("open synthetic protocol store: %v", err)
		}
		h.db = store
		h.handler = commandHandler(cache.NewCache(NewMemoryRedis()), store)
	}
	open()
	t.Cleanup(func() {
		if h.db != nil {
			if err := h.db.Close(); err != nil {
				t.Errorf("close synthetic protocol store: %v", err)
			}
		}
	})
	return h, func() {
		t.Helper()
		if err := h.db.Close(); err != nil {
			t.Fatalf("close before protocol reopen: %v", err)
		}
		h.db = nil
		open()
	}
}

func remediationProtocolCreated(entry *sync_pb.SyncEntity, response *sync_pb.CommitResponse_EntryResponse, guid string) *sync_pb.SyncEntity {
	expected := proto.Clone(entry).(*sync_pb.SyncEntity)
	expected.IdString = stringPtr(response.GetIdString())
	expected.Version = int64Ptr(response.GetVersion())
	expected.Mtime = int64Ptr(response.GetMtime())
	expected.OriginatorCacheGuid = stringPtr(guid)
	expected.OriginatorClientItemId = stringPtr(entry.GetIdString())
	return expected
}

func remediationProtocolAdvance(t *testing.T, expected, update *sync_pb.SyncEntity, response *sync_pb.CommitResponse_EntryResponse) {
	t.Helper()
	protocolRequireCommitSuccess(t, response)
	if response.GetIdString() != expected.GetIdString() || response.GetVersion() <= expected.GetVersion() ||
		response.GetMtime() <= expected.GetMtime() {
		t.Fatal("update changed identity or did not advance version/modification time")
	}
	expected.Specifics = proto.Clone(update.Specifics).(*sync_pb.EntitySpecifics)
	expected.Name = stringPtr(update.GetName())
	expected.Deleted = boolPtr(update.GetDeleted())
	expected.Version = int64Ptr(response.GetVersion())
	expected.Mtime = int64Ptr(response.GetMtime())
}

func remediationProtocolRequireEntries(t *testing.T, response *sync_pb.GetUpdatesResponse, want map[string]*sync_pb.SyncEntity) {
	t.Helper()
	if response.GetChangesRemaining() != 0 || len(response.Entries) != len(want) {
		t.Fatalf("updates contain %d entries, remaining=%d; want exactly %d", len(response.Entries), response.GetChangesRemaining(), len(want))
	}
	seen := make(map[string]bool)
	for _, entity := range response.Entries {
		id := entity.GetIdString()
		expected, ok := want[id]
		if !ok || seen[id] {
			t.Fatalf("updates contain unexpected or duplicate synthetic ID %q", id)
		}
		seen[id] = true
		if !proto.Equal(entity, expected) {
			t.Fatalf("identity, relationships, metadata or specifics changed for synthetic ID %q", id)
		}
	}
}

func remediationProtocolPoll(t *testing.T, h *testHandler, token string, progress []*sync_pb.DataTypeProgressMarker) *sync_pb.GetUpdatesResponse {
	t.Helper()
	if len(progress) != 1 || progress[0].GetDataTypeId() != remediationAIChatTypeID || len(progress[0].GetToken()) == 0 {
		t.Fatal("missing AI Chat progress marker")
	}
	message := &sync_pb.ClientToServerMessage{
		MessageContents: sync_pb.ClientToServerMessage_GET_UPDATES.Enum(), Share: stringPtr(""),
		GetUpdates: &sync_pb.GetUpdatesMessage{
			FetchFolders: boolPtr(true), GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_PERIODIC),
			FromProgressMarker: progress,
		},
	}
	response := doAuthedRequest(t, h, token, mustMarshal(t, message), false)
	if response.GetErrorCode() != sync_pb.SyncEnums_SUCCESS || response.GetUpdates == nil {
		t.Fatal("incremental AI Chat download failed")
	}
	return response.GetUpdates
}

func remediationAIChatSpecifics(t *testing.T, revision string, encrypted bool) []*sync_pb.EntitySpecifics {
	t.Helper()
	conversation := &sync_pb.AIChatConversationSpecifics_Conversation{
		Uuid: stringPtr("synthetic-conversation"), Title: stringPtr("Synthetic " + revision),
		ModelKey: stringPtr("synthetic-model"), TotalTokens: proto.Uint64(1<<54 + 17), TrimmedTokens: proto.Uint64(0),
		CreatedTimeUnixEpochMicros: int64Ptr(1700000000000000), LastModifiedTimeUnixEpochMicros: int64Ptr(1700000000000017),
	}
	compressed := remediationAIChatGzip(t, "Synthetic compressed text: "+revision)
	raw := &sync_pb.AIChatCompressibleString{Value: &sync_pb.AIChatCompressibleString_Raw{Raw: "Synthetic raw " + revision}}
	empty := &sync_pb.AIChatCompressibleString{Value: &sync_pb.AIChatCompressibleString_Raw{Raw: ""}}
	truncated := &sync_pb.AIChatCompressibleString{WasTruncatedForSync: boolPtr(true)}
	source := &sync_pb.AIChatWebSource{
		Title: stringPtr("Synthetic source"), Url: stringPtr("https://example.invalid/source"),
		FaviconUrl: stringPtr("https://example.invalid/favicon"), PageContent: compressed, ExtraSnippets: []string{"first", "second"},
	}
	tool := &sync_pb.AIChatToolUseEvent{
		ToolName: stringPtr("synthetic-tool"), Id: stringPtr("synthetic-call"), ArgumentsJson: compressed,
		IsServerResult: boolPtr(false),
		Output: []*sync_pb.AIChatContentBlock{
			{Content: &sync_pb.AIChatContentBlock_ImageContentBlock{ImageContentBlock: &sync_pb.AIChatImageContentBlock{ImageUrl: stringPtr("https://example.invalid/image")}}},
			{Content: &sync_pb.AIChatContentBlock_TextContentBlock{TextContentBlock: &sync_pb.AIChatTextContentBlock{Text: truncated}}},
			{Content: &sync_pb.AIChatContentBlock_WebSourcesContentBlock{WebSourcesContentBlock: &sync_pb.AIChatWebSourcesContentBlock{
				Sources: []*sync_pb.AIChatWebSource{source}, Queries: []string{"synthetic-query"}, RichResults: []*sync_pb.AIChatCompressibleString{compressed, empty},
			}}},
		},
		Artifacts:           []*sync_pb.AIChatToolArtifact{{Type: stringPtr("synthetic-artifact"), ContentJson: compressed}},
		PermissionChallenge: &sync_pb.AIChatPermissionChallenge{Assessment: stringPtr("synthetic assessment"), Plan: stringPtr("synthetic plan")},
	}
	file := &sync_pb.AIChatUploadedFile{
		Filename: stringPtr("synthetic.bin"), Filesize: proto.Uint32(8), Type: int32Ptr(0),
		Data: []byte{0, 1, 2, 127, 128, 254, 255, 0}, DataTruncatedForSync: boolPtr(false), ExtractedText: compressed,
	}
	entry := &sync_pb.AIChatConversationSpecifics_Entry{
		Uuid: stringPtr("synthetic-entry"), ConversationUuid: stringPtr("synthetic-conversation"), DateUnixEpochMicros: int64Ptr(1700000000000020),
		EntryText: stringPtr("Synthetic entry " + revision), Prompt: stringPtr("synthetic prompt"), CharacterType: int32Ptr(1),
		EditingEntryUuid: stringPtr("synthetic-parent-entry"), ActionType: int32Ptr(0), SelectedText: stringPtr("synthetic selection"), ModelKey: stringPtr("synthetic-model"),
		Events: []*sync_pb.AIChatEntryEventProto{
			{EventOrder: int32Ptr(0), Event: &sync_pb.AIChatEntryEventProto_Completion{Completion: compressed}},
			{EventOrder: int32Ptr(1), Event: &sync_pb.AIChatEntryEventProto_Completion{Completion: empty}},
			{EventOrder: int32Ptr(2), Event: &sync_pb.AIChatEntryEventProto_Completion{Completion: truncated}},
			{EventOrder: int32Ptr(3), Event: &sync_pb.AIChatEntryEventProto_SearchQueries{SearchQueries: &sync_pb.AIChatSearchQueriesEvent{Queries: []string{"synthetic first", "synthetic second"}}}},
			{EventOrder: int32Ptr(4), Event: &sync_pb.AIChatEntryEventProto_WebSources{WebSources: &sync_pb.AIChatWebSourcesEvent{Sources: []*sync_pb.AIChatWebSource{source}, RichResults: []*sync_pb.AIChatCompressibleString{raw}}}},
			{EventOrder: int32Ptr(5), Event: &sync_pb.AIChatEntryEventProto_InlineSearch{InlineSearch: &sync_pb.AIChatInlineSearchEvent{Query: stringPtr("synthetic inline"), ResultsJson: compressed}}},
			{EventOrder: int32Ptr(6), Event: &sync_pb.AIChatEntryEventProto_ToolUse{ToolUse: tool}},
		},
		AssociatedContent: []*sync_pb.AIChatAssociatedContentProto{
			{Uuid: stringPtr("synthetic-page"), Title: stringPtr("Synthetic page"), Url: stringPtr("https://example.invalid/page"), ContentType: int32Ptr(0), ContentUsedPercentage: int32Ptr(37), LastContents: compressed},
			{Uuid: stringPtr("synthetic-truncated-page"), ContentType: int32Ptr(1), LastContents: truncated},
		},
		UploadedFiles: []*sync_pb.AIChatUploadedFile{
			file,
			{Filename: stringPtr("synthetic-truncated.bin"), Filesize: proto.Uint32(42000), DataTruncatedForSync: boolPtr(true), ExtractedText: truncated},
			{Filename: stringPtr("synthetic-empty.bin"), Filesize: proto.Uint32(0), Data: []byte{}, ExtractedText: empty},
		},
	}
	specifics := []*sync_pb.EntitySpecifics{
		{SpecificsVariant: &sync_pb.EntitySpecifics_AiChatConversation{AiChatConversation: &sync_pb.AIChatConversationSpecifics{
			Kind: &sync_pb.AIChatConversationSpecifics_Conversation_{Conversation: conversation},
		}}},
		{SpecificsVariant: &sync_pb.EntitySpecifics_AiChatConversation{AiChatConversation: &sync_pb.AIChatConversationSpecifics{
			Kind: &sync_pb.AIChatConversationSpecifics_Entry_{Entry: entry},
		}}},
	}
	for _, message := range []proto.Message{conversation, entry, compressed, truncated, file, source, tool, entry.Events[0]} {
		remediationProtocolUnknownFields(message)
	}
	for i, specific := range specifics {
		remediationProtocolUnknownFields(specific)
		remediationProtocolUnknownFields(specific.GetAiChatConversation())
		if encrypted {
			// Synthetic opaque bytes exercise storage, not client cryptography.
			specifics[i] = &sync_pb.EntitySpecifics{
				SpecificsVariant: &sync_pb.EntitySpecifics_AiChatConversation{AiChatConversation: &sync_pb.AIChatConversationSpecifics{}},
				Encrypted:        &sync_pb.EncryptedData{KeyName: stringPtr("synthetic-key"), Blob: stringPtr(base64.StdEncoding.EncodeToString(mustMarshal(t, specific)))},
			}
			remediationProtocolUnknownFields(specifics[i])
			remediationProtocolUnknownFields(specifics[i].GetAiChatConversation())
			remediationProtocolUnknownFields(specifics[i].Encrypted)
		}
	}
	return specifics
}

func remediationAIChatGzip(t *testing.T, value string) *sync_pb.AIChatCompressibleString {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatalf("compress synthetic AI Chat value: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("finish synthetic AI Chat value: %v", err)
	}
	return &sync_pb.AIChatCompressibleString{Value: &sync_pb.AIChatCompressibleString_Gzipped{Gzipped: buffer.Bytes()}}
}

func remediationProtocolUnknownFields(message proto.Message) {
	unknown := protowire.AppendTag(nil, 19001, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 17)
	unknown = protowire.AppendTag(unknown, 19002, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, []byte{0, 128, 255, 0})
	message.ProtoReflect().SetUnknown(unknown)
}
