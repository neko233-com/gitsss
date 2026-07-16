#!/usr/bin/env bash

set -euo pipefail

port=3000
root_url=
job_instance_url=
image=gitsss/gitsss:latest
image_archive=
data_volume=gitsss-data
install_dir="${PWD}/gitsss-install"
no_start=false

usage() {
  echo "Usage: install.sh [--port PORT] [--root-url URL] [--job-instance-url URL]"
  echo "                  [--image IMAGE] [--image-archive FILE] [--data-volume NAME]"
  echo "                  [--install-dir DIR] [--no-start]"
}

while (($#)); do
  case "$1" in
    --port)
      port=$2
      shift 2
      ;;
    --root-url)
      root_url=$2
      shift 2
      ;;
    --job-instance-url)
      job_instance_url=$2
      shift 2
      ;;
    --image)
      image=$2
      shift 2
      ;;
    --image-archive)
      image_archive=$2
      shift 2
      ;;
    --data-volume)
      data_volume=$2
      shift 2
      ;;
    --install-dir)
      install_dir=$2
      shift 2
      ;;
    --no-start)
      no_start=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! [[ "$port" =~ ^[0-9]+$ ]] || ((port < 1 || port > 65535)); then
  echo "Invalid port: $port" >&2
  exit 2
fi

for value in "$root_url" "$job_instance_url" "$image" "$data_volume"; do
  if [[ "$value" == *$'\n'* || "$value" == *$'\r'* ]]; then
    echo "Configuration values cannot contain newlines" >&2
    exit 2
  fi
done

command -v docker >/dev/null 2>&1 || {
  echo "docker is required" >&2
  exit 1
}
docker compose version >/dev/null 2>&1 || {
  echo "docker compose is required" >&2
  exit 1
}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source_compose="${script_dir}/../compose.yaml"
[[ -f "$source_compose" ]] || {
  echo "compose.yaml not found beside installer" >&2
  exit 1
}

if [[ -n "$image_archive" ]]; then
  [[ -f "$image_archive" ]] || {
    echo "Image archive not found: $image_archive" >&2
    exit 1
  }
  docker load --input "$image_archive"
fi

docker image inspect "$image" >/dev/null 2>&1 || {
  echo "Image $image is unavailable. Load offline archive with --image-archive." >&2
  exit 1
}

[[ -n "$root_url" ]] || root_url="http://localhost:${port}/"
[[ "$root_url" == */ ]] || root_url="${root_url}/"
[[ -n "$job_instance_url" ]] || job_instance_url="http://host.docker.internal:${port}"

mkdir -p "$install_dir"
cp "$source_compose" "${install_dir}/compose.yaml"
env_tmp=$(mktemp "${install_dir}/.env.XXXXXX")
trap 'rm -f "$env_tmp"' EXIT
cat >"$env_tmp" <<EOF
GITSSS_IMAGE=$image
GITSSS_ACTIONS_JOB_IMAGE=$image
GITSSS_HTTP_PORT=$port
GITSSS_ROOT_URL=$root_url
GITSSS_ACTIONS_JOB_INSTANCE_URL=$job_instance_url
GITSSS_DATA_VOLUME=$data_volume
EOF
mv "$env_tmp" "${install_dir}/.env"
trap - EXIT

if [[ "$no_start" == false ]]; then
  docker compose --project-directory "$install_dir" up --detach --force-recreate --pull never
fi

echo "Installed: $install_dir"
echo "URL: $root_url"
echo "Switch port without data migration:"
echo "  $0 --install-dir '$install_dir' --data-volume '$data_volume' --port 3000 --root-url 'http://localhost:3000/'"
