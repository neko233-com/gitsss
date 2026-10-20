# Desktop workspace refinement

The project-local Oil UI workflow keeps a compact, readable, traceable desktop
workspace. Changes stay in Go templates, TypeScript and CSS, preserving handlers,
permissions, form fields, Vue/React entrypoints, dependencies and build contracts.
Layout follows the current browser viewport and reacts to resizing. Forms stack
when space is limited; short windows disable sticky previews so submission stays
reachable through ordinary page scrolling. This uses CSS, without resize polling.
Below 1051 CSS pixels, the organization workspace navigation becomes an explicit
drawer with keyboard focus containment, Escape/backdrop close and focus return.
The closed drawer is inert; without JavaScript its links remain in normal flow.
Dashboard columns stack at narrow widths. Shared dialogs stay within available
width/height, and settings/software views preserve readable content and actions.
The bundled manual also constrains flex content and lets its header links scroll
within the header, rather than forcing the whole document beyond a narrow window.

## Creation and migration

The create/import link retains its initial owner. Owner identities show the
stable name, personal/organization type and separate display name. The server
remains authoritative for name validation, visibility, ownership and templates.

All nine migration providers share source/access, import options, destination
and a sticky confirmation panel. Provider authentication and available migration
items remain intact. Local previews never include tokens, passwords or source
URLs containing credentials. Advanced migration choices are disclosed on demand;
mirror, LFS and endpoint-error states expand the options initially. Editing the
source does not overwrite a manual or
server-returned repository name. Blank credentials do not enable protected items.
Mirror restrictions remain intact. LFS advanced settings stay expanded while
enabled, and validation errors reveal the endpoint. Changing source retains the
initial owner/mirror context. No import engine or API is replaced.

## Repository details

The header displays repository identity beside visibility and existing actions.
File columns can shrink around long commit messages; keyboard focus highlights
the same row as hover. Sidebar links wrap, languages remain vertically aligned,
and code/file search inputs have explicit accessible names. Ref selection,
clone URLs, copy/archive actions, topics and source browsing retain existing APIs.

## Sign-in and feedback

The desktop sign-in surface pairs concise workspace context with a readable
form and inline errors. Username/password fields, autocomplete, CAPTCHA, CSRF,
redirects, remembered-device behavior and backend authentication are preserved.
Only configured providers render. Binding continues to use the same inner form.
The keyboard-accessible password button changes input visibility without storing
credentials. Selected create, migration and sign-in forms become busy after
browser validation and prevent duplicate submissions. Back/forward restoration
resets this state; server errors render a new form. Other forms are unaffected.

Short native transitions follow theme tokens and reduced-motion preferences.
No runtime dependency, polling, CDN or continuous decoration loop is added.
An empty built-in frontend-plugin manifest avoids a missing-file console error;
the existing custom manifest still takes precedence. It is included in frontend
source fingerprints and does not enable any plugin.

## Verification and rollback

Check shared controls and actual page families, prioritizing creation, all nine
migration providers, repository details and sign-in. Record exact routes/states;
distinguish local OAuth tests from real provider authorization and production
read-only checks from isolated writes. Do not claim unconfigured flows passed.

There are no account, OAuth binding, RBAC, Git, SSH, LFS, schema, configuration or
storage migrations. Deploy a binary with matching embedded assets only after
local/browser checks. Protect the database backup and compare identity/RBAC/SSH
before/after. Rollback restores the previous binary; Git/LFS need no migration.

See [migration](../site/migration.html#source-selection),
[repository workspace](../site/enterprise.html#repository-workspace) and
[sign-in](../site/identity-channels.html#signin-workspace).
