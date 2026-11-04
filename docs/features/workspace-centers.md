# Desktop workspace and organization centers

The desktop interface has one global search bar and one workspace navigation rail. A user belonging to one organization opens that organization directly; `/?view=organizations` always opens the directory. Organization names and display notes remain visible. Repository URLs and permission checks are unchanged.

## Internal package registries and central mirrors

The organization links page (`/org/{owner}/dashboard?tab=links`) includes Maven, Go Modules, PyPI/pip, npm, Cargo, NuGet and Composer endpoints with copyable client configuration and the configured mirror status. These are the existing package registries, with local packages taking precedence. Publishing still uses their existing authenticated upload APIs.

### Organization links

The shared list and each user's personal list each hold up to 10,000 HTTP(S) links. Shared links remain site-wide and editable only by site administrators; personal links remain private to their owner. Search matches titles and URLs without case sensitivity. Pages show at most 24 matching links across both lists, and each editor modifies only that scope's visible rows. Existing links outside the page are preserved; a stale editor must reload before saving. JavaScript updates the page in place; without JavaScript a form submission returns to the list and can add one link or remove checked links.

The existing app-state keys and JSON format are unchanged; no database migration is needed. A bounded in-process cache avoids repeatedly decoding full lists, and a database revision check detects changes from other processes. Rollback preserves stored links, but older binaries retain their 32-link save limit and may render a large list at once. Reduce a list before rollback if it must remain editable with the older binary, or retain the data and upgrade again.

An administrator enables central mirroring in `app.ini` and restarts GitSSS:

```ini
[packages.mirror]
ENABLED = true
MAVEN_URL = https://repo.maven.apache.org/maven2
GO_URL = https://proxy.golang.org
; Optional externally reachable HTTPS base for Go clients when the main site uses HTTP.
GO_CLIENT_BASE_URL = https://git.example.com:3443
GO_FILE_HOSTS = storage.googleapis.com
PYPI_URL = https://pypi.org/simple
NPM_URL = https://registry.npmjs.org
CARGO_INDEX_URL = https://index.crates.io
CARGO_FILES_URL = https://static.crates.io/crates
NUGET_URL = https://api.nuget.org/v3-flatcontainer
COMPOSER_URL = https://repo.packagist.org
COMPOSER_FILE_HOSTS = api.github.com,codeload.github.com,github.com,gitlab.com,bitbucket.org
PYPI_FILE_HOSTS = files.pythonhosted.org
NPM_FILE_HOSTS =
PRIVATE_PATTERNS = github.com/your-company/**,com.yourcompany:*,@your-company/*
METADATA_TTL = 5m
ARTIFACT_TTL = 168h
TIMEOUT = 2m
MAX_SIZE = 10737418240
MAX_FILE_SIZE = 536870912
MAX_ENTRIES = 50000
```

URLs can point to an administrator-selected compatible mirror, including an internal HTTP upstream. They must not contain credentials, query strings or fragments. File hosts are exact names (optionally with a port), never wildcards. The upstream host is automatically allowed; extra artifact/CDN hosts must be listed explicitly. The Go default allows the official proxy's `storage.googleapis.com` ZIP redirects. Redirects obey the same allowlist. GitSSS account tokens and cookies are never sent to upstreams. Existing outbound proxy settings apply. An upstream 429 is returned with `Retry-After` unless an older cached copy is available; configure another compatible upstream or the site's outbound proxy when the origin limits your egress IP.

The default is **disabled** to preserve existing installations' network and missing-package behavior. An enabled mirror resolves local packages first. A known local package name never falls back to public versions or missing public files. `PRIVATE_PATTERNS` also reserves names not yet published; the organization's npm scope and the GitSSS hostname's Go module paths are reserved automatically. Configure company Maven coordinates, Python names and other private Go paths before enabling. Patterns use GitSSS glob syntax (`**` spans path segments).

| Client | Organization registry | Client configuration |
| --- | --- | --- |
| Maven / Gradle | `https://git.example.com/api/packages/acme/maven` | Set a Maven mirror with `mirrorOf=central`; keep the same URL for your private repository and distribution management. |
| Go Modules | `https://git.example.com/api/packages/acme/go` | Set `GOPROXY` to this URL; published ZIP uploads use `PUT .../go/upload`. |
| pip / Twine | `https://git.example.com/api/packages/acme/pypi` | pip uses `.../pypi/simple/`; Twine uploads to `.../pypi`. |
| npm / pnpm | `https://git.example.com/api/packages/acme/npm/` | Set the default `registry` to this URL so both public and private dependencies use it. |
| Cargo | `sparse+https://git.example.com/api/packages/acme/cargo/` | Configure a Cargo sparse source replacement and the `gitsss` registry; use `cargo login --registry gitsss` for a private organization. |
| NuGet / dotnet | `https://git.example.com/api/packages/acme/nuget/index.json` | Add a NuGet v3 source. Package versions and package files use the flat-container mirror. |
| Composer | `https://git.example.com/api/packages/acme/composer` | Configure a Composer v2 repository and disable direct Packagist access for the project. |

Use a package-read token for authenticated downloads and a package-write token for publishing. Store credentials in the client's normal credential file/environment, never in registry URLs, project source or logs. Maven uses a `<server>` whose ID matches the mirror/repository ID. npm uses the registry-path-scoped `_authToken` in the user `.npmrc`; pip uses a keyring or a protected `.netrc`. Composer supports a protected `auth.json` or `COMPOSER_AUTH`; NuGet supports its credential providers. Go's `.netrc` and `GOAUTH` apply only to HTTPS: an authenticated HTTP `GOPROXY` will not work, and `GOINSECURE` does not change this proxy behavior. If the main site uses HTTP, configure a trusted HTTPS reverse proxy and set `GO_CLIENT_BASE_URL` to its HTTPS base, including any application subpath. Trust its certificate on each client before use. For internal Go modules set `GOPRIVATE`/`GONOSUMDB` to the private module patterns and `GONOPROXY=none` if those modules are served by this proxy. Public modules retain checksum verification; the proxy forwards the signed `sumdb/sum.golang.org` protocol. An HTTP pip registry requires `--trusted-host` (shown in the page's configuration).

For Go 1.27, a `.netrc` entry for a proxy using a non-default port must match the full host and port, for example `machine git.example.com:3443`. Keep the token in the protected credential file. `GO_CLIENT_BASE_URL` only changes the displayed Go client address; the reverse proxy must forward the package path and authorization headers to GitSSS without caching private responses.

Maven example in `settings.xml` (inside `<settings>`):

```xml
<mirrors>
  <mirror>
    <id>gitsss</id>
    <mirrorOf>central</mirrorOf>
    <url>https://git.example.com/api/packages/acme/maven</url>
  </mirror>
</mirrors>
```

```sh
go env -w GOPROXY=https://git.example.com/api/packages/acme/go
python -m pip config set global.index-url https://git.example.com/api/packages/acme/pypi/simple/
npm config set registry https://git.example.com/api/packages/acme/npm/
```

Do not add pip's public `--extra-index-url` for private packages: use the single internal index. npm tarballs and PyPI file/PEP 658 metadata links are rewritten to owner-bound, signed internal URLs; file reads still require the owner's package permissions. Hashes, npm integrity, Python version constraints and yanked markers are preserved. Maven files/checksums, Go escaped module paths and Cargo index checksums are passed through without changing bytes. NuGet mirror support covers the v3 flat-container version list and package files; search and registration metadata remain local. Composer v2 metadata rewrites approved distribution archives to signed, organization-bound URLs. Distribution hosts and redirects must match `COMPOSER_FILE_HOSTS`; unapproved hosts fail closed. Composer's source checkout fallback is not mirrored and can reach the original Git host; use network controls if all dependency traffic must remain internal. A successful cold response reports `X-GitSSS-Mirror: MISS`, a cached response `HIT`, and an expired cached response used during an upstream/network failure `STALE`. Upstream 404/410 responses are not cached and do not resurrect stale entries. TTL expiry triggers a new upstream read; an unexpired cached dependency is available without an upstream connection. This is demand caching, not a full synchronized copy of any central registry. npm search/audit and publishing remain the existing local registry operations.

### Mirror compatibility, capacity and rollback

- API/schema: existing package publish/download URLs, access checks, package database and blob storage are unchanged. New signed artifact paths live under the same authorized registry. No schema migration.
- Storage: only the `package-mirror` reservation and `APP_DATA_PATH/gitsss/extensions/package-mirror/` are added to layout v1. Git repositories, refs, LFS and published package bytes are untouched. Unknown layout versions stop before cache creation.
- Capacity/performance: finalized cache entries are bounded by `MAX_SIZE` (10 GiB default), including their 4 KiB headers, and `MAX_ENTRIES` (50,000 default), and evicted oldest-first. A process indexes existing entries once, then maintains the capacity index incrementally. Use one GitSSS writer per cache directory; independent processes sharing a directory do not coordinate quota. Individual files are capped at 512 MiB by default; metadata is additionally capped at 32 MiB. Up to four upstream fills run concurrently; identical downloads share a fill. Temporary download space can add up to four file limits. Allow headroom for these files. Cache hits stream from disk and support HTTP Range/HEAD; metadata is rewritten per organization. Central cache bytes are shared, but authorization is checked on every request.
- Rollback: disable `[packages.mirror] ENABLED` or restore the previous binary. Existing private packages continue using the same storage. The cache is disposable and can be removed while the service is stopped; no Git/LFS/database migration is required.
- Verification: run the scoped cache/settings tests and `TestPackageMirrors` with both pure-Go SQLite and CGO SQLite. Real-client/central-network acceptance is recorded separately from fixture tests and requires clean client caches.

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
