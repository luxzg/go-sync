package main

import (
	"net/url"
	"path/filepath"
)

func sqliteDSN(path string) (string, error) {
	if path == ":memory:" {
		return "file::memory:?" + sqliteDriverOptions(), nil
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "file", Path: absolutePath, RawQuery: sqliteDriverOptions()}
	return u.String(), nil
}
