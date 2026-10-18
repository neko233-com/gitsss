# Software transfer panel acceptance · 1.10 · 2026-10-18

This record covers an isolated desktop instance and native Edge. It contains no production account, address, repository name or credential. Production deployment requires a separate guarded backup, binary/version readback and unchanged Git/SSH/LFS/service census.

## Implemented behavior

Uploads and downloads share the software-center transfer panel. Both expose pause, resume, cancellation and failure retry, progress and speed. Files and committed download ranges are cached in IndexedDB, partitioned by user and organization. Explicitly paused tasks remain paused across reload; interrupted active tasks resume when the authenticated software center is reopened. Cached uploads verify the complete server prefix before sending more bytes.

Downloads use validated 8 MiB Range requests and atomic local chunk/offset commits. They never navigate to browser download settings. A completed transfer is **Ready to save**, followed by **Save to computer** and an explicit save confirmation that releases cache. HTTP cannot prove that the browser's final local disk save succeeded. An offered save survives refresh. Clearing completed records retains unsaved downloads.

## Local evidence

| Layer | Result |
| --- | --- |
| Backend | 363 unit-test packages passed; managed/native download receipt behavior covered |
| Frontend | 66 files, 180 tests passed; range validation, pause/resume, cancellation, restore, transient 503 retry and quota retry |
| Static gates | Go formatting/lint, ESLint, Vue/TypeScript, CSS and edited Markdown checks passed |
| Native macOS | Affected packages passed CGO SQLite with race detection and pure-Go SQLite |
| Insecure HTTP | Real Edge source caching and Worker hashing worked with `isSecureContext === false` |
| Upload recovery | 64 MiB + 313 bytes: cache source, commit at least 8 MiB, delete original source, close/reopen browser, resume without another file chooser, verify final SHA-256 |
| Legacy pending upload | Existing uncached 8 MiB prefix resumed through the original-file picker, re-cached successfully, and passed whole-file SHA-256 |
| Download recovery | Pause and reload preserve pause; resume, close/reopen browser, fetch subsequent ranges, save from panel and verify SHA-256 |
| Quota failure | Injected browser quota failure after the first 8 MiB; committed offset retained; retry completed after freeing the injected limit |
| Retry and snapshots | Two transient 503 responses retried; changed source snapshot refused rather than mixed with cached bytes |
| Cleanup | Cancel removes local task/chunks and server ticket without deleting the source; confirmed saving releases cached bytes |
| Multi-tab and isolation | Second tab waits; closing the owner permits takeover; native IndexedDB tests cover source metadata/bytes, cancelled staging, two-user scope isolation, leases and missing-chunk rejection |
| Receipt performance | Intermediate managed ranges did not persist server progress receipts; the final range did; ordinary clients retain persisted per-request receipts |
| Desktop views | 29 page families in light/dark: 58 renders, no WCAG A/AA automated findings, page errors or horizontal overflow; 1366/1600 software-panel screenshots inspected |

Screenshots in this change are dated synthetic desktop captures. Automated accessibility checks supplement keyboard and interaction checks; they do not certify every assistive-technology combination. Test transfer speeds are not a LAN performance guarantee.

## Recovery boundaries

Browser quota, private mode, site-data removal, eviction and differing browser profiles/origins can prevent recovery. Different ports are different origins. An uncached upload remains usable in its current page with an explicit warning; after reload it needs the original source. Managed downloads require writable local storage. Browser Blob saving can impose additional large-file limits. No arbitrary-size or power-loss recovery guarantee is claimed.

Browser caches are temporary recovery data, not backups. Completed uploads, confirmed saves and cancellation release task bytes. Seven-day inactive records are pruned on the next visit. A crashed tab may take up to 15 seconds to relinquish its lease. Server layout, database schema, Git/LFS paths, authorization keys and package bytes are unchanged. See [the API, cache and rollback contract](../features/software-center-transfers.md).
