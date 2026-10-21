# Workspace navigation and date/time controls

## Organization service links

Organization links sit directly below Repositories in the organization sidebar, with a colored link icon. Service cards show the name, address, shared/personal scope and a new-tab action. Favicons are requested lazily by the visitor's browser from the target origin's `/favicon.ico`, without URL query strings, credentials or referrers. The GitSSS server does not fetch or proxy link targets. Unavailable icons, mixed-content HTTP targets on HTTPS pages and disabled JavaScript retain bundled fallback icons. Link destinations, visibility and editor permissions remain unchanged; saving refreshes icons while preserving another editor's draft.

## Global search

Press **Ctrl+K** or **⌘K** from a signed-in desktop page, or click the header search. Native dialog focus management and GitHub's MIT-licensed `@github/combobox-nav` provide Escape, Tab, arrow keys and Enter behavior. Slash retains the existing header focus shortcut. Opening search does not discard an editor draft; another active dialog keeps its focus.

Search covers permission-filtered repositories, issues/pull requests, the first 50 member organizations, and current workspace navigation. The read-only browser-session search endpoint returns at most eight repositories and eight work items per query; at most 30 distinct results are displayed. The full repository/code search links remain available. Organization-board card contents, software file contents and complete code indexes are not included in inline results. Search labels and empty/loading/error states follow the interface language. Requests debounce by 200 ms, are aborted when superseded/closed, and late results cannot replace newer ones. Queries/results stay in memory only. The browser-session endpoint uses existing repository and issue permissions without exporting an API token; no new search index or data store is created. Result links use the current host/port alias and reject external origins, credential-bearing and executable URLs.

## Date and time

Date and datetime fields use the MIT-licensed [Flatpickr](https://flatpickr.js.org/options/) package bundled locally. Its JavaScript, locale catalog and CSS load only on pages with a date field or when a dynamic date field receives focus. No CDN requests are made. Calendar language follows the interface language; theme tokens keep light/dark surfaces readable. Month/year selection, keyboard navigation, today/now and optional clearing share one control. Datetime controls show the browser's local timezone and use 24-hour time with minute precision.

Names and ISO date/local-datetime values submitted to existing forms stay unchanged. The original input type is retained in `data-picker-type` when the library enhances it to text. Native date parsing plus required/min/max/step validation remains enforced, and administrator configuration still maps wall-clock local values to Unix timestamps. Clearing an optional administrator datetime explicitly sends zero. Failed enhancement restores the native input. Git, SSH, LFS, storage and database contracts are unaffected; rollback is the previous binary.

## Actions traceability

Actions list and summary expose the commit as **trackId**, linking to the full commit, plus the trigger event and account/remark. The summary shows the first 12 SHA characters; the full SHA remains in the existing commit URL. Current/historical attempts retain their own trigger user, time and run links, so a rerun is distinguishable from the original push. These are GitSSS's GitHub-compatible Actions records. Looking at a run does not trigger a workflow or query external CI.

See [HTML Docs and notifications](html-docs-notifications.md) for per-file HTML preview and the compact Feishu card. Frontend checks use isolated accounts/HOME and real desktop Edge, with keyboard interactions, both themes, request races, access restrictions and console/asset health checked before deployment. Automated accessibility scans supplement interaction checks; they do not establish universal usability or a full accessibility certification.

## Activity cards

Organization, personal and profile activity feeds share compact cards. Push headers show the account and its remark (`full_name`) plus the full repository name. Branch and time have separate labeled fields; the message area contains readable, wrapping commit subjects without repeated avatar/hash prefixes. Each message retains a keyboard-accessible commit link, and multi-commit pushes retain their comparison link. Existing feed permissions, pagination and stored events are unchanged.

Activity time uses the browser's local timezone, a fixed 24-hour `yyyy-MM-dd HH:mm:ss` timestamp and a localized short weekday, followed by ` | ` and a live relative label. For example, `2026-10-16 09:08:07 Fri | 2 days ago` or `2026-10-16 09:08:07 周五 | 前天`. Yesterday and the day before yesterday use local calendar days, including across midnight and daylight-saving transitions. A shared timer refreshes older activity labels at most once per minute; no polling or per-card network requests are added. The accessible tooltip includes the timezone. Without JavaScript, the exact timestamp and UTC offset remain visible. Other date controls retain their existing format.

## Activity heatmap

The calendar fills the center column without enlarging its labels. Summary values show the last 365 local calendar days (including today), last 7/30 days, active days and peak daily count. Padding before the window and future days do not count; the older API's extra leading-week data no longer inflates the displayed yearly total. These are existing permission-filtered activity events, including pushes and collaboration actions, grouped by action time. They are not commit counts, changed lines, working hours or productivity scores.

The legend names every inclusive daily range: `0`, `1–4`, `5–9`, `10–19`, `20–49`, `50+`. Display settings let users edit the five positive-level colors and increasing integer lower bounds (1 to 1,000,000, first bound fixed at 1), apply or reset them. Preferences stay in local browser storage and are shared between the user's charts on that origin; they do not alter server data or other browsers. Storage-disabled browsers retain preferences for the current view. Labels, numbers and date/count tooltips keep the chart understandable without distinguishing colors. Tab enters the calendar once; arrows move by day/week, Home/End reach the window edges, and Enter/Space applies the existing date filter. Team filtering replaces and unmounts the old chart. The grid uses one bounded pass over the data and 365 day buttons, with no per-cell requests or tooltip instances.

## Custom repository tags

Repository administrators use **Manage tags** in the repository About sidebar to add/remove up to 25 custom tags; press Enter to add a value and Save to apply. Tags support Chinese and other lowercase Unicode letters/numbers plus `_`, `-` and `.`, start with a letter/number and contain at most 35 Unicode characters. Names are trimmed, lowercased and deduplicated by the existing topic sanitizer. Invalid values remain editable with an inline error; network/server failures retain input and allow retry. Archived repositories stay read-only.

Ordinary organization/personal repository search, Explore and Ctrl+K repository results already share the permission-filtered topic query, so a tag or part of a tag matches without a special prefix. Clicking a tag uses the existing exact topic filter. Repository list permissions remain authoritative; tags do not grant access. Storage tables, repository `topics` metadata and `/api/v1/repos/{owner}/{repo}/topics` remain compatible, with Chinese and underscore names accepted additively. No extra label database or index is created. Issue labels remain separate.

Tag suggestions and their repository counts now include only repositories visible to the caller, including public-only API token restrictions. This closes the previous global suggestion leak of private-only tag names. Cancel restores the saved selection. Rollback preserves the same data; an older binary can display new Unicode tags but may reject editing names outside its older ASCII validator.

## Entrance and personal context

The signed-out entrance uses one locally bundled Paper Shaders canvas, capped below 300,000 pixels. Reduced-motion preference starts it paused; users can pause/resume it. The library suspends rendering offscreen/in the background, and WebGL failure falls back to a static SVG. No shader library is requested on signed-in pages. Organization-first navigation also includes the current account as `username (Personal)` alongside the organization list; the existing personal URL remains `/?view=personal`. Code, diff, blame and editor comments, including documentation comments, share neutral gray tokens in each theme.

The embedded offline manual also includes its linked feature contracts and acceptance reports. These text-only directories are bundled with the binary; runtime source files remain inaccessible.

## Entrance viewport and animation

The signed-out desktop entrance fits the available viewport, including navigation,
feature summaries and footer, at 1280×720 and larger desktop sizes at 100% zoom.
Content is allowed to scroll at higher text zoom or unusually short windows so
controls never disappear. There is no forced overflow clipping.

Paper Shaders is bundled locally and capped at 120,000 physical pixels (previously
298,000). Moving branch nodes and eight pointer-responsive particles also have a lightweight SVG/CSS fallback when WebGL
is unavailable. Pause/play is available in either mode. Animation stops outside
the viewport or in a hidden tab, respects reduced motion (static SVG, no canvas
allocation), and preserves its state across browser back/forward restoration.
Only the signed-out entrance loads the shader module. The global search hint is
shown as a non-wrapping `Ctrl+K`; Control+K and Command+K both remain supported.

## Administrator resource meters

The admin home and storage page show used disk bytes / measured filesystem capacity
with a percentage and a native accessible progress bar. Warning and protection
states also retain text labels; unavailable measurements never show a fake zero
percent. Git/LFS inventory remains a separate figure, marked as a lower bound if
the scan is incomplete; disk usage includes other applications on the same volume.

System status shows two Go runtime meters: allocated heap / reserved heap address
space, and retained runtime memory (`Sys - HeapReleased`) / total obtained (`Sys`).
Neither denominator is physical RAM or an enforced memory limit. These values are
not process RSS. The cumulative allocation counter includes objects already freed
and is labelled separately. Read-only samples are shared for one second under a
lock; admin refresh requests pause while the tab is hidden and resume at the next
five-second tick. No forced GC, memory limit or cache eviction is introduced.

Disjoint storage inventory roots no longer retain a full-path deduplication map
for every file. Overlapping roots still deduplicate exact paths with first-root
category precedence, and scan bounds, errors, symlink handling and cache TTL stay
unchanged. Repository/LFS bytes, paths, refs and storage APIs do not change; rollback
uses the previous binary without a data migration.

Locale translations now use indexed strings plus presence bits rather than one
integer-key map per language. Language discovery, custom JSON overrides, nested
keys, explicit empty values, fallback languages and formatting remain compatible.
All languages stay available; this does not unload languages or affect accounts.
Local all-locale benchmarks measure the retained dictionary and initialization
allocations separately from process RSS. No runtime memory-limit tuning is applied.

## Repository creation and owner identity

The create-repository form displays the stable account name, an explicit
**Organization / Personal** badge immediately after it, and a distinct display
name when configured. The same identity appears in both the owner menu and the
selected value. Search matches names and display names through the existing
keyboard-accessible dropdown. Repository ownership still submits the original
account ID; labels and remarks do not rename accounts, URLs or clone paths and
do not grant permissions. Existing organization creation checks and limits apply.

Repository creation uses a dedicated desktop workspace with a main editor and a
sticky creation preview. Owner/name, description, visibility, default branch and
empty/initialized/template results update locally as fields change, without API
requests or draft storage. Native radio controls choose an empty repository for
pushing an existing project, or a first commit with optional initial files. Template
reuse and advanced controls are progressively disclosed; all settings stay on the
same page. The private checkbox and server-owned form values remain authoritative.
Changing modes retains entered values, and template selection takes precedence.
The first template selection preselects Git content; users can adjust it. A template
preview states that its default branch is inherited rather than guessing a branch.
The original auto_init, uid and other submitted fields remain compatible.
Chinese labels distinguish repository ownership, source templates,
issue-label presets, README templates and offering a repository as a template.
Visibility help follows the private checkbox: private access requires explicit
authorization, while public repositories still follow site-wide sign-in policy.
Initialization help explains the first commit versus an empty repository and
automatic checking when a .gitignore or license is selected. These are presentation
changes; API/form field names, default visibility, storage, Git format and
permissions remain unchanged. Roll back by restoring the previous binary and
matching embedded assets; no data migration is needed.

## Migration source selection and language legend

The import landing page offers a full-width Git URL option and compact platform
links, each with its own description. Local search matches names and descriptions,
reports the visible count and provides an empty-state reset; Escape clears search.
Original service_type, org and mirror parameters and every provider link remain
server rendered. Keyboard navigation, opening links in a new tab, disabled
JavaScript and the configured source permissions retain their existing behavior.

A brief 200 ms opacity/translation entrance and pointer-following highlight provide
feedback without loading React, React Bits or any additional animation dependency.
The visual direction references [React Bits Spotlight Card](https://reactbits.dev/components/spotlight-card); the implementation is independent.
One requestAnimationFrame batches pointer updates for the active card. There is no
continuous render loop, canvas, blur or remote asset request. Background tabs clear
the highlight; reduced motion skips entrance effects and disables pointer movement
and hover translation, including preference changes while the page is open.

Repository sidebar languages use one semantic list row per language: color, name
and right-aligned percentage. Names ellipsize while full text remains accessible;
long names have a title. The combined distribution bar, language order, percentages
and backend language statistics are unchanged. These presentation changes require
no Git, LFS, database or storage migration; rollback restores the previous binary
and its embedded assets.
