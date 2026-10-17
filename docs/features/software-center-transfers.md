# Software center transfer compatibility · 1.9

This change is additive inside the layout-reserved software-center extension. Existing nodes, blob digests, Git/LFS/package bytes and paths, API routes and app_state version 1 remain unchanged. New optional metadata records chunk offsets, SHA-256 state, per-chunk verification chains, CRC-32 checksums and bounded download selections.

Uploads use one durable partial file and fixed 8 MiB chunks. Each chunk is verified and fsynced before its committed offset advances. Retries reconcile against the server offset; a crash tail is truncated to that offset before another write. Browser reloads require reselecting the original file; the complete uploaded prefix is verified before continuing. Incomplete sessions reserve quota and expire after seven inactive days. Cancellation releases quota and temporary bytes. Disk admission applies to each chunk.

Directory imports, moves and batch deletion validate names, depth, cycles, conflicts and active uploads. Batch operations are atomic within one metadata write. Identical content retains organization-local deduplication.

Batch downloads use a seekable virtual ZIP64 Store archive over existing blobs, including empty directories, without creating a second archive on disk or buffering file bodies. CRC-32 for legacy blobs is calculated lazily; new uploads calculate it alongside SHA-256. HTTP Range and ETag support native browser download-manager resumption. Download selections are user-scoped, bounded and expire after 24 hours; mutations to selected content invalidate their snapshot. UI progress measures bytes sent by the server; browser download completion and local disk writes remain browser-managed on HTTP-only origins. Download control through File System Access requires a secure origin and is not assumed for intranet HTTP.

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
| `GET /downloads/{id}/file` | native streaming download, supports Range/If-Range/ETag |
| `DELETE /downloads/{id}` | removes a ticket, never the source file |

The browser fingerprints size, modification time and first/last 64 KiB. Before resume it verifies the entire committed prefix chain; completed candidates require a whole-file SHA-256 match. Hashing runs in dedicated workers without Web Crypto, so plain intranet HTTP works. At most two browser upload workers and eight server chunk streams run concurrently. Each user has at most 32 incomplete sessions and 32 download tickets per organization; organization limits are 128 of each. Import limits: 20,000 entries, 32 levels; a selection has at most 500 roots. ZIPs use Store (no recompression), with at most 4 MiB of path names. Legacy CRC preparation has two concurrent workers. HTTP delivery has eight slots. The transfer panel renders a bounded completed history.

Folder picker APIs do not expose empty directories; drag directory entries preserve them when the browser provides them. Folder imports merge exactly named existing directories and reject conflicting file names. Directory creation is atomic per 500-path batch, while each file is independently finalized: interrupted imports can leave completed directories/files and resumable sessions, never overwrite existing content. Pending uploads block moving/deleting their ancestor until finished or cancelled. Downloads are stable selections: rename/move/delete changes can invalidate a ticket; create a new selection then retry. Tickets do not retain deleted blobs. Progress receipts survive service restarts; active browser downloads use the existing URL and Range to resume.

A browser cannot reliably report its final local disk write through ordinary HTTP. The UI explicitly distinguishes server delivery from the browser download manager, which controls disk writes, download cancellation and resume. No File System Access secure-context API is required. Reverse proxies must forward PATCH, Range/If-Range and bodies, and allow at least one 8 MiB chunk; do not cache private software routes.

The directory tree renders at most 500 rows and directs users to global search for additional folders. Each listing page and move search renders at most 200 rows; completed upload history is capped at 40 rows. The directory batch uses a Unicode case-fold index, preserving existing case-conflict behavior without scanning all existing nodes for every new directory. See [the acceptance record](../verification/software-transfers-2026-10-18.md).
