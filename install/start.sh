#!/usr/bin/env bash

set -euo pipefail

install_dir=${1:?install directory is required}
docker_bin=${2:-docker}
log_dir="${install_dir}/logs"
log_path="${log_dir}/autostart.log"

mkdir -p "$log_dir"

deadline=$((SECONDS + 120))
until "$docker_bin" version --format '{{.Server.Version}}' >>"$log_path" 2>&1; do
  if ((SECONDS >= deadline)); then
    printf '%s Docker did not become ready within two minutes.\n' "$(date -Is)" >>"$log_path"
    exit 1
  fi
  sleep 2
done

exec "$docker_bin" compose --project-directory "$install_dir" up --detach --remove-orphans --pull never >>"$log_path" 2>&1
