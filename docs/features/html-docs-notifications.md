# Repository HTML Docs and channel notifications

## Repository Docs

The repository sidebar includes **Docs** beside Wiki. Commit a complete static HTML site to `docs/site/index.html` (recommended), `docs/index.html`, `site/index.html`, `_site/index.html`, or the repository root. GitSSS detects these locations in that order. Repository administrators can expand **Docs settings** to select a branch, tag or commit, a relative directory, an HTML entry file, or disable the site. Empty directory means automatic discovery. Settings use optimistic revisions; a stale save returns HTTP 409.

Readers must have repository code access. Opaque frames use a signed read capability limited to one user, repository, commit and settings revision, expiring after 30 minutes; account status and current repository permissions are rechecked for each asset. Reload Docs to renew it. Treat the iframe URL as a temporary private link, and do not share it. This supports private assets even when sandbox requests cannot send SameSite cookies. Each document URL is pinned to its Git commit, so CSS, images, links and scripts stay consistent while the branch advances. Relative links work within the selected site directory. GitSSS reads blobs directly, including repository-owned LFS assets, with a 32 MiB limit per file. It does not check out or duplicate the documentation tree. Symlinks, submodules, Git metadata and parent traversal are rejected.

HTML runs in an opaque sandbox: inline/classic JavaScript, ES modules, local styles, fonts and images work. Fetch/XHR can read resources and JSON search indexes only within the same scoped documentation content directory. CORS permits opaque-origin reads of that scoped route; account and repository permissions are still checked. Application cookies, parent DOM access, form submissions, popups, nested frames, external CDN/API calls and service workers are unavailable. Bundle external dependencies locally. Use relative asset paths instead of absolute `/assets` URLs. The same restrictions apply when opening a document directly. This is static Git-hosted documentation, not a GitHub Actions or GitHub Pages deployment runner.

Routes: `GET /{owner}/{repo}/-/docs`, administrator CSRF-protected `POST` at the same path, and permission-checked `GET /{owner}/{repo}/-/docs/content/{commit}/{access}/{path}`. Administrators should review committed HTML like code. A normal `git push` publishes the next version; no workflow file is required.

## Native notifications

Repository, organization and system webhooks can independently route events to **Feishu/Lark, WeCom, Telegram, Discord, Slack, DingTalk, Microsoft Teams and Matrix**. Select the provider under Settings → Webhooks. Multiple hooks deliver independently through the existing durable webhook queue. Existing event selections and branch filters remain effective. Add repository include/exclude globs (`engineering/*`, `{tools/*,services/*}`), actor globs, or skip bots for finer routing. A manual test intentionally bypasses routing filters.

Rich presentation provides repository, branch/tag, actor, commit count, linked commit IDs, authors, changed files, an omitted-commit count and a button/link to the full change. Feishu uses interactive cards, Discord embeds with mentions disabled, Telegram escaped HTML and inline buttons, WeCom/DingTalk Markdown, Slack Block Kit, Teams Adaptive Cards and Matrix formatted notices. Provider-specific size budgets bound details. Include full commit bodies optionally; choose one to ten displayed commits, English or Simplified Chinese, footer and supported accent color. Advanced title/body Go templates receive `.Event`, `.Text` and `.Payload`; syntax, input and rendered output are bounded. User-supplied text is escaped for each provider. Do not put secrets in templates.

Telegram uses the existing bot endpoint URL with `chat_id` and optional `message_thread_id` query parameters; Matrix retains its homeserver/room endpoint and authorization header. Channel URLs and bot credentials remain administrator-only configuration. Delivery history and errors redact native notification endpoint paths and queries. A provider JSON failure code marks delivery failed even if HTTP status is 200. View delivery details, fix configuration and manually redeliver as needed; automatic retries are not added because a timeout may already have delivered a message. Rate limits and retry delays are supplied by the provider response. Keep duplicate legacy hooks inactive when moving a destination to the richer presentation.

The existing REST hook API accepts a JSON string in `config.notification`:

```json
{"type":"feishu","active":true,"events":["push"],"config":{"url":"https://notification.example.com/your-private-hook","content_type":"json","notification":"{\"style\":\"rich\",\"language\":\"en-US\",\"max_commits\":5,\"include_files\":true,\"repository_filter\":\"engineering/*\"}"}}
```

Partial config edits preserve stored provider metadata. `GET` returns the nonsecret notification presentation/routing settings. Authorization headers and bot-token metadata are not exported. The URL itself remains part of the existing administrator API contract; protect API output as configuration. Legacy presentation is available for compatibility; version-1 queued payloads retain their original delivery format.

Provider references: [Feishu custom bots](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot), [Telegram Bot API](https://core.telegram.org/bots/api), [Discord webhook embeds](https://discord.com/developers/docs/resources/webhook), [WeCom robots](https://developer.work.weixin.qq.com/document/path/91770).

## Avatars and email

Settings → Profile offers style tabs for anime, fantasy adventure, Adventurer, Lorelei, username initials and custom upload. The catalog contains **30 local 256×256 presets**: the original 24 retain their indexes and attribution; six new original illustrations are distributed under this repository's MIT license. Fantasy adventure is an original anime aesthetic and does not bundle third-party game characters. No external avatar service is needed. Extend the embedded manifest with a new style and append assets without reassigning existing indexes. Username initials use deterministic local rendering and eight palettes. Existing custom uploads keep the shared automatic square crop and compression pipeline.

Settings → Account adds a direct primary-email edit. Addresses are globally unique, case-insensitively, including other accounts' aliases and unconfirmed addresses. The old address remains an alias so commit attribution is preserved; users can remove nonprimary aliases separately. With email confirmation enabled, the primary address changes only after verification and confirmation sends are throttled. Existing login and notification email contracts remain in place.

## Compatibility and rollback

No database schema, Git/LFS path, Git layout v1, object bytes, refs, account identifiers, keys, SSH port or Actions storage changes are required. Router and access logging redact the temporary Docs capability; test setup pins SSH files to its private data directory with authorization-file generation disabled. Docs settings use the existing `app_state` store and notification settings use existing webhook metadata. The additional avatar assets are embedded in the binary. Downgrading to the previous binary leaves these settings inert and preserves repositories/LFS. Previously queued version-1 webhooks retain their format; rich version-2 tasks are rendered at delivery time. Retain a normal database/configuration backup before deployment. Do not restart unrelated build/game services to validate this update.
