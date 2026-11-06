package main

import (
	"errors"
	"github.com/brave/go-sync/schema/protobuf/sync_pb"
)

// Validate semantics that the reused command code assumes but protobuf makes
// optional. Invalid commit entries still take the upstream per-entry error path.
func prepareCommand(pb *sync_pb.ClientToServerMessage) error {
	switch pb.GetMessageContents() {
	case sync_pb.ClientToServerMessage_GET_UPDATES:
		if pb.GetUpdates == nil {
			return errors.New("missing GetUpdates payload")
		}
		for _, marker := range pb.GetUpdates.FromProgressMarker {
			if marker == nil || marker.DataTypeId == nil {
				return errors.New("missing progress marker data type")
			}
		}
	case sync_pb.ClientToServerMessage_COMMIT:
		if pb.Commit == nil {
			return errors.New("missing Commit payload")
		}
		for _, e := range pb.Commit.Entries {
			if e == nil || e.Specifics == nil {
				continue
			}
			_, history := e.Specifics.SpecificsVariant.(*sync_pb.EntitySpecifics_History)
			if e.Specifics.SpecificsVariant == nil || (history && (e.ClientTagHash == nil || e.GetClientTagHash() == "")) {
				// Unknown-only future specifics cannot be stored safely by this
				// parent revision. Return INVALID_MESSAGE, never silently drop.
				e.Specifics = nil
			}
		}
	}
	return nil
}
