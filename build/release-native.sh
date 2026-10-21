#!/usr/bin/env bash
# Cross-compile gitsss on the host. Go's build cache is safe for concurrent
# builds; xz runs only after the compiles finish so the two phases do not
# fight for CPU.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

dry=0
if [ "${1:-}" = "--dry-run" ]; then
	dry=1
fi

version=${VERSION:-}
if [ -z "$version" ]; then
	version=$(git describe --tags --always | sed 's/^v//; s/-/+/')
fi
jobs=${RELEASE_JOBS:-2}
xz_level=${XZ_LEVEL:-6}
gogit=${RELEASE_GOGIT:-0}
out_dir=${DIST:-dist}/release
tags="bindata sqlite sqlite_unlock_notify timetzdata netgo osusergo"
ldflags="-s -w -X main.Version=${version} -X main.Tags=${tags// /,}"

platforms=(
	"linux amd64 gitsss-${version}-linux-amd64"
	"linux arm64 gitsss-${version}-linux-arm64"
	"darwin amd64 gitsss-${version}-darwin-amd64"
	"darwin arm64 gitsss-${version}-darwin-arm64"
	"windows amd64 gitsss-${version}-windows-amd64.exe"
	"windows arm64 gitsss-${version}-windows-arm64.exe"
	"windows 386 gitsss-${version}-windows-386.exe"
)
if [ "$gogit" = "1" ]; then
	platforms+=(
		"windows amd64 gitsss-${version}-gogit-windows-amd64.exe gogit"
		"windows arm64 gitsss-${version}-gogit-windows-arm64.exe gogit"
		"windows 386 gitsss-${version}-gogit-windows-386.exe gogit"
	)
fi

build_one() {
	local goos=$1 goarch=$2 name=$3 extra=${4:-}
	local build_tags=$tags
	if [ -n "$extra" ]; then
		build_tags="$build_tags $extra"
	fi
	local helper="gitsss-hook-${version}-${goos}-${goarch}"
	if [ "$goos" = windows ]; then helper+='.exe'; fi
	if [ "$dry" -eq 1 ]; then
		printf 'GOOS=%s GOARCH=%s go build -tags %q -ldflags %q -o %s/%s\n' \
			"$goos" "$goarch" "$build_tags" "$ldflags" "$out_dir" "$name"
		if [ -z "$extra" ]; then
			printf 'GOOS=%s GOARCH=%s go build -ldflags %q -o %s/%s ./cmd/gitsss-hook\n' \
				"$goos" "$goarch" '-s -w' "$out_dir" "$helper"
		fi
		return 0
	fi
	echo "build $name"
	GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -trimpath -buildvcs=false \
		-tags "$build_tags" -ldflags "$ldflags" -o "$out_dir/$name" .
	if [ -z "$extra" ]; then
		GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -trimpath -buildvcs=false \
			-ldflags '-s -w' -o "$out_dir/$helper" ./cmd/gitsss-hook
	fi
}

if [ "$dry" -eq 0 ]; then
	mkdir -p "$out_dir"
fi

status=0
pids=()
for spec in "${platforms[@]}"; do
	if [ "$status" -ne 0 ]; then
		break
	fi
	if [ "$dry" -eq 1 ]; then
		# shellcheck disable=SC2086
		build_one $spec
		continue
	fi
	if [ "${#pids[@]}" -ge "$jobs" ]; then
		wait "${pids[0]}" || status=1
		pids=("${pids[@]:1}")
	fi
	if [ "$status" -ne 0 ]; then
		break
	fi
	# shellcheck disable=SC2086
	build_one $spec &
	pids+=("$!")
done

if [ "$dry" -eq 1 ]; then
	exit 0
fi

for pid in "${pids[@]}"; do
	wait "$pid" || status=1
done
if [ "$status" -ne 0 ]; then
	exit "$status"
fi

echo "compress xz -$xz_level with $jobs jobs"
compress_one() {
	local file=$1
	if command -v xz >/dev/null 2>&1; then
		xz -T1 -k "-$xz_level" -f "$file"
	else
		local python=${PYTHON:-python3}
		"$python" - "$file" "$xz_level" <<'PY'
import lzma
import os
import shutil
import sys

path = sys.argv[1]
temporary = path + ".xz.tmp"
try:
    with open(path, "rb") as source, lzma.open(temporary, "wb", preset=int(sys.argv[2])) as target:
        shutil.copyfileobj(source, target)
    os.replace(temporary, path + ".xz")
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY
	fi
	sha256sum "$file" | awk -v name="$(basename "$file")" '{print $1"  "name}' > "$file.sha256"
	sha256sum "$file.xz" | awk -v name="$(basename "$file.xz")" '{print $1"  "name}' > "$file.xz.sha256"
}
status=0
pids=()
for file in "$out_dir"/*; do
	case $file in
	*.xz | *.sha256) continue ;;
	esac
	[ -f "$file" ] || continue
	if [ "${#pids[@]}" -ge "$jobs" ]; then
		wait "${pids[0]}" || status=1
		pids=("${pids[@]:1}")
	fi
	if [ "$status" -ne 0 ]; then
		break
	fi
	compress_one "$file" &
	pids+=("$!")
done
for pid in "${pids[@]}"; do
	wait "$pid" || status=1
done
exit "$status"
