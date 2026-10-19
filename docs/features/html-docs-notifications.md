# Repository HTML Docs and channel notifications

## Repository Docs

The repository sidebar includes **Docs** beside Wiki. Commit a complete static HTML site to `docs/site/index.html` (recommended), `docs/index.html`, `site/index.html`, `_site/index.html`, or the repository root. GitSSS detects these locations in that order. Repository administrators can expand **Docs settings** to select a branch, tag or commit, a relative directory, an HTML entry file, or disable the site. Empty directory means automatic discovery. Settings use optimistic revisions; a stale save returns HTTP 409.

Readers must have repository code access. Opaque frames use a signed read capability limited to one user, repository, commit and settings revision, expiring after 30 minutes; account status and current repository permissions are rechecked for each asset. Reload Docs to renew it. Treat the iframe URL as a temporary private link, and do not share it. This supports private assets even when sandbox requests cannot send SameSite cookies. Each document URL is pinned to its Git commit, so CSS, images, links and scripts stay consistent while the branch advances. Relative links work within the selected site directory. GitSSS reads blobs directly, including repository-owned LFS assets, with a 32 MiB limit per file. It does not check out or duplicate the documentation tree. Symlinks, submodules, Git metadata and parent traversal are rejected.

HTML runs in an opaque sandbox: inline/classic JavaScript, ES modules, local styles, fonts and images work. Fetch/XHR can read resources and JSON search indexes only within the same scoped documentation content directory. CORS permits opaque-origin reads of that scoped route; account and repository permissions are still checked. Application cookies, parent DOM access, form submissions, popups, nested frames, external CDN/API calls and service workers are unavailable. Bundle external dependencies locally. Use relative asset paths instead of absolute `/assets` URLs. The same restrictions apply when opening a document directly. This is static Git-hosted documentation, not a GitHub Actions or GitHub Pages deployment runner.

Routes: `GET /{owner}/{repo}/-/docs`, administrator CSRF-protected `POST` at the same path, and permission-checked `GET /{owner}/{repo}/-/docs/content/{commit}/{access}/{path}`. Administrators should review committed HTML like code. A normal `git push` publishes the next version; no workflow file is required.

## Preview any HTML file

HTML/HTM source views include **Preview HTML** immediately before Raw. A new tab pins the file and relative JS/CSS/images/modules to the viewed commit. It shares the Docs sandbox and 30-minute, user/repository/commit/revision capability, with current permissions rechecked on every request. Preview permits relative assets throughout the readable repository tree and works independently of the Docs enabled setting; it never changes that setting. Traversal past the repository root, symlinks/submodules, Git metadata and files above 32 MiB remain blocked. External APIs/CDNs, application cookies, forms/popups and parent DOM access remain blocked. Use repository-relative resource URLs. No checkout or duplicate files are created.

## Native notifications

Repository, organization and system webhooks can independently route events to **Feishu/Lark, WeCom, Telegram, Discord, Slack, DingTalk, Microsoft Teams and Matrix**. Select the provider under Settings → Webhooks. Multiple hooks deliver independently through the existing durable webhook queue. Existing event selections and branch filters remain effective. Add repository include/exclude globs (`engineering/*`, `{tools/*,services/*}`), actor globs, or skip bots for finer routing. A manual test intentionally bypasses routing filters.

Feishu push cards have separate repository, branch/tag, actor, `trackId`, CI/CD and commit-message sections. The actor shows the account login plus its `full_name` remark when different; actor routing filters continue to match the login alone. `trackId` displays the first 12 characters of the pushed head SHA and links to the full 40/64-character commit ID. It identifies the commit, not a distinct delivery: pushing the same commit again keeps the same trackId. Missing/deleted heads use the existing full delivery UUID without inventing a commit link. The message section contains only commit messages, with an overflow link prompt; commit counts, repeated hashes, authors and file lists are omitted from this compact card. The full-change button retains compare/first-push handling. Explicit custom body templates still override the default message section.

CI/CD shows the persisted GitSSS Actions runs associated with this exact push (repository, SHA, ref, push event, before/after and account), including queued/running/final states. This is the local GitHub-compatible Actions engine, not a query to github.com or an external CI provider. Lookup is read-only, bounded to 20 recent matching-head candidates and two seconds. There is no workflow execution or network polling. Because delivery may precede Actions detection, absence reads **No run recorded at delivery**, not a claim that CI was skipped; lookup errors/incomplete identity read **Unable to confirm**. The status links to the repository Actions page for current details. Cards are delivery-time snapshots and are not edited as later jobs finish; enable workflow_run/workflow_job notifications for later updates.

Other rich channels retain repository, branch/tag, actor, commit count, linked commit IDs, authors, changed files, an omitted-commit count and a button/link to the full change. Feishu uses interactive cards, Discord embeds with mentions disabled, Telegram escaped HTML and inline buttons, WeCom/DingTalk Markdown, Slack Block Kit, Teams Adaptive Cards and Matrix formatted notices. Provider-specific size budgets bound details. Include full commit bodies optionally; choose one to ten displayed commits, English or Simplified Chinese, footer and supported accent color. Advanced title/body Go templates receive `.Event`, `.Text` and `.Payload`; syntax, input and rendered output are bounded. User-supplied text is escaped for each provider. Do not put secrets in templates.

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

Feishu repository, trackId, Actions and change links use the configured public `ROOT_URL`, including its subpath. A local API request must not leak its loopback origin into shared cards. Update `ROOT_URL` when changing the public hostname; existing queued payloads are normalized at delivery.


## Per-repository Feishu card colors

Rich Feishu cards now default to a stable theme for each repository, derived locally from repository ID (full name if ID is absent). The same repository keeps its automatic color after a rename. The twelve named Feishu themes are categorical and can repeat across repositories; they do not indicate build success or failure. No database lookup or new network request is needed.

In a Feishu webhook editor, choose **Feishu card color**. A fixed theme sets the hook default. Under **Repository color overrides**, enter exact, case-insensitive rules, one per line:

```text
engineering/server = green
engineering/client = purple
```

Order: explicit repository rule, hook theme, automatic theme. At most 200 rules / 16 KiB, duplicate names and unsupported themes are rejected before saving. Existing webhooks automatically use repository colors; historical delivered cards are unchanged. Legacy text notifications are unchanged.

API: `config.notification` remains a JSON string. Add `feishu_header_color` (`auto` or a supported theme) and `feishu_repository_colors` (the multiline rules above). Partial provider metadata edits retain color rules. The existing `color` RGB setting for other channels is unchanged. Settings remain in webhook metadata; no schema/storage/SSH migration. Older binaries ignore these additive fields.


## DIY templates for all notification channels

Repository, organization and system webhook editors share **Advanced customization**. In rich mode, customize only the title/body or optionally replace the complete provider JSON message. Clear a field to restore its default. Existing `{{.Event}}`, `{{.Text}}` and `{{.Payload.repository.full_name}}` Go expressions remain valid in title/body; `{{commitMessage}}` works without a leading dot.

| Variable | Value |
| --- | --- |
| `commitMessage` | Full pushed head message, or final commit message in chronological payload order; empty for events without a commit |
| `commitMessages` | All payload commit messages separated by blank lines; not the whole repository history |
| `repository`, `repositoryUrl` | Full owner/repository and canonical site link |
| `branch` | Branch/tag name without refs prefix; workflow head branch when available |
| `actor`, `actorName`, `actorDisplay` | Git account, remark, and account plus remark; filters continue matching the account |
| `trackId`, `commitUrl` | Twelve-character new head ID and full commit link; complete delivery UUID for JSON templates without a new commit, with no commit URL |
| `compareUrl` | Canonical changes link, falling back to the new head commit |
| `commitAuthor`, `timestamp` | Head author and original payload commit timestamp, without invented delivery time |
| `event`, `action`, `text` | Event kind, action, and default formatted text |
| `title`, `body`, `footer`, `color` | JSON-only: computed presentation text and Feishu repository theme |

Title example: `{{repository}} · {{branch}}`. Body example: `{{actorDisplay}}
{{commitMessage}}
{{commitUrl}}` (enter actual newlines in the editor).

### Native JSON examples

Use the example for the selected channel. Placeholders are context-aware: both `"{{commitMessage}}"` and `{{json commitMessage}}` produce correctly escaped JSON strings, including quotes, backslashes, Unicode and newlines. Values are never executed as templates. JSON templates support named placeholders only; conditions/loops remain available in title/body. Missing event-specific values are empty strings. JSON escaping does not escape a channel's Markdown/HTML language: use plain-text elements for untrusted commit messages, or deliberately design and validate your channel formatting.

**Feishu** (interactive card; repository theme remains available):

```json
{"msg_type":"interactive","card":{"header":{"template":"{{color}}","title":{"tag":"plain_text","content":"{{repository}} · {{branch}}"}},"elements":[{"tag":"div","text":{"tag":"plain_text","content":"{{actorDisplay}}\n{{commitMessage}}"}},{"tag":"note","elements":[{"tag":"plain_text","content":"trackId: {{trackId}}"}]}]}}
```

**Telegram** (plain text, configured chat/topic destination):

```json
{"text":"{{repository}} · {{branch}}\n{{actorDisplay}}\n{{commitMessage}}\n{{compareUrl}}","disable_web_page_preview":true}
```

**Discord**:

```json
{"content":"{{repository}} · {{branch}}\n{{commitMessage}}\n{{compareUrl}}"}
```

**WeCom / DingTalk**:

```json
{"msgtype":"text","text":{"content":"{{repository}} · {{branch}}\n{{commitMessage}}\n{{compareUrl}}"}}
```

**Slack** (plain-text Block Kit):

```json
{"text":"{{repository}}","blocks":[{"type":"section","text":{"type":"plain_text","text":"{{actorDisplay}}\n{{commitMessage}}"}}]}
```

**Microsoft Teams**:

```json
{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","content":{"type":"AdaptiveCard","version":"1.4","body":[{"type":"TextBlock","text":"{{repository}} · {{branch}}","weight":"Bolder","wrap":true},{"type":"TextBlock","text":"{{commitMessage}}","wrap":true}]}}]}
```

**Matrix**:

```json
{"msgtype":"m.notice","body":"{{repository}} · {{branch}}\n{{actorDisplay}}\n{{commitMessage}}\n{{compareUrl}}"}
```

Templates are scoped per webhook. Source is limited to 16 KiB and rendered JSON to 64 KiB. Save checks syntax, known variables, nonempty JSON object and required provider envelope fields. Rendering rechecks real event data before any request; errors stop delivery rather than silently sending a broken or fallback message. Provider-specific size/schema rules still apply and provider rejection is recorded. Native JSON is rich-mode only.

Secrets, endpoint URLs, Feishu signatures, Telegram chat/topic routing, Slack destination and Matrix transaction handling remain server-managed. Discord templates cannot enable automatic mass mentions. No filesystem, network, shell, environment or secret variables are exposed. Existing retry/delivery/event filtering contracts stay unchanged.

REST: send `config.notification` as the existing JSON string, adding `title_template`, `message_template` and/or `payload_template`. Escape the template string with your JSON library. Channel-only API edits preserve templates. Storage uses existing webhook metadata with no schema/Git/LFS/SSH migration. For rollback, export webhook metadata privately; clear `payload_template` before reverting to an older binary if custom formatting must not revert to defaults.


## Independent channel policies and native email

Add one webhook per channel from the same repository/organization's webhook list. Each row has its own enabled state, events, branch/repository/actor conditions, tag rule, templates and delivery history. Feishu failure does not cancel an independent email task. Multiple channels can observe the same event. API contracts and existing webhook IDs remain unchanged.

Example: Feishu uses **Selected events below** plus **All events**. Add an **Email** channel with **Published stable releases only**, tag rule `v*` and the **Release** event. This sends the first published stable release matching the tag, not ordinary pushes, draft/prerelease versions, release edits or deletions. Alternatively **New tags only** plus **Create** sends tag creation only, avoiding a second notification for the tag push/release. All filters are AND conditions; a tag rule suppresses events without a tag. Explicit manual test delivery bypasses event/routing conditions and must be used deliberately.

Native email uses the site's existing `[mailer]` SMTP/sendmail configuration. It does not accept SMTP passwords in webhook metadata. Enter 1–20 comma-separated recipients; addresses are validated and deduplicated. One envelope per recipient hides the list from other recipients. Default email has a compact HTML layout and text alternative. Native JSON DIY is `{"subject":"{{repository}} · {{event}}","text":"{{body}}\n{{compareUrl}}"}`; subject must be one line, and text is escaped before HTML rendering. No custom executable HTML is accepted.

Disabled, uninitialized or dummy mail transport fails explicitly. SMTP webhook calls use a bounded timeout matching webhook delivery timeout (minimum one second); existing ordinary email behavior stays unchanged. Delivery succeeds when the configured transport accepts the message, not when a person receives/reads it. SMTP rejection/timeouts fail that channel task. Partial recipient failures may mean earlier recipients already received mail; manual retry can duplicate those messages, so inspect delivery status before retrying. There is no automatic resend after ambiguous acceptance.

API hook type `email` accepts `config.recipients` and the existing `config.notification` JSON string. Its URL is an internal `mailto:configured-smtp` marker, not a network HTTP endpoint. `event_policy` accepts `all`, `formal_release`, `tag_created`; `tag_filter` is an optional glob, maximum 512 bytes. Normal events/subscriptions still apply. Additive type/metadata requires no database/schema/Git/SSH/LFS migration. Disable email hooks before rolling back to a binary that does not know this type; export private settings first.

## Frontend resource integrity

Vite records a SHA-256 of frontend source/configuration inputs in `public/assets/frontend-build.json`. Before embedding assets, `go generate -tags bindata ./modules/public` runs `node tools/frontend-build.mjs check`. Stale source stamps, missing manifest outputs or missing workspace/search styles fail the build. Build both normal and React bundles with the package.json Node/pnpm engine versions. Build tools are not server runtime dependencies. Acceptance compares the embedded build stamp and manifest CSS/JS bytes served by the candidate, then inspects real browser layout and interactions. A new Go template with an old asset bundle is not a valid deployment.


## Complete messages and formatted email

Rich channels always retain every supplied commit message, including paragraphs, Unicode and line breaks. Legacy `include_commit_body` and `max_commits` metadata remain readable for compatibility but no longer hide body lines or supplied commits in rich mode. If the source event itself omits commits, the notification explicitly says so and links to changes; GitSSS never invents missing text.

Messages exceeding provider limits are split into numbered UTF-8 parts before sending. Feishu uses a conservative **19,000 encoded bytes** below its [20 KB webhook limit](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot); Telegram stays below 4,096 characters, Slack section text below 3,000, and Discord embed description below 4,096. Teams, Matrix, DingTalk, WeCom and email have independent envelope/text checks. No fixed card height or server-side ellipsis is applied to message bodies. Slack asks the client to expand sections; provider clients may still apply their own display rules.

Parts are sent sequentially with channel pacing. A failure stops later parts and records accepted/total counts; GitSSS does not automatically resend an ambiguous partial delivery. Inspect the recorded count before manually redelivering because previously accepted parts can be duplicated. Each part keeps the delivery ID and commit trackId; request headers record part numbers and SHA-256 fingerprints. Matrix transaction IDs include the original delivery ID and part number. Safety limits are 1 MiB body and 128 parts; impossible fixed templates or over-limit inputs fail explicitly before any part is sent.

JSON DIY templates can use `{{page}}` and `{{pages}}`. During pagination, `{{commitMessage}}`, `{{commitMessages}}`, `{{body}}` and `{{text}}` refer to the current fragment, so include one of these in the main content. Fixed metadata is repeated. Templates that reference the entire `.Payload` in Go templates remain bounded to 64 KiB rendered output and report an explicit error. JSON source remains limited to 16 KiB. Escape content for the channel's markup when authoring native JSON templates; JSON escaping alone does not escape HTML/Markdown.

Native emails use an inline-styled, table-based HTML layout compatible with common desktop mail clients: GitSSS identity, event heading, repository, branch/tag, actor and remark, linked trackId, Actions snapshot, timestamp, full multi-line details, and a changes button. Untrusted text is HTML-escaped and button URLs accept HTTP(S) only. No remote images, scripts or tracking pixels are needed. DIY `subject`/`text` retains the styled wrapper. Subject/metadata stay concise; full details remain in the body. SMTP acceptance and actual inbox arrival are separate verification stages.


## Product identity

The default logo and SVG/PNG favicons use an original vector **SSS / Git** mark. High contrast blue/white remains visible in light and dark navigation. Generated Actions status badges continue to compose over the favicon. The frontend source fingerprint covers all four logo/favicon files as well as UI source, preventing an older embedded logo bundle from passing the build gate. Existing custom deployment logos remain an operator override.

Notification settings validation errors appear on the same HTTP 422 page, with channel recipients and template values retained. Invalid templates never replace saved hooks.
