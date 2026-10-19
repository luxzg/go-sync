//go:build sqlite_cgo && cgo

package main

import _ "github.com/mattn/go-sqlite3"

const (
	sqliteBackend    = "mattn"
	sqliteDriverName = "sqlite3"
)

func sqliteDriverOptions() string {
	return "_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL"
}
