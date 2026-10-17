# Workspace centers verification · 2026-10-17

## Delivered surface

- One desktop workspace navigation, searchable repository and organization entry points, direct organization landing, readable light/dark colors, and unchanged readable administrator typography.
- Organization software center: default 100 GiB quota, owner quota control, directory tree, folder/file rename and recursive deletion, global name search, streamed uploads, ranged downloads and organization-local content deduplication.
- The existing Docker/OCI registry with organization package navigation and explicit client connection instructions.
- Repository daily/weekly/monthly contribution reports, contributor filters, filtered totals, CSV and explicit counting limits.
- First-push notification links use a concrete commit, while normal updates use comparisons.
- HTTP Git advertisements stream with bounded protocol detection while preserving v0/v1/v2 framing.

## Local gates and real clients

- Backend Make unit package selection: 363 packages passed. Frontend: 64 files / 170 tests passed. Go lint, Make ESLint, Vue/TypeScript and CSS lint passed.
- macOS CGO race checks and pure-Go SQLite checks passed for the affected services, routers, authorization, settings and database packages. Test HOME and SSH roots were isolated.
- Edge inspected nine main page types. Directory create/rename/search, delete confirmation cancellation, weekly/monthly grouping, filtered totals and CSV download passed. The tested 1366 and 1600 pixel desktop views had no horizontal overflow or page exceptions. Light and dark center views were checked.
- Windows Git clients passed HTTP clone/fetch/pull/push, LFS upload/download, pull-request merging and permission checks against the isolated instance. Explicit v0/v1/v2, shallow/deepen/unshallow, partial-clone lazy object retrieval and three concurrent no-op fetches passed.
- A 64 MiB file matched SHA-256 after streamed upload and full download; Range returned the exact requested bytes. Quota reservations, cancellation/truncation, duplicate bytes, directory depth, recursive deletion and snapshot invalidation have targeted tests. HTTP checks rejected anonymous/nonmember access and cross-site writes; maximum page input was bounded.
- Docker 29 built, pushed, removed local image/cache, pulled and extracted a real image. Manifest and content digests matched. The temporary nested daemon exposed no host port and was removed; other running containers and Docker client configuration were unchanged.
- Loopback transfer timings are private diagnostic observations, not LAN throughput claims.

Screenshots use synthetic organizations and repositories. Private addresses, credentials, hook endpoints, production repository names and operational receipts remain outside Git.

## Compatibility and deployment boundary

No schema migration, Git layout/object/ref rewrite, LFS relocation, SSH key/account/configuration change, or package/Actions storage change is introduced. Software center metadata uses existing app_state records and bytes use the layout-reserved software-center extension. One writer process per data directory is required. Preserve the database plus extension for backup; rollback is the previous binary, with the extension retained.

Each deployment still requires original Git/SSH/LFS checks, existing service health, and rollback retention. Local acceptance does not substitute for production-client evidence or a production build/restart exercise.
