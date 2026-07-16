# Gitea 1.26.1 takeover verification

Date: 2026-07-16

## Scope

- Source: official `gitea/gitea:1.26.1` using its standard `/data` volume layout.
- Target: the single `gitsss/gitsss:dev` Ubuntu 24.04 image.
- Upgrade mode: stop the source container and mount the same `/data` volume into gitsss without setting a compatibility data path.

## Fixture

- Accounts: `migration-admin`, `migration-user`.
- Repositories: `migration-admin/migration-repo-one`, `migration-user/migration-repo-two`.
- Each repository contained an initialized `main` branch and a second commit adding `fixture.txt`.

## Evidence

- gitsss detected `/data/gitea/conf/app.ini` and selected `/data/gitea` as its work and custom path.
- The database initialized successfully and repository hooks changed their application path from `/usr/local/bin/gitea` to `/usr/local/bin/gitsss`.
- Both original account passwords authenticated through the gitsss API.
- Both repositories, branches, two-commit histories, and file contents were readable through the gitsss API.
- Browser verification showed the migrated repository in the native gitsss UI with its code, issues, pull requests, workflows, packages, projects, releases, wiki, activity, and settings navigation.
- The built-in offline Actions runner registered after takeover.

## Cleanup

The source and target test containers, fixture volume, and temporary official Gitea image are removed after the verification. The only retained product image tag is `gitsss/gitsss:dev`.

## Status and production note

This remains evidence for the optional stopped-volume fallback. It is not the
primary migration mode and was not run against the live
`git.example.internal:3000` instance. HTTP-only account/RBAC/two-repository evidence
for the current baseline is recorded in `gitea-http-import-1.26.2.md`.
