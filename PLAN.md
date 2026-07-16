# gitsss production plan

## Product boundary

gitsss is a self-maintained intranet Git server based on the copied Gitea
v1.26.2 source baseline. It does not link Gitea product libraries at runtime.
The only public command and product name is `gitsss`.

The Gitea page layout, account model, repository model, Git protocols, issues,
pull requests, packages, releases, projects and administration behavior remain
the functional baseline. The retained gitsss-specific concepts are limited to:

1. Git repository storage foundations;
2. storage awareness and write admission protection;
3. the operational data dashboard.

The old standalone Go service and React application were removed after these
concepts were integrated into the platform baseline. There is one server, one
UI layout and one product data model.

## Required production outcomes

- Default HTTP port 3000 and new-install data root `/data/gitsss`.
- Import accounts, organization/team/direct RBAC, selected Git refs, and LFS
  from a running Gitea by HTTP without source SSH, filesystem, or volume access.
- Retain stopped-volume takeover as an optional full-fidelity fallback for
  password hashes, TOTP, SSH keys, issues, and other database-only state.
- Preserve the gitsss skin on the Gitea-compatible page layout.
- Ship one Ubuntu 24.04 image at `gitsss/gitsss`; no separate public runner
  image or command.
- Embed the Actions runner, Node.js, Go and common action runtimes. Workflows
  must fail closed instead of downloading an unknown action from the internet.
- Support Feishu and other intranet notifications with administrator-defined,
  safely parsed templates and auditable delivery behavior.
- Keep useful GitLab collaboration behavior where it improves the Gitea
  baseline, including approval rules, protected branches, scoped labels,
  CODEOWNERS, merge-request template conventions, artifacts and merge-after-
  checks behavior.
- Validate linux/amd64 and linux/arm64 containers, then package macOS arm64 for
  Mac mini M4 deployment. Windows and macOS/amd64 builds remain release gates.
- Use local formatting, linting, tests, packaging and checksum verification.
  GitHub workflow files are forbidden.

## Migration acceptance

Primary HTTP migration keeps source Gitea online and untouched during rehearsal.
Final apply uses a short selected-repository write freeze and a separate target.
Acceptance verifies:

- imported account active/admin/restricted/login state and forced temporary
  password change;
- organization ownership, custom teams, unit permissions, public membership,
  team repository grants, and direct collaborator permissions;
- selected repository default branches, refs, Git object IDs, and LFS content;
- credentials output permissions, health, clone, push, rollback data, and final
  generated URLs on port 3000.

HTTP cannot preserve password hashes, TOTP, tokens, SSH keys, issues, releases,
or other database-only state. If those are required, use the stopped immutable
volume fallback and verify original credentials plus full database content.
Synthetic evidence lives under `docs/verification/`. The live
`git.example.internal:3000` service must not be accessed or changed by automated tests.

## Release policy

No feature releases are published during the rebuild. One consolidated release
is created only after every required local gate, migration rehearsal,
multi-platform package, checksum and configured signature has passed.
