# Gitea 1.27 compatibility upgrade

The running source is Gitea 1.27.1 with database version 343. GitSSS's previous
platform copy used Gitea 1.26.2 and assigned migration 331 to project fields.
Upstream assigned 331 to Actions run attempts. These histories cannot be joined
by changing a database version number.

## Scope and acceptance

Integrate the Gitea 1.27.1 platform implementation, including its migrations,
Actions run attempts/reusable workflows, team visibility, branch protection and
runner protocol. Retain GitSSS's enterprise defaults, HTTP importer, embedded
runner, agent APIs, project fields and existing internal module paths. Project
fields must use a GitSSS-specific extension migration instead of an upstream ID.
No repository GitHub Actions workflows are installed or executed.

Validate fresh installation, the previous GitSSS schema, and an independent
snapshot of the production schema. Compare original credential, membership,
release, package, LFS and Actions records without writing their values to logs.
Run Git/LFS and isolated Actions acceptance against the new binary before use.

## Storage, capacity and performance

Git, LFS, database, package, Actions log/artifact paths and provider configuration
remain stable. Rehearsal uses a SQLite backup and four small pilot Git mirrors;
it does not duplicate the source's large repository collection. Final takeover
uses the original data paths after the source service stops. Upstream migration
331 reorganizes Actions metadata into run attempts; allocate room for a database
backup and temporary schema indexes. Measure migration time on the snapshot.
Never allow two servers to write one database or mutable repository tree.

Windows local migration and restore paths are encoded as proper `file:` URLs
before opening assets and patches. Absolute-path and local-import permission
checks still apply. This changes URL encoding only; it does not move storage.

## Rehearsal evidence (2026-10-16)

An independent SQLite snapshot retained all 116 original tables, columns and row
IDs at upstream version 343. The migration took about 1.35 seconds. Runtime checks
accepted the original administrator token, organization/team roles and visibility,
four pilot repositories, existing runner IDs and Actions run metadata. Live
production runners and original SSH-key use remain cutover-window checks.

Four HTTP pilot repositories matched 135 source refs. Separate disposable tests
covered HTTP clone/fetch/push, branch/tag deletion, pull-request merging, LFS,
SSH, read-only permissions and editable team labels. An isolated native runner
completed checkout, shell execution and artifact upload. A delta test added Git
and LFS data, pruned a source branch, rejected independent destination changes,
and confirmed an unchanged rerun did not increase its 274127-byte mirror cache.
These results cover the pilot and snapshot; they do not authorize cutover.

## Deployment and rollback

During evaluation Gitea stays on 3000 and GitSSS on 5030. Production runners,
scheduled tasks and imported webhooks remain disabled on GitSSS. Cutover requires
the operator's later instruction. At that window stop both writers, take a fresh
consistent database backup, preserve the original configuration/secrets and Git,
LFS, package and Actions storage, then start the accepted GitSSS binary. Add the
3000 listener only after Gitea releases it.

For rollback stop GitSSS, restore the pre-cutover database and original
configuration, and restart the original Gitea 1.27.1 binary on its original data
paths. Keep post-cutover Git changes separately if rollback follows new writes.
The old GitSSS 1.26-based binary cannot read database version 343; retain its own
staging database backup when upgrading that instance. Do not lower version
numbers manually.

## Additive organization tasks and local avatars

Extension schema version 2 adds `org_task` only. Existing upstream 343 records,
Git/LFS objects, repository boards and owner project routes stay intact. New
cards reference the organization, creator and one assignee; every operation is
scoped to the authenticated organization and assignment requires membership.
Only the creator or organization owner can delete a card. Rendering is bounded
to 500 matching cards; search and an explicit limit notice handle larger boards.
No additional Git repositories or object copies are created by task recording.

Rollback to extension version 1 requires its pre-upgrade database backup. A final
native takeover must preserve trial-only `org_task` rows and any legacy trial
projects, mapping organization and user IDs rather than copying numeric IDs
blindly. Never replace the trial database before exporting and verifying them.

The 24 embedded DiceBear avatars add about 250 KiB of PNG data to the binary.
Each is 256px and has recorded attribution, source, license and checksum. Selection uses existing avatar storage and
processing, preserving provider keys and paths. New uploads always crop to square
and downscale to configured rendered size; PNG/JPEG compression keeps the smaller
suitable representation, preserves transparency and does not upscale. GIF uploads
use the first frame. Existing avatar bytes are unchanged.
