# Desktop workspace and organization centers

The desktop interface has one global search bar and one workspace navigation rail. A user belonging to one organization opens that organization directly; `/?view=organizations` always opens the directory. Organization names and display notes remain visible. Repository URLs and permission checks are unchanged.

## Software center compatibility report

- Scope: an additive, organization-member-only file share at `/{organization}/-/software`. Default quota: 100 GiB per organization, including in-flight uploads. Organization owners can adjust it from 1 GiB to 10 TiB, but cannot set it below current usage plus reservations. Restricted accounts require organization ownership.
- Metadata: a versioned `software-center-v1-{organizationID}` record in the existing `app_state` table. No schema migration, repository checkout or Git/LFS/package storage modification.
- Bytes: `APP_DATA_PATH/gitsss/extensions/software-center/{organizationID}/`, reserved through layout contract v1. Immutable SHA-256 blobs are deduplicated within each organization; directory and filename changes update metadata only.
- Capacity: at most 20,000 entries and 32 directory levels. Logical file sizes count against quota even when bytes are deduplicated. Uploads reserve quota before streaming and are limited to eight simultaneous streams per process. A live disk admission check reserves at least 2 GiB or 5% of the volume (whichever is larger), accounting conservatively for other in-flight software uploads. Temporary uploads are removed on normal cancellation; abandoned reservations expire after 24 hours and their temporary bytes are removed on the next mutation.
- Performance: streaming uploads use bounded memory and one temporary file; finalization links the file into the blob store without copying its bytes. Downloads use authenticated HTTP Range/ETag/Last-Modified and browser downloads. Listings/search never read blob contents. Read-only metadata snapshots are limited to eight organizations for 15 seconds and invalidated after every successful mutation; authorization is always checked separately. Failed blob deletion stays in a persistent cleanup queue and is retried on the next mutation.
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
