//go:build sqlite_cgo && !cgo

package main

// Fail at build time instead of shipping go-sqlite3's nonfunctional no-CGO stub.
var _ = sqlite_cgo_requires_CGO_ENABLED_1_and_a_C_compiler
