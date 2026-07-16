# Mac mini M4 deployment

Mac mini M4 runs the verified `linux/arm64` Ubuntu 24.04 image. The server,
embedded Actions runner, common Actions, Node.js, Go, Git, SQLite, SSH, and web
UI stay in one image. Persistent state stays in one Docker volume.

## Offline image transfer

Build and export the arm64 image on the release machine, then copy the archive
and its SHA-256 file through the company-approved channel:

```sh
docker buildx build --platform linux/arm64 --load -t gitsss/gitsss:VERSION .
docker save gitsss/gitsss:VERSION -o gitsss-VERSION-linux-arm64.oci.tar
shasum -a 256 gitsss-VERSION-linux-arm64.oci.tar > gitsss-VERSION-linux-arm64.oci.tar.sha256
```

On the Mac mini, verify and load it without contacting an external registry:

```sh
shasum -a 256 -c gitsss-VERSION-linux-arm64.oci.tar.sha256
docker load -i gitsss-VERSION-linux-arm64.oci.tar
```

## Start

Set the LAN URL and immutable image tag, then use the production Compose file:

```sh
export GITSSS_IMAGE=gitsss/gitsss:VERSION
export GITSSS_ROOT_URL=http://192.168.110.42:3000/
docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:3000/api/healthz
```

HTTP uses port 3000. SSH is published as 2222 by default to avoid colliding
with macOS Remote Login. Set `GITSSS_SSH_PORT` before first start if another
external SSH port is required.

## Gitea HTTP migration

Use `gitsss import-gitea-http` as the primary migration path. It reads source
Gitea through HTTP only and leaves the live source host, process, filesystem,
and Docker volume untouched. Run gitsss temporarily on port `3001`, migrate and
verify the selected accounts/RBAC/repositories, then use the approved cutover
window to stop Gitea and publish gitsss on port `3000`.

See [Gitea HTTP account and repository import](gitea-http-import.md). HTTP import
creates temporary account passwords because Gitea does not expose password
hashes, TOTP secrets, tokens, or SSH keys through its API.

## Stopped-volume fallback

Use volume takeover only when full database fidelity is required. Do not copy a
live database. Stop Gitea, retain an immutable snapshot and its rollback copy,
then set `GITSSS_DATA_VOLUME` to the Docker volume containing the standard
Gitea `/data` tree. gitsss detects `/data/gitea/conf/app.ini`, opens the existing
database and repositories, migrates the schema, and rewrites hooks to the
`gitsss` executable.

Prepare a separate candidate volume with the repository helper. It refuses a
source volume mounted by any running container, uses only the local gitsss
image, disables networking, mounts the source read-only, preserves numeric
ownership, and compares an aggregate SHA-256 over content plus filesystem
metadata:

```sh
sh contrib/prepare-gitea-takeover.sh \
  --source-volume gitea-data \
  --target-volume gitsss-takeover-candidate \
  --image gitsss/gitsss:VERSION

export GITSSS_DATA_VOLUME=gitsss-takeover-candidate
export GITSSS_IMAGE=gitsss/gitsss:VERSION
docker compose up -d
```

Never point the first gitsss start at the source volume. If acceptance fails,
stop gitsss, remove only the candidate volume, and restart the unchanged Gitea
source volume.

Production acceptance must follow the selected migration mode. HTTP import
verifies temporary-login enforcement, account state, organization/team/direct
RBAC, refs, object IDs, LFS, clone, and push. Stopped-volume takeover additionally
verifies original credentials, TOTP, SSH keys, tokens, and database content.
