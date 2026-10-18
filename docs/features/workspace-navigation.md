# Workspace navigation and date/time controls

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
