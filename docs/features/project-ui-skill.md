# Project-local Oil UI skill

GitSSS vendors the free `skills/oil-ui` skill from
[oil-oil/dsh-oil-ui](https://github.com/oil-oil/dsh-oil-ui/tree/b67ae2df6c2c9d65e177aaa1ce703e65457b24d7/skills/oil-ui)
at commit `b67ae2df6c2c9d65e177aaa1ce703e65457b24d7` (Oil UI `0.23.8`).
The requested repository is a DeepSeek Harness plugin; this project uses its
bundled skill with a Codex adapter, without installing the plugin runtime.

## Discovery and scope

The project entrypoint is `.agents/skills/oil-ui/SKILL.md`. Codex can discover it
on the next turn; `$oil-ui` invokes it explicitly. The root `AGENTS.md` also
routes redesign, visual refinement and UX review to this entrypoint. No global
skill installation or user-profile configuration is required.

The directory is tracked with narrow `.gitignore` exceptions; unrelated local
agent settings stay ignored. Screenshots, prototypes and review evidence go in
ignored `output/ui-review/<task>/` or a private temporary directory. Do not put
internal repository data, credentials or private screenshots in public commits.

## Architecture adaptation

The skill keeps Go HTML templates, Go routes, TypeScript, CSS, Fomantic, Vue,
existing React islands and both Vite entrypoints. Changes follow the page's
current implementation; importing a design method does not migrate frameworks,
add another frontend application or change language/build dependencies.

The adapter prioritizes the established compact desktop workspace, semantic
theme tokens, light/dark readability, precise Chinese, keyboard navigation,
reduced motion and bounded animation work. It preserves form fields, URLs,
permissions, CSRF and progressive enhancement. Git/SSH/LFS, accounts, database,
storage and API contracts remain under the root compatibility rules.

References are loaded for the affected problem, rather than all at once.
Broad direction exploration is reserved for a task that needs it; established
product directions and small changes do not force extra design-choice rounds.
Installation alone changes no product UI and requires no server restart.

## Tools and provenance

`references/codex-tools.md` maps DSH-only names to available Codex tools and
the bundled CLI. Offline comparisons use `scripts/build_explorer.py`; local
screenshots can use `scripts/shoot.mjs`, with an explicit desktop `--size`.
Actual authenticated application flows retain the project's browser workflow.
These developer tools are not frontend runtime dependencies or release assets.

The original entrypoint is preserved as `UPSTREAM.md`. `SOURCE.json` records
upstream version/commit and per-file SHA-256 values, plus adapter files.
The MIT license, font OFL notice and upstream resources remain with the skill.
The font notice only has trailing whitespace removed; its entry records both
upstream and installed hashes without changing license wording.
No DSH service registration, Pro dependency, automatic version check or
promotional startup script is enabled.

## Updating and removing

For an explicitly requested update, download `skills/oil-ui` at a selected
commit to a temporary directory using skill-installer. Review changes, retain
the project entrypoint and adapter references, refresh provenance/checksums,
then validate discovery, references and changed CLI behavior. Do not overwrite
local edits or copy the plugin's workflows, package manifest or runtime.

Removal deletes only `.agents/skills/oil-ui/`, its root AGENTS routing line and
its narrow ignore exceptions/documentation. Keep unrelated `.agents` content.
Neither install nor removal changes deployed GitSSS, its data or system Git.

See the [Agent collaboration manual](../site/agents.html#project-ui-skill).
