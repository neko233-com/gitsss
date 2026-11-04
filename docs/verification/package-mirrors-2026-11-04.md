# Package registry mirrors acceptance — 2026-11-04

## Delivered behavior

Maven, Go Modules, PyPI/pip and npm retain the existing organization package endpoints and publishing permissions. An explicit `[packages.mirror] ENABLED = true` adds public upstream fallback and a bounded disk cache. The organization links page displays the four addresses, copy controls and client configuration. The default remains disabled even when `[packages] ENABLED = true`.

No package schema, published package storage, Git repository, ref or LFS migration is involved. Cache bytes occupy only the layout v1 `package-mirror` extension. See [configuration and compatibility](../features/workspace-centers.md#internal-package-registries-and-central-mirrors).

## Automated checks

- Windows amd64, Go 1.27.2, `CGO_ENABLED=0`: the existing Go/Maven/npm/PyPI publishing and download integration tests plus `TestPackageMirrors` passed together.
- Linux amd64 on this machine, Go 1.27.2 and GCC 12.2, `CGO_ENABLED=1`: the same five integration tests and mirror configuration tests passed.
- The Windows pure-Go SQLite executable and Linux CGO SQLite executable also built locally. These are development builds, not packaged releases.
- Cache tests passed with `-race`: cold/warm reads, persistent cache reuse, offline stale reads, concurrent fill coalescing, Range, quota eviction, invalid paths, denied redirects, file limits, signed links bound to the owner and protocol, Python metadata constraints, and 429/stale handling.
- Authorized requests do not forward account credentials upstream. Private organization reads and configured private names do not reach upstreams. A locally published Go module blocks public fallback for another version. Mirror disabling blocks reads from a populated cache.
- The repository-pinned golangci-lint v2.14.0 formatter and scoped Go lint passed with 0 issues. CSS and Markdown lint passed. `task help` worked; the Windows Task wrappers could not run Make, so their pinned Go commands were run directly against the changed areas.
- Both Vite builds and `node tools/frontend-build.mjs check` passed with Node 24.19.0 / pnpm 11.9.0. Frontend stamp: `c2a0a58529b095324b5a018764247a0dd6fab2a17276d39cc9455cd1e2ccdc94`.

All rehearsals used disposable HOME/USERPROFILE and application data. SSH authorization paths were isolated by the existing test setup. No production service or repository was switched.

## Actual package clients

The complete application HTTP router ran in an isolated local fixture server. Each round used new client caches; commands selected only the internal registry. Publishing compatibility is covered by the existing protocol tests above.

| Client / package | Round 1 | Round 2 | Artifact bytes | Final mirror response |
| --- | ---: | ---: | ---: | --- |
| npm pack `is-number@7.0.0` | 0.878 s | 0.562 s | 3,730 | HIT |
| pip download `six==1.17.0` | 1.319 s | 1.025 s | 11,050 | HIT |
| Go mod download `rsc.io/quote@v1.5.2` | 0.862 s | 0.660 s | 2,987 | HIT |
| Maven dependency:get `org.apache.commons:commons-lang3:3.17.0` | 14.900 s | 4.739 s | 673,587 | HIT |

Every artifact had the same SHA-256 in both rounds. npm tarballs and PyPI wheels were downloaded through rewritten internal URLs. The Go client retained `GOSUMDB=sum.golang.org`; application logs recorded internal sumdb support/lookup requests. The times include client startup and dependency resolution on a loopback server and do not establish LAN or production throughput.

Maven Central returned 429 to this machine's direct Go egress. Maven acceptance therefore used `https://maven.aliyun.com/repository/central` through the configurable `MAVEN_URL`. The resulting JAR's SHA-256, `6ee731df5c8e5a2976a1ca023b6bb320ea8d3539fbe64c8a1d5cb765127c33b4`, also matched a separate download from official Central through the Windows network path. The configured product default remains official Central. An uncached upstream 429 retains its status and Retry-After header; cached data remains usable during throttling.

Local raw results: `output/ui-review/package-mirrors/client-results.json`. These contain public package identities and hashes, without account credentials.

## Browser evidence and limits

Edge displayed the final page at an observed 1366×768 CSS viewport, with two columns and no document horizontal overflow. The four registry addresses and unique copy button names were visible. Copy produced the success icon. Enter expanded Maven configuration; Tab reached its copy action with visible keyboard focus. The actual light screenshot is `output/ui-review/package-mirrors/registries-light.jpg`.

The browser connection timed out during dark-theme and narrow-window verification. Those visual checks, low-height coverage, breakpoint transitions and actual 200% browser zoom are **not accepted** by this record. The screenshot and desktop interaction checks are self-review, not an independent review or complete accessibility certification.

No commit, push, release tag, package publication or production deployment was performed. The supplied port 3000 installation still requires deployment of the change and an explicit mirror configuration before exposing central mirroring to its clients.

## Additional local checks

The later local implementation adds Cargo sparse-index/crate reads and NuGet v3 flat-container reads. A separate isolated fixture server successfully served two runs of `cargo fetch` for `itoa 1.0.14` and `dotnet restore` for `Newtonsoft.Json 13.0.3` with fresh client caches. The Cargo crate SHA-256 was identical across both runs; all four NuGet packages downloaded by each restore had matching hashes. The internal index and package endpoints subsequently returned `HIT`. These are local client checks, not production acceptance; NuGet search and registration metadata remain local.

Composer 2 installed `psr/log 3.0.2` twice using a Docker client, separate empty client caches and only the internal Composer repository. Both archive SHA-256 values matched, and the organization-bound distribution URL returned `HIT`. Composer's source fallback is outside this mirror. Fixture tests cover denial of unapproved artifact hosts.

`TestPackageMirrors` passed with Linux CGO SQLite for cold/warm Cargo and NuGet requests and private-name/owner checks. A separate `TestHomeLinksPagination` integration check saved 10,000 personal links, rendered 16 on the last page, edited one and verified an adjacent entry was unchanged. Focused unit tests also covered paging, filtering, stale edits, capacity, and database compare-and-set. The incremental cache eviction tests include restart and replacement accounting.

Playwright displayed the link page at 1366 and 800 CSS pixels, added and saved a link, and displayed the registry cards in light and dark themes. A 1366×900 → 683×450 → 1366×900 viewport reflow retained an edited draft and then saved it; this is an equivalent layout simulation, not actual browser zoom. The fake service favicon produced an expected network error; the local fallback remained visible. Actual 200% browser zoom and browser automation with JavaScript disabled have not been verified. Form submissions without JavaScript were exercised by the integration test. The frontend was rebuilt and verified after the JavaScript changes. The running production registration page reports registration disabled. No production change or release was performed by these local checks.

One attempted browser check overlapped a second test process using the same SQLite fixture file and correctly encountered a stale revision. That run is excluded from acceptance. The Windows pure-Go and Linux CGO/race integration checks were then rerun serially, followed by an isolated browser save, and passed.
