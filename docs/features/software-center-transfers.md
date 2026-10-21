# Software center transfer compatibility · 1.10

This change is additive inside the layout-reserved software-center extension. Existing nodes, blob digests, Git/LFS/package bytes and paths, API routes and app_state version 1 remain unchanged. New optional metadata records chunk offsets, SHA-256 state, per-chunk verification chains, CRC-32 checksums and bounded download selections.

Uploads use one durable partial file and fixed 8 MiB chunks. Each chunk is verified and fsynced before its committed offset advances. Retries reconcile against the server offset; a crash tail is truncated to that offset before another write. The browser commits each selected File to IndexedDB before starting its network upload. Reloads restore cached sources automatically and verify the entire uploaded prefix before continuing. Explicitly paused tasks stay paused; interrupted active tasks resume automatically. Incomplete sessions reserve quota and expire after seven inactive days. Cancellation releases quota and temporary bytes. Disk admission applies to each chunk.

Directory imports, moves and batch deletion validate names, depth, cycles, conflicts and active uploads. Batch operations are atomic within one metadata write. Identical content retains organization-local deduplication.

Batch downloads use a seekable virtual ZIP64 Store archive over existing blobs, including empty directories, without creating a second archive on disk or buffering file bodies. CRC-32 for legacy blobs is calculated lazily; new uploads calculate it alongside SHA-256. HTTP Range and ETag support page-managed download resumption. Download selections are user-scoped, bounded and expire after 24 hours; unchanged expired selections can be renewed, while changed snapshots are refused. The client receives at most 8 MiB per request, validates status 206, exact Content-Range, size and ETag, then atomically commits chunk and offset to IndexedDB. Failed chunks retry up to four attempts with backoff. Pausing aborts the request and retains committed chunks; cancelling deletes the ticket and local cache without deleting source files. Two uploads and two downloads can run concurrently.

## Browser-local recovery and saving

The transfer panel manages both directions without opening browser download settings. IndexedDB database `gitsss-software-transfers-v1` contains tasks, upload sources, download chunks and a per-user/organization lease. Tasks are partitioned by signed-in user ID and organization route; browser profiles and origins (including different ports) have separate caches. A transactional 15-second lease, renewed every three seconds and released on page exit when possible, prevents duplicate managers in separate tabs. After a crashed tab, takeover waits for lease expiry. Background throttling may relinquish ownership; existing writers check ownership before the next chunk.

Selected File objects are structured-cloned into browser storage, without a JavaScript whole-file ArrayBuffer. Until that transaction completes, leaving the page warns about uncached sources. Older pending sessions have no cached source: choose the original once; it is then cached for later recovery. An unavailable/full local cache lets uploads continue in the current page with a visible warning; recovery then needs the original file. Managed downloads require writable IndexedDB. Quota errors retain previously committed download chunks for retry after freeing space. Private browsing, browser eviction, site-data clearing or source persistence failure can prevent automatic recovery. Browser quota, not the organization's 100 GiB server quota, limits local caching. No browser cache bypass or guaranteed arbitrary-size save is assumed.

Only unfinished tasks appear in the transfer panel, including queued, active, paused, failed and **Ready to save** tasks. Confirmed uploads disappear automatically and release source bytes and task metadata. The panel disappears when nothing remains. Old completed records are cleaned on restore without replaying them. Failed or paused tasks keep their recovery controls.

Completed downloads remain **Ready to save**. **Save to computer** creates a local Blob download; the network transfer is already finished. Browsers do not reliably expose the result of that final ordinary-HTTP save, so **Saved — release cache** is an explicit confirmation, not an inferred disk-write success. Until confirmation, saving can be retried without downloading again. Confirmation automatically removes the row and releases cached bytes and task metadata. Download from the original file row again when needed. Cancellation releases all task bytes. Seven-day inactive cache records are pruned when this software center next opens. Keep normal backups; browser caches are temporary recovery data.

## File list and drag movement

Bundled icons distinguish folders, installers, archives, images, code and other files. Row actions have visible borders, icons and labels; the Move dialog remains available for keyboard and touch use. Drag a row, or a selected group, onto the directory tree, a breadcrumb or a folder row. Eligible targets highlight and the server remains authoritative for membership, cycles, conflicts and active uploads. Self/descendant targets are rejected before submitting; successful moves refresh the existing listing. Browser-native external file drops continue to upload.

Name dialogs reset their unsaved-form baseline when opened and closed. Saving, cancelling or Escape-closing a dialog does not leave a hidden dirty form that incorrectly blocks a later refresh; a failed save retains its input and error.

Modification times use local `yyyy-MM-dd HH:mm:ss` with semantic ISO timestamps. File sizes retain explicit binary units and have five fixed green-to-red bands: under 10 MiB, 10–100 MiB, 100 MiB–1 GiB, 1–10 GiB and at least 10 GiB. The legend and numeric size remain visible so color is supplementary. This is a size scale, not a danger or malware assessment.

Server Git/LFS/package storage and layout are unchanged. Version 1.9 can read all server data after rolling back from 1.10; browser tasks remain inert until a compatible frontend runs again. Finish or cancel page-managed downloads before frontend rollback. No server-side ZIP copy is introduced. Plain intranet HTTP is supported without secure-context File System Access APIs. See [Chrome's File System Access documentation](https://developer.chrome.com/docs/capabilities/web-apis/file-system-access) and [browser storage quotas](https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria).

Backup the existing database and extension together. Rollback to 1.8 reads completed files without moving their bytes. Finish or cancel incomplete resumable uploads before rolling back: the older binary cannot preserve the new session fields when it writes metadata. Recreate download selections after rollback. Single-writer deployment remains required. Validate both CGO and pure-Go SQLite, interrupted/restarted transfers, concurrent reservations, Range ZIP extraction and unchanged Git/SSH/LFS before deployment.

## Shared desktop accessibility

The same Fomantic primitives remain in place. Menu triggers use button semantics, popup menus own menu items, and generated comboboxes reuse their visible labels. Keyboard dropdown handling remains with Fomantic; Arrow/Enter/Escape and focus are included in desktop acceptance. Avatar images and Markdown permalink links have names. Heatmap cells expose image semantics, login follows DOM tab order, and disabled form explanations retain readable contrast. These changes do not alter route, API, permission or theme identifiers.

## Transfer API (authenticated organization members)

All paths below are relative to `/{organization}/-/software`. Existing single-upload and single-download routes remain compatible. Form bodies use URL-encoded repeated `ids` / `paths` fields; raw chunk bodies use `application/octet-stream`. Browser same-origin protections and membership checks apply to every request, including progress polling and Range retries.

| Method and path | Input / behavior |
| --- | --- |
| `POST /batch/directories` | `parent`, repeated relative `paths`; up to 500 paths per atomic batch, returns relative path to stable directory ID |
| `POST /batch/move` | repeated `ids`, destination `parent`; refuses cycles, duplicates, excessive depth and active uploads |
| `POST /batch/delete` | repeated `ids`; ancestor selections include descendants, no partial deletion on validation errors |
| `POST /uploads` | `parent`, `name`, byte `size`, SHA-256 `client_key`; idempotently returns a user-owned candidate |
| `GET /uploads` / `GET /uploads/{id}` | own pending sessions / committed offset and verification chain |
| `PATCH /uploads/{id}?offset=&size=` | fixed 8 MiB chunks (last may be smaller), `X-Chunk-SHA256` header; mismatch returns 400/409 |
| `POST /uploads/{id}/complete` | finalizes a fully received file; repeat calls return the same node |
| `DELETE /uploads/{id}` | cancels an incomplete session and releases quota |
| `POST /downloads` | repeated `ids`; returns a 24-hour user-scoped file or ZIP ticket |
| `GET /downloads` / `GET /downloads/{id}` | own tickets / server-delivery progress and average request speed |
| `GET /downloads/{id}/file` | streaming download, supports Range/If-Range/ETag |
| `DELETE /downloads/{id}` | removes a ticket, never the source file |

The browser fingerprints size, modification time and first/last 64 KiB. Before resume it verifies the entire committed prefix chain; completed candidates require a whole-file SHA-256 match. Hashing runs in dedicated workers without Web Crypto, so plain intranet HTTP works. At most two browser upload workers and eight server chunk streams run concurrently. Each user has at most 32 incomplete sessions and 32 download tickets per organization; organization limits are 128 of each. Import limits: 20,000 entries, 32 levels; a selection has at most 500 roots. ZIPs use Store (no recompression), with at most 4 MiB of path names. Legacy CRC preparation has two concurrent workers. HTTP delivery has eight slots. The transfer panel renders a bounded completed history.

Folder picker APIs do not expose empty directories; drag directory entries preserve them when the browser provides them. Folder imports merge exactly named existing directories and reject conflicting file names. Directory creation is atomic per 500-path batch, while each file is independently finalized: interrupted imports can leave completed directories/files and resumable sessions, never overwrite existing content. Pending uploads block moving/deleting their ancestor until finished or cancelled. Downloads are stable selections: rename/move/delete changes can invalidate a ticket; create a new selection then retry. Tickets do not retain deleted blobs. Progress receipts survive service restarts; the panel resumes from its locally committed offset using Range.

The page sends `X-GitSSS-Transfer: managed`: intermediate ranges update in-memory server progress, and only the final range persists its delivery receipt, avoiding a full organization metadata write per 8 MiB. Ordinary clients retain the existing per-request persisted receipt. The server progress API remains compatible and reports delivery per request; the new panel uses locally committed bytes as its durable progress. Reverse proxies must forward PATCH, Range/If-Range and bodies, and allow at least one 8 MiB chunk; do not cache private software routes.

The directory tree renders at most 500 rows and directs users to global search for additional folders. Each listing page and move search renders at most 200 rows; completed transfers are retired rather than accumulating history rows. The directory batch uses a Unicode case-fold index, preserving existing case-conflict behavior without scanning all existing nodes for every new directory. See [the 1.10 panel acceptance record](../verification/software-transfer-panel-2026-10-18.md) and [the 1.9 transfer baseline](../verification/software-transfers-2026-10-18.md).
