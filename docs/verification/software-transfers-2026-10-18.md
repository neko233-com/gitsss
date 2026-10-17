# Software center and desktop acceptance — 2026-10-18

This report covers the 1.9 implementation in isolated test instances. Screenshots use synthetic Engineering accounts and files. Production deployment receipts remain private because they contain enterprise inventory. No GitHub Actions workflow was created or run.

| Layer | Evidence |
| --- | --- |
| Backend | 363 unit-test packages; Go lint: zero issues |
| Frontend | 65 files / 174 tests; ESLint, Vue/TypeScript and CSS checks |
| SQLite | macOS arm64 CGO + race checks and pure-Go SQLite checks |
| Desktop | Edge, 29 page families × light/dark = 58 successful views; zero page errors, horizontal overflow or automated WCAG A/AA findings |
| Resume | 32 MiB + 137-byte file, pause/reload/reselect; 8 MiB committed prefix; final SHA-256 matches |
| Process restart | Persisted upload offset/hash chain and paused download receipt survive a real isolated service restart; remaining upload and Range download match source bytes |
| Folder drag | Native Edge directory drag: 101 files and an empty directory preserved; only 40 completed history rows remain |
| Directory scale | 600 folders, 500 tree rows maximum, 200 listing rows/page; global search reaches folders outside the visible tree |
| Bulk operations | Move dialog, cycle/conflict rejection, ancestor deduplication, active-upload protection, atomic deletion and confirmation |
| Downloads | Virtual ZIP64 extraction verified with Go, Python and Windows .NET; exact Range bytes; metadata for a 5 GiB file without allocating that file |
| Authorization | Nonmember and anonymous access denied; other users cannot inspect/cancel a session; nonowners cannot change quota; cross-origin mutation rejected |
| Keyboard | Native dialog Escape/focus restoration; Fomantic ArrowDown/Escape with accurate expanded state |
| Privacy | Current public source and publishable branches/tags checked against a private deny-list; local private backups retained |

The page families include the organization directory, overview, links, members, teams, shared Kanban, software center, packages, repository code/issues/pulls/Actions/HTML Docs/contributions/settings/hooks, profile/appearance/account/security/keys/applications, organization settings/team rules/hooks, and administration/users/hooks/configuration. Notification email settings were disabled by instance configuration, so their route was not counted. Automated accessibility checks complement interaction and visual review; they are not a complete manual accessibility certification.

The observed native folder-drag run took about four seconds on the isolated local machine; the 600-folder render/search scenario took about 0.6 seconds. These are local observations, not LAN throughput or a performance guarantee. Transfer memory is bounded by chunks and streaming buffers. ZIP downloads do not create a duplicate archive on disk. Directory import indexes names once per batch, and directory/transfer rendering is bounded.

Downloads report server delivery; the browser download manager controls local disk completion, pause and resume (Ctrl+J). After a page reload, uploads require the original file and verify the complete committed prefix. A service restart may require signing in again according to the configured session backend. See [API, limits and rollback](../features/software-center-transfers.md).
