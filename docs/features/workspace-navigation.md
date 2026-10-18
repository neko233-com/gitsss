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

## Entrance and personal context

The signed-out entrance uses one locally bundled Paper Shaders canvas, capped below 300,000 pixels. Reduced-motion preference starts it paused; users can pause/resume it. The library suspends rendering offscreen/in the background, and WebGL failure falls back to a static SVG. No shader library is requested on signed-in pages. Organization-first navigation also includes the current account as `username (Personal)` alongside the organization list; the existing personal URL remains `/?view=personal`. Code, diff, blame and editor comments, including documentation comments, share neutral gray tokens in each theme.

The embedded offline manual also includes its linked feature contracts and acceptance reports. These text-only directories are bundled with the binary; runtime source files remain inaccessible.
