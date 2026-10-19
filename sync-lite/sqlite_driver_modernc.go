//go:build !sqlite_cgo

package main

import _ "modernc.org/sqlite"

const (
	sqliteBackend    = "modernc"
	sqliteDriverName = "sqlite"
)

func sqliteDriverOptions() string {
	return "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"
}
