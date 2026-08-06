#!/usr/bin/env bash
# Read-only handoff validation for an internal Gitsss-compatible server.
# Set GITSSS_REMOTE_PASSWORD; the secret is never written to the console or URL.
set -euo pipefail

BASE_URL="${GITSSS_REMOTE_URL:-http://git.example.internal:3000}"
REPO="${GITSSS_REMOTE_REPO:-engineering/server-project-sf-go}"
USERNAME="${GITSSS_REMOTE_USER:-automation-user}"
PASSWORD="${GITSSS_REMOTE_PASSWORD:-}"
NO_CLONE="${GITSSS_REMOTE_NO_CLONE:-0}"
BASE_URL="${BASE_URL%/}"
REPO="${REPO#/}"
REPO="${REPO%/}"

if [[ -z "$PASSWORD" ]]; then
  read -r -s -p "Remote password: " PASSWORD
  printf '\n' >&2
fi

auth="$(printf '%s' "${USERNAME}:${PASSWORD}" | base64 | tr -d '\r\n')"
probe() {
  local path="$1"
  curl --fail --silent --show-error -u "${USERNAME}:${PASSWORD}" "${BASE_URL}${path}" >/dev/null
  echo "[remote] 200 ${path}"
}

probe /api/healthz
probe /api/swagger
probe "/${REPO}"

remote="${BASE_URL}/${REPO}.git"
GIT_TERMINAL_PROMPT=0 git -c "http.extraheader=Authorization: Basic ${auth}" ls-remote "$remote" refs/heads/main refs/heads/master
echo "[remote] smart HTTP refs passed"

if [[ "$NO_CLONE" != "1" ]]; then
  temp_root="$(mktemp -d "${TMPDIR:-/tmp}/gitsss-remote-verify.XXXXXX")"
  trap 'rm -rf -- "$temp_root"' EXIT
  GIT_TERMINAL_PROMPT=0 git -c "http.extraheader=Authorization: Basic ${auth}" clone --depth 1 --filter=blob:none "$remote" "$temp_root/repo"
  mapfile -t workflow_files < <(find "$temp_root/repo" -type f \( -path '*/.gitea/workflows/*.yml' -o -path '*/.gitea/workflows/*.yaml' -o -path '*/.github/workflows/*.yml' -o -path '*/.github/workflows/*.yaml' \))
  echo "[remote] workflow files: ${#workflow_files[@]}"
  [[ ${#workflow_files[@]} -gt 0 ]] || { echo "no workflow files found" >&2; exit 1; }
  command -v git-lfs >/dev/null 2>&1 && git lfs version || true
fi

echo "[remote] read-only validation passed"
