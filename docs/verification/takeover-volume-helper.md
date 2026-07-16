# Takeover volume helper verification

Date: 2026-07-16

The production helper was verified with two temporary Docker volumes and the
single local `gitsss/gitsss:dev` image.

- A running container mounted the source volume; the helper refused to copy it
  and did not create the target volume.
- After the source container stopped, the helper copied the source read-only
  into a new candidate volume with networking disabled.
- The aggregate SHA-256 over content, type, mode, UID/GID, size, mtime, and
  symbolic-link targets matched for source and target.
- Installed `app.ini` and a standard repository root were detected.
- The candidate received source and role labels.
- The test container and both temporary volumes were removed.

The helper never starts gitsss or mutates the source volume. Acceptance and
cutover remain explicit operator steps.
