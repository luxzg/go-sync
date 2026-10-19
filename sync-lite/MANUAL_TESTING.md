# Verify Sync Between Two Brave Devices

Use this checklist after installation or a server update. It checks actual
two-way synchronization, not just whether the server responds. Call your desktop
**A** and your phone **B**, or use two other devices you control.

Use harmless, uniquely named test items. **Do not leave your existing chain,
reset Sync, delete your Sync account, or use clear-data controls.** Changes and
deletions can propagate to other devices; touch only the test items you create.
This guide is an operator procedure, not a claim that these checks have run.

## 1. Prepare Both Clients

1. After an upgrade, confirm the service startup log reports the intended version
   (`0.3.1` for this release). The installed binary's `--version` output alone does
   not prove the running service has restarted with it.
2. In each browser's Sync settings, confirm both devices belong to the existing
   chain and enable the categories you intend to test on **both** devices.
   Confirm the same custom server base URL, including its port. Do not append
   `/command/`. Do not create a replacement chain for an existing installation.
3. Keep both browsers open and the phone awake, with access to the server's
   network. Use normal windows, not private windows. Allow initial sync to settle.
4. Record browser versions/channels (About Brave), server version/backend, and
   the test date. No sync code, keys, or database contents are needed.

## 2. Open The Diagnostic Page

Enter `brave://sync-internals/` in both browsers. Labels vary by Brave version;
the following refers to Chromium's diagnostic layout, which Brave builds upon.

- **About:** check the reported server URL, recent sync activity, errors, and
  **Type Info** for the enabled categories. Note their counters before testing.
- **Sync Node Browser:** select a category and inspect its records. Click
  **Refresh** after changes; the tree is not necessarily live.
- **Search**, if present: search for your unique test marker across local sync
  records. Names/URLs may not be exposed for every type or encryption state.
- **Traffic Log** or the protocol log: look for successful **Commit** activity
  on the sender and **GetUpdates** activity on the receiver after your change.

These are each client's local view, not a direct server database viewer. A
successful request, matching counts, or Nigori keys alone does not prove a new
item arrived. Verify the uniquely marked item in the **other browser's normal
UI** as well. Total counts need not match between different platforms.

Diagnostic reference: [Chromium Sync diagnostics](https://www.chromium.org/developers/design-documents/sync/diagnostics/).
For category behavior, see [Understanding Brave Sync](https://support.brave.com/hc/en-us/articles/360059793111-Understanding-Brave-Sync).

## 3. Prove Bookmarks Work In Both Directions

Pick a fresh marker for this run, for example `SL03-check-01`. Do not manually
create the same marker on the receiving device: that would hide a sync failure.

1. On **A only**, create a bookmark folder named `SL03-check-01-from-A`.
   Add a bookmark named `SL03-check-01-bookmark-A` pointing to
   `https://example.com/?sync-lite-test=SL03-check-01-A`.
2. On **B**, wait for the folder and bookmark in the normal bookmark manager.
   Check both title and URL. Search all bookmark locations, including desktop
   or mobile folders, rather than only the currently displayed folder.
3. On **B**, refresh Sync Node Browser and inspect **Bookmarks**, or use Search
   for `SL03-check-01`. If readable, the record should agree with the UI result.
4. On **B**, rename that received bookmark to `SL03-check-01-renamed-on-B`.
   Verify the changed name reaches **A** without editing it there.
5. On **B only**, create a different folder/bookmark using the suffix `from-B`
   and URL marker `SL03-check-01-B`. Verify it reaches **A**.

This establishes fresh A-to-B creation, B-to-A creation, and a remote edit.
Start with this small test before trying larger sets or other categories.

## 4. Check Tabs, Groups, And Other Items

Use distinct markers for each direction. Check each category independently;
passing Bookmarks does not establish that every data type works.

| Item | Action on A | What to check on B |
| --- | --- | --- |
| Open tabs | Open `https://example.com/?sync-lite-test=SL03-check-01-tab-A` and leave it open | Look in the browser's other-device tabs view, commonly under History/Recent tabs; inspect Sessions in diagnostics if available |
| Tab groups | Create a group named `SL03-check-01-group-A` with two harmless tabs; enable any save/sync option the client exposes | Look in the receiving client's synced/saved groups UI, where supported; check group name and member URLs |
| Passwords (optional) | Manually add a dummy saved login for `https://sync-lite-test.invalid`, username `SL03-check-01-A`, and a non-secret throwaway password | Look for that dummy entry in the password manager; never use a real password as a test marker |
| History (optional) | Visit a harmless page with a fresh URL marker | Check the receiving client's synced history view, allowing for client filtering and platform differences |

Repeat from **B to A** with `-B` markers. Record unsupported or untested
categories separately instead of calling them passes. Preferences/autofill and
other supported categories can be checked similarly with disposable values;
desktop/mobile feature differences can limit what is visible.

Do not expect remote tabs to automatically open as local tabs. Group support
and presentation also vary by client/platform; an unsaved local group is not
necessarily a synced group. In diagnostics, tabs may be represented under
**Sessions** and groups under a name such as **Saved Tab Groups**; labels and
availability depend on the browser version. The visible remote item is the
main success criterion, not an assumed diagnostic type name.

## 5. If An Item Has Not Arrived

1. Give both clients a few minutes with the phone awake. This is a practical
   observation window, not a guaranteed sync deadline. Note the elapsed time.
2. Recheck the same chain, custom endpoint, network access, and category toggles.
   Inspect errors and activity in About on **both** clients.
3. Refresh Sync Node Browser. If available, **Trigger GetUpdates** on the
   receiver can request a download; note if this manual step was needed. It does
   not prove the sender uploaded the item or normal automatic sync worked.
4. If the sender shows successful commits but the receiver has no marked item,
   note the receiver's download/error state. If diagnostics show it but the UI
   does not, record that distinction instead of declaring full success.
5. Check sanitized service error messages if needed. Do not try clear-data,
   account deletion, profile removal, or a new chain as troubleshooting steps.

### Diagnosing Persistent Commit Conflicts

`Last synced: just now` and a successful Download step do not establish that
uploads succeeded. Current-cycle counters and last-completed-session results
are separate; zero current conflicts does not erase a prior failed commit.
Backoff can postpone the next attempt. Record the displayed retry time and
timezone, let it pass with the phone awake, and check for a fresh completed
session. Do not repeatedly force retries or reset Sync.

**Do not toggle a combined History and tabs setting to reset Sessions.**
Chromium's [History-disable path](https://raw.githubusercontent.com/chromium/chromium/main/components/history/core/browser/sync/history_sync_bridge.cc)
removes visits imported from other devices. Sync-lite's server-side History
expiration is 14 days, so older imported visits may not download again. Exact
behavior depends on the client version; a server backup does not protect
phone-only history. Keep the combined setting enabled while diagnosing sync.

A Commit response can report overall success while individual `entryresponse`
items say `CONFLICT`. Match each response to the same position in that request's
entry list; do not assume the conflict refers to the last bookmark you changed.
`changes_remaining` indicates more download pages, not a count of failed commits.

If ordinary logs show nothing, `0.3.1` can classify otherwise silent update
conflicts. This is diagnostic support, **not a repair**:

1. Use the [backup-first binary upgrade](README.md#upgrade-from-02-or-03-to-031).
   Preserve the database and the existing chain. No schema migration is involved.
2. After backing up the config, edit `/etc/sync-lite/sync-lite.env` and add one
   line: `SYNC_LOG_CONFLICTS=1`. Preserve every other setting.
3. Restart with `sudo systemctl restart sync-lite`. Check that startup reports
   `0.3.1` and says conflict diagnostics are enabled.
4. Keep the phone open past its retry time. Check the journal without dumping
   any browser data:

```bash
sudo journalctl -u sync-lite --since "10 minutes ago" --no-pager |
  grep -F 'sync conflict operation='
```

Example, using public type metadata only:

```text
sync conflict operation=update reason=version_mismatch data_type=32904
```

| Reason | Meaning, not an automatic repair instruction |
| --- | --- |
| `missing_entity` | The requested update's server ID was not found in this chain |
| `version_mismatch` | The stored version is missing or differs from the submitted base version |
| `duplicate_client_tag` | A new insert collided with an existing client-tag reservation |
| `duplicate_entity_id` | A new insert collided with an existing entity ID |

`32904` is Bookmarks; `1004874` is Saved Tab Groups; `50119` is Sessions.
Other numbers identify other protocol types. A session-level conflict does not identify which type
failed; do not assume it was the bookmark being tested.

Logs are limited to one line per operation/reason/type every 30 seconds, with
bounded tracking memory. They omit chain/item/device IDs, versions, titles,
URLs, passwords, keys, and payloads. No output is inconclusive: the client may
still be in backoff, may not have retried, or may not have reached this process.
Share only the matching reason/type lines and the client/version/timing summary.
Remove the setting (or leave it empty) and restart after diagnosis.

Diagnostics do not authorize changing database records or weakening version
checks. Any production data change needs separately reviewed preconditions, a
fresh consistent backup, explicit scope, and subsequent browser validation.

The counter distinction is visible in [Chromium's diagnostic implementation](https://chromium.googlesource.com/chromium/src/+/82baf7795e06d210f92656c31919a5711bdec58d/components/sync/service/sync_internals_util.cc).

**Privacy:** diagnostics can reveal URLs, device identifiers, payloads, and key
material. Do not enable detailed payload capture or export/share full node
dumps for this test. Keep sync codes, Nigori keys, tokens, passwords, private
hostnames, and unrelated browsing data out of screenshots and reports.

## 6. Record The Result And Clean Up

For each category, record A-to-B and B-to-A results, approximate delay, and any
manual refresh/download required. A short report is enough:

```text
Date/time and timezone:
Server version/backend:
A: OS, Brave version/channel/package:
B: OS, Brave version/channel/package:
Bookmarks: A->B / B->A / remote rename:
Tabs: A->B / B->A:
Groups: A->B / B->A:
Other categories tested:
Automatic or Trigger GetUpdates required:
Errors / not tested / unsupported:
```

Once both directions are confirmed, remove **only** the disposable bookmarks,
dummy login, and test groups you created. Bookmark deletion reaching the other
device is an additional useful check. Close test tabs on their originating
device; do not assume closing a remote-tab listing closes its source tab.
Keep the pre-upgrade server backup. Report the tested scope so the project's
validation record can be updated without implying untested categories passed.
