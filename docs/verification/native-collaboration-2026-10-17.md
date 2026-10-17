# Native collaboration verification · 2026-10-17

## Delivered surface

- Git-backed HTML Docs in repository navigation, with pinned resources, LFS, classic scripts, ES modules and scoped JSON search indexes.
- Rich native notification presentation and per-hook policies for Feishu, WeCom, Telegram, Discord, Slack, DingTalk, Teams and Matrix.
- Thirty stable local avatar presets grouped by style, generated username initials, custom crop/compression and unique primary-email editing.
- Readable dark branding/links and the Organization links label.

## Local gates and observed behavior

- The Make backend unit-test package selection passed for 362 packages; affected webhook tests were rerun after the final delivery guard.
- Frontend: 61 files / 165 tests passed. Go lint, the Make ESLint target, Vue/TypeScript checking and CSS lint passed.
- Native macOS CGO race checks and pure-Go SQLite checks passed for the affected services, settings, authorization and database packages.
- Edge verified style-tab switching, preset and initials saves, original-image upload with crop preview and compressed save, primary-email editing, dark text/branding, notification form persistence and REST configuration edits.
- Docs loaded local CSS and classic scripts, ran an ES module with a fetched JSON index, searched in-page and navigated between HTML pages within the repository layout. The parent application remained isolated. LFS content matched the original bytes and SHA256. Tampering, directory escape, stale settings and access after permission revocation were rejected.
- The synthetic Feishu card was delivered by the native macOS service with HTTP 200 and a successful provider business code. Endpoint paths were redacted in recorded delivery history. Other channel payloads and provider rejection handling passed local request/delivery tests; external destinations require their own credentials.
- Desktop dark-mode measurements: navigation brand contrast 11.85:1, footer links 8.47:1 and the main heading 12.09:1 in the tested Dark theme.
- All ten supported native target variants were compiled locally. Twenty checksums, ten XZ round trips and ten executable architectures were checked, plus a Windows executable version smoke check.

Screenshots in `docs/site/assets/screenshots/html-docs.png` and `avatar-styles.png` use an isolated synthetic instance. Production addresses, credentials, repository names and notification URLs are excluded from this repository; operational receipts remain outside Git.

## Compatibility and rollback

No schema, Git layout v1, repository/LFS bytes, storage roots, SSH account, host key or Actions storage migration is introduced. Docs settings use existing app_state records; notification presentation uses existing webhook metadata. Router/access logs redact scoped Docs URLs. Test setup explicitly pins SSH authorization files to its private test-data directory and disables automatic authorization-file generation; this only changes test initialization.

Keep the previous binary and a protected database/configuration backup. Rollback replaces the binary; retained settings become inactive on older versions. No unrelated build or game process needs a restart. The full integration suite requires its own fixture configuration; the unit-test gate follows Make's explicit package selection.
