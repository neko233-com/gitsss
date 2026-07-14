# LFS and game assets v1

gitsss ships a native Git LFS Basic transfer service at `/git/{repository}.git/info/lfs`. A real Git LFS 3.7 client test pushes a tracked PSD through Smart HTTP, stores it by SHA-256 OID, clones the repository and verifies the restored asset byte-for-byte.

## Storage and integrity

- Objects live below `GITSSS_LFS_ROOT` (default `<data-directory>/lfs`) as `aa/bb/{sha256}` and are deduplicated across repositories without changing the LFS OID.
- Uploads are streamed to a protected same-volume partial area, hashed during write, synced, linked into place without replacement and never exposed before their OID matches.
- Existing objects are reused only when the expected size matches. Conflicting or non-regular filesystem entries fail closed.
- LFS storage is included automatically in storage-awareness inventory. Batch upload checks live disk admission before returning an upload action; write protection returns an object-level 507 result.
- gitsss does not recompress PSD, FBX, audio or video. Already compressed MP3/MP4 data remains exact, and source SHA/OID compatibility is preserved.

## Transfer

The standard Basic flow supports batch upload/download, authenticated `PUT`, `GET`, `HEAD`, ETag and HTTP byte ranges. A custom client may resume a sequential upload with `Content-Range: bytes start-end/total`; gitsss records the protected partial size in `X-Gitsss-Upload-Offset`, accepts incomplete chunks with 202, and publishes only after the final whole-file hash passes.

Batch calls, upload authorization/results and downloads use the same `repositories:push` or `repositories:clone` Token scopes as Git and write audit records. Bearer parsing is exact; Git credential helpers may use Basic auth with the Token as password. Tokens in URLs are unsupported.

Set `GITSSS_PUBLIC_URL` when LFS action URLs must use a reverse-proxy origin or base path. The configured value must be HTTP(S) without credentials, query or fragment.

## Clone performance

New hosted repositories enable Git protocol v2 partial-clone filters, reachable object wants, commit-graph writing and bitmap generation. The Smart HTTP integration test clones with `--filter=blob:none` and verifies promisor configuration. LFS assets then transfer independently with range support, avoiding large binary blobs in the initial Git pack.
