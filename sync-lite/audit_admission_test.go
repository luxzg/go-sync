package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/brave/go-sync/command"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuditA13AdmissionBeyondTombstonePage(t *testing.T) {
	for _, tc := range []struct {
		live    int
		high    bool
		allowed bool
	}{{0, false, true}, {49, false, true}, {50, false, false}, {99, true, true}, {100, true, false}} {
		t.Run(fmt.Sprintf("live-%d-high-%v", tc.live, tc.high), func(t *testing.T) {
			h, cleanup := newTestHandler(t)
			defer cleanup()
			token, chain := mustToken(t)
			command.LoadHighDeviceLimitClientIDs("")
			if tc.high {
				command.LoadHighDeviceLimitClientIDs(chain)
			}
			defer command.LoadHighDeviceLimitClientIDs("")
			payload := mustMarshal(t, &sync_pb.EntitySpecifics{SpecificsVariant: &sync_pb.EntitySpecifics_DeviceInfo{DeviceInfo: &sync_pb.DeviceInfoSpecifics{}}})
			tx, err := h.db.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			for i := 0; i < 501+tc.live; i++ {
				deleted := i < 501
				if _, err := tx.Exec(`INSERT INTO sync_entities (client_id,id,version,mtime,deleted,data_type,folder,specifics) VALUES (?,?,1,?,?,154522,0,?)`, chain, fmt.Sprint(i), i+1, deleted, payload); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			msg := &sync_pb.ClientToServerMessage{Share: stringPtr(""), MessageContents: sync_pb.ClientToServerMessage_GET_UPDATES.Enum(), GetUpdates: &sync_pb.GetUpdatesMessage{GetUpdatesOrigin: enumGUPtr(sync_pb.SyncEnums_NEW_CLIENT), FromProgressMarker: markers([]int32{154522}, []int64{0}), FetchFolders: boolPtr(false)}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, "/command/", bytes.NewReader(mustMarshal(t, msg))).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			rsp := &sync_pb.ClientToServerResponse{}
			if err := proto.Unmarshal(rr.Body.Bytes(), rsp); err != nil {
				t.Fatal("not a controlled protocol response", rr.Code, err)
			}
			want := sync_pb.SyncEnums_THROTTLED
			if tc.allowed {
				want = sync_pb.SyncEnums_SUCCESS
			}
			if rsp.GetErrorCode() != want {
				t.Fatalf("admission returned %v, want %v", rsp.GetErrorCode(), want)
			}
			if tc.allowed && (len(rsp.GetGetUpdates().GetEntries()) != 500 || rsp.GetGetUpdates().GetChangesRemaining() == 0) {
				t.Fatal("admission shortcut leaked into ordinary paginated download")
			}
		})
	}
}
