package main

import (
	"github.com/brave/go-sync/datastore"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAuditA03StoredIdentity(t *testing.T) {
	for _, change := range []string{"history-bypass", "other-type", "wrong-tag"} {
		t.Run(change, func(t *testing.T) {
			s := sqliteTestOpen(t, filepath.Join(t.TempDir(), "identity.db"))
			e := sqliteTestEntity("synthetic-chain", "victim", 100)
			sqliteTestInsert(t, s, e)
			before := sqliteTestSnapshot(t, s.db)
			update := e
			update.Specifics = []byte("different opaque payload")
			oldVersion := *e.Version
			switch change {
			case "history-bypass":
				update.DataType = sqliteTestPtr(datastore.HistoryTypeID)
				oldVersion++
			case "other-type":
				update.DataType = sqliteTestPtr(47745)
			case "wrong-tag":
				update.ClientDefinedUniqueTag = sqliteTestPtr("not-the-stored-tag")
				update.Deleted = sqliteTestPtr(true)
			}
			conflict, deleted, err := s.UpdateSyncEntity(sqliteTestContext(t), &update, oldVersion)
			if err != nil || !conflict || deleted {
				t.Fatalf("identity mutation accepted: %v %v %v", conflict, deleted, err)
			}
			if !reflect.DeepEqual(before, sqliteTestSnapshot(t, s.db)) {
				t.Fatal("rejected update changed data")
			}
		})
	}
}
