# Desktop workspace and organization centers

The desktop interface has one global search bar and one workspace navigation rail. A user belonging to one organization opens that organization directly; `/?view=organizations` always opens the directory. Organization names and display notes remain visible. Repository URLs and permission checks are unchanged.

## Language package registry removal

The Go, npm, PyPI, Maven, Cargo, NuGet and Composer package-manager endpoints are removed, along with the central mirror cache integration. Type-specific management and web detail routes for these packages also return 404. Existing container, generic and other package types remain available. This intentionally removes the affected API routes; clients must stop using those URLs. Existing package database rows and blob bytes are retained, and may still appear in general historical package lists. No schema, Git, LFS or storage path changes are made. No package data or previously populated mirror cache is deleted automatically. Restore the previous binary and configuration to restore the previous routes and mirror behavior; keep the database and package storage intact when rolling back. Removing the mirror avoids its bounded cache, network requests and background work; other package traffic is unchanged.

## Organization links

The shared list and each user's personal list each hold up to 10,000 HTTP(S) links. Shared links remain site-wide and editable only by site administrators; personal links remain private to their owner. Search matches titles and URLs without case sensitivity. Pages show at most 24 matching links across both lists, and each editor modifies only that scope's visible rows. Existing links outside the page are preserved; a stale editor must reload before saving. JavaScript updates the page in place; without JavaScript a form submission returns to the list and can add one link or remove checked links.

The existing app-state keys and JSON format are unchanged; no database migration is needed. A bounded in-process cache avoids repeatedly decoding full lists, and a database revision check detects changes from other processes. Rollback preserves stored links, but older binaries retain their 32-link save limit and may render a large list at once. Reduce a list before rollback if it must remain editable with the older binary, or retain the data and upgrade again.

## Software center compatibility report

- Scope: an additive, organization-member-only file share at `/{organization}/-/software`. Default quota: 100 GiB per organization, including in-flight uploads. Organization owners can adjust it from 1 GiB to 10 TiB, but cannot set it below current usage plus reservations. Restricted accounts require organization ownership.
- Metadata: a versioned `software-center-v1-{organizationID}` record in the existing `app_state` table. No schema migration, repository checkout or Git/LFS/package storage modification.
- Bytes: `APP_DATA_PATH/gitsss/extensions/software-center/{organizationID}/`, reserved through layout contract v1. Immutable SHA-256 blobs are deduplicated within each organization; directory and filename changes update metadata only.
- Capacity: at most 20,000 entries and 32 directory levels. Logical file sizes count against quota even when bytes are deduplicated. Uploads reserve quota before streaming and are limited to eight simultaneous streams per process. A live disk admission check reserves at least 2 GiB or 5% of the volume (whichever is larger), accounting conservatively for other in-flight software uploads. Temporary uploads are removed on normal cancellation; legacy single-stream reservations expire after 24 hours; resumable sessions expire after seven inactive days. Their temporary bytes are removed on the next mutation. See [transfer compatibility](software-center-transfers.md).
- Performance: streaming uploads use bounded memory and one temporary file; finalization links the file into the blob store without copying its bytes. Downloads use authenticated HTTP Range/ETag/Last-Modified and page-managed downloads. IndexedDB caches sources and received ranges for automatic recovery; pause/resume/cancel/retry and final saving share the software transfer panel. See [browser cache limits](software-center-transfers.md#browser-local-recovery-and-saving). Listings/search never read blob contents. Read-only metadata snapshots are limited to eight organizations for 15 seconds and invalidated after every successful mutation; authorization is always checked separately. Failed blob deletion stays in a persistent cleanup queue and is retried on the next mutation.
- Operations: this local extension supports one GitSSS writer process per data directory, matching native single-process deployments. Metadata mutations are serialized per organization. Do not mount one extension directory into multiple independent writers.
- Migration: none for existing data. Back up the existing database together with this extension directory. Existing Git, LFS and package backups keep their original paths.
- Rollback: restore the previous binary; it ignores the new app_state keys and extension. Retain the extension directory and database to recover software-center content after upgrading again.
- Deletion: deleting a directory removes its subtree. A blob is reclaimed only after no remaining file in that organization references it. UI confirmation lists the selected name. Keep normal backups for accidental deletions.

## Contributions

The repository contribution report reads the default branch in one bounded Git stream and caches results by commit SHA, date range and timezone for ten minutes. It excludes merges, follows mailmap identity mappings, and uses committer timestamps. Binary changes count as files, not lines. Renames count as delete/add. Generated/vendor changes remain included. Reports are limited to 366 days and 50,000 commits; truncated results are explicitly labeled. Daily, Monday-based weekly and monthly views share the same data without another Git traversal. CSV exports escape spreadsheet formulas.

## Container registry

The existing OCI Distribution API at `/v2/` is the Docker/OCI registry. Organization packages and images share existing package permissions and content-addressed storage. Clients use `docker login HOST`, then push/pull `HOST/ORGANIZATION/IMAGE:TAG`. Use HTTPS for normal operation. An HTTP-only intranet endpoint requires an explicitly scoped client `insecure-registries` entry; the server does not weaken client TLS settings.

## Notification destinations

Push cards open a real comparison when both endpoints exist. Initial pushes open the concrete commit; deletion falls back to the repository. A card for a deleted repository cannot remain accessible after the repository is removed.
