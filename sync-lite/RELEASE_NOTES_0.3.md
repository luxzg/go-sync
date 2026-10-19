# sync-lite 0.3: Issue Reply Draft

**Follow-up:** the current patch is `0.3.1`, adding optional conflict diagnostics.
An Android commit CONFLICT reported during `0.3` field testing remains under
investigation; automated passes below do not establish a complete browser pass.
See the [manual guide](MANUAL_TESTING.md#diagnosing-persistent-commit-conflicts)
before presenting this draft as a current all-clear.

sync-lite **0.3** now keeps modernc as the preferred default and adds an optional
`mattn/go-sqlite3` build selected with `-tags sqlite_cgo`.

The default Linux x86-64 path is tested and remains the recommendation for
existing users. Both drivers passed the automated tests on Linux x86-64,
including HTTP/HTTPS startup, shutdown/restart, and database compatibility with
`0.2`. The schema and existing server configuration are unchanged; no client
reset is needed. Back up the stopped database before replacing the binary.
The [README upgrade guide](https://github.com/luxzg/go-sync/tree/master/sync-lite#upgrade-from-02-to-03)
has the steps. This update also incorporates reviewed Brave upstream changes and
refreshes the SQLite/protobuf dependencies.

For the preferred build, from `sync-lite/`:

```bash
./scripts/build.sh
```

For the [reported MIPS target](https://github.com/brave/go-sync/issues/393#issuecomment-6260138531),
the optional build can be attempted with:

```bash
CC="zig cc --target=mipsel-linux-musleabi -static" \
GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
./scripts/build.sh mattn
```

That should produce `dist/sync-lite-linux-mipsle-cgo`. Go 1.26+ and a suitable
C cross-compiler are needed. The helper enables CGO and selects `sqlite_cgo`;
the regular build continues using modernc.

**I do not have a MIPS router, so this MIPS build and actual router operation
are still untested.** Could you try it and provide feedback covering:

- Whether compilation, startup, and actual Brave sync succeed, including restart.
- Router manufacturer/model, CPU/architecture, OS/distribution, firmware version.
- Go and Zig/compiler versions, exact build command, and `--version` output.
- Sanitized build/runtime errors if it fails.

Please use a disposable test chain first and keep sync codes, tokens, private
keys, database contents, and private hostnames out of reports. The [build notes](https://github.com/luxzg/go-sync/tree/master/sync-lite#experimental-mips-build)
and [maintenance record](https://github.com/luxzg/go-sync/blob/master/sync-lite/MAINTENANCE.md)
describe the tested scope and pending SQLite engine follow-up. Feedback would
help establish which hardware/firmware combinations actually work.
