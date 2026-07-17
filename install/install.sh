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
no_autostart=false

usage() {
  echo "Usage: install.sh [--port PORT] [--root-url URL] [--job-instance-url URL]"
  echo "                  [--image IMAGE] [--image-archive FILE] [--data-volume NAME]"
  echo "                  [--install-dir DIR] [--no-start] [--no-autostart]"
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
    --no-autostart)
      no_autostart=true
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

for value in "$root_url" "$job_instance_url" "$image" "$data_volume" "$install_dir"; do
  if [[ "$value" == *$'\n'* || "$value" == *$'\r'* ]]; then
    echo "Configuration values cannot contain newlines" >&2
    exit 2
  fi
done

if [[ "$install_dir" == *'"'* ]]; then
  echo "Install directory cannot contain double quotes" >&2
  exit 2
fi

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
source_start_script="${script_dir}/start.sh"
[[ -f "$source_start_script" ]] || {
  echo "start.sh not found beside installer" >&2
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
mkdir -p "${install_dir}/logs"
cp "$source_compose" "${install_dir}/compose.yaml"
cp "$source_start_script" "${install_dir}/start.sh"
chmod 0755 "${install_dir}/start.sh"
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

configure_autostart() {
  local docker_bin=$1
  case "$(uname -s)" in
    Linux)
      command -v systemctl >/dev/null 2>&1 || {
        echo "systemd is required for Linux autostart; use --no-autostart on unsupported hosts" >&2
        exit 1
      }
      local systemctl_cmd=(systemctl)
      local install_cmd=(install)
      if ((EUID != 0)); then
        command -v sudo >/dev/null 2>&1 || {
          echo "root or sudo is required to enable Linux autostart" >&2
          exit 1
        }
        systemctl_cmd=(sudo systemctl)
        install_cmd=(sudo install)
      fi
      local unit_tmp
      unit_tmp=$(mktemp "${install_dir}/gitsss.service.XXXXXX")
      cat >"$unit_tmp" <<EOF
[Unit]
Description=GitSSS container service
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
Environment="GITSSS_INSTALL_DIR=${install_dir}"
ExecStart=${install_dir}/start.sh "\${GITSSS_INSTALL_DIR}" "${docker_bin}"
ExecStop=${docker_bin} compose --project-directory "\${GITSSS_INSTALL_DIR}" stop
TimeoutStartSec=150
TimeoutStopSec=150

[Install]
WantedBy=multi-user.target
EOF
      "${install_cmd[@]}" -m 0644 "$unit_tmp" /etc/systemd/system/gitsss.service
      rm -f "$unit_tmp"
      "${systemctl_cmd[@]}" daemon-reload
      "${systemctl_cmd[@]}" enable gitsss.service
      ;;
    Darwin)
      local uid
      uid=$(id -u)
      local launch_dir="${HOME}/Library/LaunchAgents"
      local plist="${launch_dir}/com.gitsss.autostart.plist"
      mkdir -p "$launch_dir"
      cat >"$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.gitsss.autostart</string>
  <key>ProgramArguments</key><array>
    <string>${install_dir}/start.sh</string>
    <string>${install_dir}</string>
    <string>${docker_bin}</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>${install_dir}/logs/launchd.out.log</string>
  <key>StandardErrorPath</key><string>${install_dir}/logs/launchd.err.log</string>
</dict></plist>
EOF
      launchctl bootout "gui/${uid}" "$plist" >/dev/null 2>&1 || true
      launchctl bootstrap "gui/${uid}" "$plist"
      launchctl enable "gui/${uid}/com.gitsss.autostart"
      ;;
    *)
      echo "Autostart is unsupported on $(uname -s); use --no-autostart" >&2
      exit 1
      ;;
  esac
}

if [[ "$no_start" == false && "$no_autostart" == false ]]; then
  configure_autostart "$(command -v docker)"
fi

echo "Installed: $install_dir"
echo "URL: $root_url"
if [[ "$no_start" == false && "$no_autostart" == false ]]; then
  echo "Autostart: enabled silently for $(uname -s)"
fi
echo "Switch port without data migration:"
echo "  $0 --install-dir '$install_dir' --data-volume '$data_volume' --port 3000 --root-url 'http://localhost:3000/'"
