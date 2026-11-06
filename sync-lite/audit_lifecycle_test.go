package main

import (
	"context"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
	"testing"
)

func auditTaggedEntity(id string) *sync_pb.SyncEntity {
	e := makeCommitEntity(id, makeBookmarkSpecifics())
	e.ClientTagHash = stringPtr("synthetic-shared-tag")
	return e
}

func TestAuditA02ReservationOwnership(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, _ := mustToken(t)
	a := auditTaggedEntity("a")
	ar := protocolCommit(t, h, token, "device", a)[0]
	protocolRequireCommitSuccess(t, ar)
	a.IdString = ar.IdString
	a.Version = ar.Version
	a.Deleted = boolPtr(true)
	deleted := protocolCommit(t, h, token, "device", a)[0]
	protocolRequireCommitSuccess(t, deleted)
	b := auditTaggedEntity("b")
	br := protocolCommit(t, h, token, "device", b)[0]
	protocolRequireCommitSuccess(t, br)
	a.Version = deleted.Version
	again := protocolCommit(t, h, token, "device", a)[0]
	protocolRequireCommitSuccess(t, again)
	if r := protocolCommit(t, h, token, "device", auditTaggedEntity("c"))[0]; r.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
		t.Fatal("old deletion released another live owner's reservation")
	}
	a.Version = again.Version
	a.Deleted = boolPtr(false)
	if r := protocolCommit(t, h, token, "device", a)[0]; r.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
		t.Fatal("resurrection stole another live tag")
	}
	b.IdString = br.IdString
	b.Version = br.Version
	b.Deleted = boolPtr(true)
	b.ClientTagHash = nil
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", b)[0])
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", a)[0])
	if r := protocolCommit(t, h, token, "device", auditTaggedEntity("d"))[0]; r.GetResponseType() != sync_pb.CommitResponse_CONFLICT {
		t.Fatal("resurrection did not reacquire its tag")
	}
}

func TestAuditA02ResurrectionCounts(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	token, chain := mustToken(t)
	e := auditTaggedEntity("a")
	rsp := protocolCommit(t, h, token, "device", e)[0]
	protocolRequireCommitSuccess(t, rsp)
	for _, state := range []bool{true, false, true, false} {
		e.IdString = rsp.IdString
		e.Version = rsp.Version
		e.Deleted = boolPtr(state)
		e.ClientTagHash = nil
		rsp = protocolCommit(t, h, token, "device", e)[0]
		protocolRequireCommitSuccess(t, rsp)
		counts, err := h.db.GetClientItemCount(context.Background(), chain)
		want := 1
		if state {
			want = 0
		}
		if err != nil || counts.ItemCount != want {
			t.Fatalf("deleted=%v counts=%+v err=%v want=%d", state, counts, err, want)
		}
	}
	e.Version = rsp.Version
	e.Deleted = nil
	protocolRequireCommitSuccess(t, protocolCommit(t, h, token, "device", e)[0])
	counts, err := h.db.GetClientItemCount(context.Background(), chain)
	if err != nil || counts.ItemCount != 1 {
		t.Fatal("omission changed count", counts, err)
	}
}
