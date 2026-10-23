# Organization intranet share

Organization members share files and short notes without putting them in Git or
LFS. The tab is on the organization page, after Worktime. Bytes live in
`APP_DATA_PATH/org-share` unless `[org_share] PATH` points somewhere else.

## Capacity and disk

Each organization starts at 100 GB (`MAX_BYTES`, 100 GiB). A site admin changes
one organization from `/-/admin/org-share`, `gitsss admin org-share quota`, or
`PUT /api/v1/admin/org-share/quota`. `max_bytes` of 0 restores the default.

Before an upload is accepted, gitsss checks both:

- organization bytes already stored, plus in-progress uploads, plus this file
- free space on the share volume, minus `RESERVE_BYTES` (default 1 GiB)

If either check fails, the request stops and the response explains the quota or
the disk numbers. No partial object is kept for that refusal. The admin page
shows the volume size, free space, reserve, and every organization's usage.

A single file defaults to a 32 GiB ceiling (`MAX_FILE_BYTES`).

## Lifetime

New files and notes expire after 30 days (`EXPIRE_DAYS`). `EXPIRE_DAYS = 0`
disables the default expiry. A member can mark one object permanent in the web
UI, with `permanent: true` on the API, or with `--permanent` on the CLI.
Permanent objects stay until someone deletes them. Expired objects are removed
when the folder is listed and by `gitsss admin org-share cleanup`.

## Transfer

Uploads are chunked. Clients may send several `Content-Range` chunks at once
and retry a chunk that already landed. The default chunk is 8 MiB and at most
8 chunks run together. Downloads honor a single HTTP Range per request, so a
client can fetch ranges in parallel.

Members can read and write. The uploader or an organization owner deletes one
object. Only an owner clears the whole share. Other users receive 404 or 403.

## CLI

On the server:

```sh
gitsss admin org-share usage
gitsss admin org-share quota --org acme --bytes 53687091200
gitsss admin org-share put --org acme --uploader 1 --file ./go.tar.gz --permanent --parallel 4
gitsss admin org-share get --org acme --id 7 --output ./go.tar.gz --parallel 4
gitsss admin org-share list --org acme
```

From another machine, with a token file:

```sh
gitsss admin org-share push --url http://127.0.0.1:5300/api/v1/orgs/acme/shares --token-file ./token --file ./go.tar.gz --parallel 4
gitsss admin org-share pull --url http://127.0.0.1:5300/api/v1/orgs/acme/shares/files/7 --token-file ./token --output ./go.tar.gz --parallel 4
```

## HTTP API

All routes require an organization member. `{org}` is the organization name.

- `GET /api/v1/orgs/{org}/shares?parent=`
- `POST /api/v1/orgs/{org}/shares/text`
- `POST /api/v1/orgs/{org}/shares/uploads` with `name`, `size`, `permanent`, `resume`
- `GET /api/v1/orgs/{org}/shares/uploads/{id}` returns missing ranges
- `PATCH /api/v1/orgs/{org}/shares/uploads/{id}` with `Content-Range: bytes START-END/TOTAL`
- `POST /api/v1/orgs/{org}/shares/uploads/{id}/complete`
- `GET /api/v1/orgs/{org}/shares/files/{id}` supports `Range`
- `PATCH /api/v1/orgs/{org}/shares/files/{id}` with `{"permanent": true}`
- `DELETE /api/v1/orgs/{org}/shares/files/{id}`
- `DELETE /api/v1/orgs/{org}/shares` (owner)
- `GET /api/v1/admin/org-share`
- `PUT /api/v1/admin/org-share/quota` with `org` or `org_id`, and `max_bytes`

Quota failures use HTTP 413. Disk failures use HTTP 507. Both bodies include
`code` plus the byte counters.
