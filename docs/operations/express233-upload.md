# Native Express233 upload

GitSSS embeds `gitsss/express233-upload@v1`. It needs Node.js 24 on the
runner, with no npm dependencies or external action download. It uses the existing
Express233 HTTP API and does not change repository, LFS or artifact storage.

Configure `EXPRESS233_SERVER` and `EXPRESS233_PROJECT_ID` as repository or organization
Actions variables, and `EXPRESS233_TOKEN` as an Actions secret with write access
to the destination project. Keep private addresses and credentials out of Git.
Basic authentication is also supported with the `username` and `password` inputs;
provide either that pair or `token`, never both.

Place a workflow in the application's `.gitea/workflows/` directory. This GitSSS
source repository does not contain or run `.github/workflows/`.

```yaml
name: Upload build
on: [workflow_dispatch]
jobs:
  upload:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: ./build.sh
      - uses: gitsss/express233-upload@v1
        id: upload
        with:
          server: ${{ vars.EXPRESS233_SERVER }}
          project-id: ${{ vars.EXPRESS233_PROJECT_ID }}
          token: ${{ secrets.EXPRESS233_TOKEN }}
          version: 1.0.${{ github.run_number }}
          path: build/output
          publish: 'false'
```

## Behavior

- `path` accepts files or directories relative to `GITHUB_WORKSPACE`, one per
  line. No glob expansion. Parent paths, symlinks, `.git` and non-regular files are
  rejected before creating a version. Directories preserve their workspace-relative
  paths; at most 10,000 distinct input files are accepted.
- A new draft is created first. An existing version fails; it is never overwritten.
  Files stream through multipart requests without a second archive copy. Express233
  automatically extracts ZIP/TAR/TGZ input archives according to its existing API.
- `publish` defaults to `false`. Set `true` only when the workflow should publish
  the completed upload. Publishing may start configured Express233 deployment hooks
  and Pull Agent follow actions. Upload failure leaves the draft for inspection.
- Uploads retry network failures, HTTP 429 and 5xx at most twice by default.
  `retries` accepts 0–5. Draft creation and publication are not retried: after an
  ambiguous connection failure, inspect the receiver before rerunning.
- `timeout-seconds` is a per-request timeout, default 300, range 10–3600.
  `tags` accepts up to 32 file tags separated by newlines.
- HTTP/HTTPS base URLs are supported, including an installation path prefix.
  Redirects and URL credentials are refused, TLS verification stays enabled, and
  receiver error bodies are omitted from logs. Authentication values are masked.
  Prefer HTTPS where available; intranet HTTP sends credentials without TLS.

Outputs: `version`, `file-count` (input files before archive extraction), and
`manifest-path` (a JSON file in `RUNNER_TEMP` with input paths, sizes and SHA256,
plus the actual repository/ref/commit). The manifest records provenance locally;
it does not manufacture an HTTPS repository URL for an HTTP-only source and does
not alter Express233's VCS contract. A step summary records draft/published state.

## Validation and recovery

Run `node --test tools/tests/express233-upload.test.cjs` for streaming, path
validation, duplicate version refusal, retry, redirect and log-redaction checks.
Validate a real receiver and an isolated GitSSS runner with a disposable project
before enabling publication for a production project. No SSH settings or source
Gitea configuration are involved. Rollback is the preceding GitSSS binary; uploaded
Express233 drafts are managed by Express233's existing version UI/API.
