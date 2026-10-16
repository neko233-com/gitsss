# LFS garbage collection

Unreferenced Git LFS objects are metadata rows whose pointer blob is no longer
in the repository. `gitsss admin lfs-gc` scans them and can delete the stored
bytes. It does not move repositories, rewrite Git objects, or change the LFS
oid path layout.

Dry-run is the default. The command prints JSON on stdout.

```sh
gitsss admin lfs-gc
gitsss admin lfs-gc --repo engineering/client
gitsss admin lfs-gc --apply --older-than 168h
```

`--older-than` defaults to 168 hours. A window shorter than 24 hours is refused
unless `--force` is set. An LFS upload can reach storage before the git pointer
is pushed, so a shorter window can delete an object that is still in flight.

The same scan is on the admin page `/-/admin/lfs-gc`. Preview does not delete.
Apply requires the confirmation checkbox. Agents with a site-admin token can
`POST /api/v1/admin/lfs-gc`:

```json
{"apply": false, "older_than_hours": 168, "repository": "engineering/client"}
```

Omit `apply` or set it to false for a dry-run. `force` allows a retention
window under 24 hours. The response is the same JSON report as the CLI.

The server git config sets `lfs.concurrenttransfers` to the CPU count, capped
at 8, so large game repositories do not open one transfer per core. That
setting does not change where objects are stored.
