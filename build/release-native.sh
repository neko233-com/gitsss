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
if [ "$dry" -eq 0 ]; then
	if [ "${RELEASE_ALLOW_DIRTY:-0}" != 1 ]; then
		dirty=$(git status --porcelain --untracked-files=all -- . ':(exclude).evidence' ':(exclude)dist')
		if [ -n "$dirty" ]; then
			echo "refusing to package a dirty tree; commit the release sources or set RELEASE_ALLOW_DIRTY=1" >&2
			printf '%s\n' "$dirty" | head -20 >&2
			exit 1
		fi
	fi
	if ! command -v node >/dev/null 2>&1; then
		echo "node is required to verify the frontend build stamp" >&2
		exit 1
	fi
	node tools/frontend-build.mjs
fi
jobs=${RELEASE_JOBS:-2}
xz_level=${XZ_LEVEL:-6}
gogit=${RELEASE_GOGIT:-0}
out_dir=${DIST:-dist}/release
tags="bindata sqlite sqlite_unlock_notify timetzdata netgo osusergo"
# Go no longer records -X flags in build info; a version tag keeps artifacts verifiable.
version_tag="gitsss_release_$(printf '%s' "$version" | sed 's/[^A-Za-z0-9]/_/g')"
build_tags="$tags $version_tag"
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
	local compile_tags=$build_tags
	if [ -n "$extra" ]; then
		compile_tags="$compile_tags $extra"
	fi
	local helper="gitsss-hook-${version}-${goos}-${goarch}"
	if [ "$goos" = windows ]; then helper+='.exe'; fi
	if [ "$dry" -eq 1 ]; then
		printf 'GOOS=%s GOARCH=%s go build -tags %q -ldflags %q -o %s/%s\n' \
			"$goos" "$goarch" "$compile_tags" "$ldflags" "$out_dir" "$name"
		if [ -z "$extra" ]; then
			printf 'GOOS=%s GOARCH=%s go build -ldflags %q -o %s/%s ./cmd/gitsss-hook\n' \
				"$goos" "$goarch" '-s -w' "$out_dir" "$helper"
		fi
		return 0
	fi
	echo "build $name"
	GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -trimpath -buildvcs=false \
		-tags "$compile_tags" -ldflags "$ldflags" -o "$out_dir/$name" .
	if [ -z "$extra" ]; then
		GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -trimpath -buildvcs=false \
			-ldflags '-s -w' -o "$out_dir/$helper" ./cmd/gitsss-hook
	fi
}

if [ "$dry" -eq 0 ]; then
	mkdir -p "$out_dir"
	find "$out_dir" -maxdepth 1 -type f \( -name "gitsss-${version}-*" -o -name "gitsss-src-${version}.tar.gz*" -o -name SHA256SUMS \) -delete
	"${PYTHON:-python3}" build/package-source.py "$out_dir" "$version"
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
	*.xz | *.sha256 | *.tar.gz | *.json | */SHA256SUMS) continue ;;
	esac
	case $(basename "$file") in gitsss-*) ;; *) continue ;; esac
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
if [ "$status" -eq 0 ]; then
	"${PYTHON:-python3}" build/package-native.py "$out_dir" "$version" || status=1
	if [ "$status" -eq 0 ]; then
		if [ "$gogit" = "1" ]; then
			"${PYTHON:-python3}" build/verify-release.py "$out_dir" "$version" --gogit || status=1
		else
			"${PYTHON:-python3}" build/verify-release.py "$out_dir" "$version" || status=1
		fi
	fi
fi
exit "$status"
