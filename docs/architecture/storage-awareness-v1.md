# Storage awareness v1

gitsss measures the filesystem that contains its state instead of reporting a configured demo capacity. `GET /api/v1/storage` returns the volume capacity, free space, reserve thresholds, measurement time, bounded inventory, and large game-asset attribution. `POST /api/v1/storage/admission` uses the same live snapshot before accepting a projected write.

## Protection model

- The default operator reserve is 10 GiB and can be changed with `GITSSS_STORAGE_RESERVE_BYTES`.
- Warning begins at the larger of twice the reserve or 15% free space.
- Write protection begins at the larger of the reserve or 5% free space.
- A capacity measurement failure closes admission instead of allowing an unmeasured write.
- Inventory failures raise a warning but do not replace the filesystem's authoritative capacity values.
- gitsss never deletes user data automatically in response to low capacity.

Set `GITSSS_STORAGE_VOLUME_PATH` when the protected data volume differs from the directory containing `GITSSS_DATA_PATH`. Capacity snapshots are cached for 30 seconds by default. `GITSSS_STORAGE_CACHE_TTL` accepts a Go duration up to 10 minutes.

## Inventory roots

The embedded database and configured repository are inventoried automatically. Additional roots use the platform path-list separator:

| Environment variable | Category |
| --- | --- |
| `GITSSS_LFS_PATHS` | LFS assets |
| `GITSSS_ARTIFACT_PATHS` | pipeline artifacts |
| `GITSSS_BACKUP_PATHS` | backups |
| `GITSSS_ATTACHMENT_PATHS` | attachments |

The scanner does not follow symbolic links, deduplicates overlapping roots, and stops after `GITSSS_STORAGE_MAX_ENTRIES` files (100,000 by default). A truncated scan is explicit in the API response. Physical volume use that is not attributed to a configured root appears as `other_volume_usage`.

## Game assets

The inventory reports file counts and bytes for recognized game-asset extensions. PSD, PSB, FBX, Blender, WAV, and FLAC files are marked `chunked_lfs`; already compressed MP3, OGG, MP4, MOV, MKV, and WebM files are marked `lfs_no_recompress`. These are transport recommendations only: gitsss does not mutate Git objects, LFS OIDs, or source assets.

The built-in content-addressed LFS root is always registered as `lfs_assets`; operators do not need to repeat it in `GITSSS_LFS_PATHS`. See [`lfs-game-assets-v1.md`](lfs-game-assets-v1.md).
