#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: prepare-gitea-takeover.sh --source-volume NAME --target-volume NAME [--image IMAGE]

Clone a stopped Gitea Docker data volume into an independently verified
gitsss takeover candidate. The source volume is always mounted read-only.
EOF
}

source_volume=
target_volume=
image=gitsss/gitsss:dev
target_created=false

while [ "$#" -gt 0 ]; do
	case "$1" in
	--source-volume)
		source_volume=${2-}
		shift 2
		;;
	--target-volume)
		target_volume=${2-}
		shift 2
		;;
	--image)
		image=${2-}
		shift 2
		;;
	-h | --help)
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

case "$source_volume" in
"" | *[!A-Za-z0-9_.-]*)
	echo "Invalid or missing --source-volume" >&2
	exit 2
	;;
esac
case "$target_volume" in
"" | *[!A-Za-z0-9_.-]*)
	echo "Invalid or missing --target-volume" >&2
	exit 2
	;;
esac
if [ "$source_volume" = "$target_volume" ]; then
	echo "Source and target volumes must differ" >&2
	exit 2
fi

cleanup_failed_target() {
	status=$?
	if [ "$status" -ne 0 ] && [ "$target_created" = true ]; then
		docker volume rm "$target_volume" >/dev/null 2>&1 || true
	fi
}
trap cleanup_failed_target EXIT

docker info >/dev/null
docker image inspect "$image" >/dev/null 2>&1 || {
	echo "Image is not available locally: $image" >&2
	exit 1
}
docker volume inspect "$source_volume" >/dev/null 2>&1 || {
	echo "Source volume does not exist: $source_volume" >&2
	exit 1
}
if docker volume inspect "$target_volume" >/dev/null 2>&1; then
	echo "Target volume already exists: $target_volume" >&2
	exit 1
fi

running_users=
for container_id in $(docker ps -q); do
	if docker inspect --format '{{range .Mounts}}{{if eq .Type "volume"}}{{println .Name}}{{end}}{{end}}' "$container_id" | grep -Fqx "$source_volume"; then
		container_name=$(docker inspect --format '{{.Name}}' "$container_id" | sed 's#^/##')
		running_users="${running_users}${running_users:+, }${container_name}"
	fi
done
if [ -n "$running_users" ]; then
	echo "Source volume is mounted by running container(s): $running_users" >&2
	echo "Stop Gitea before preparing the takeover candidate." >&2
	exit 1
fi

docker volume create \
	--label gitsss.takeover.source="$source_volume" \
	--label gitsss.takeover.role=candidate \
	"$target_volume" >/dev/null
target_created=true

docker run --rm --pull=never \
	--network none \
	--read-only \
	--cap-drop ALL \
	--cap-add CHOWN \
	--cap-add DAC_OVERRIDE \
	--cap-add FOWNER \
	--security-opt no-new-privileges:true \
	--entrypoint /bin/sh \
	--volume "$source_volume:/source:ro" \
	--volume "$target_volume:/target" \
	"$image" -ec 'cd /source && tar --format=pax --numeric-owner -cpf - . | tar --numeric-owner -xpf - -C /target'

volume_hash() {
	volume=$1
	docker run --rm --pull=never \
		--network none \
		--read-only \
		--cap-drop ALL \
		--cap-add DAC_READ_SEARCH \
		--security-opt no-new-privileges:true \
		--entrypoint /bin/sh \
		--volume "$volume:/volume:ro" \
		"$image" -ec '
			cd /volume
			{
				printf "content\0"
				find . -xdev -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum
				printf "metadata\0"
				find . -xdev -printf "%P\t%y\t%m\t%U\t%G\t%s\t%T@\t%l\0" | LC_ALL=C sort -z
			} | sha256sum
		' |
		awk '{print $1}'
}

source_hash=$(volume_hash "$source_volume")
target_hash=$(volume_hash "$target_volume")
if [ "$source_hash" != "$target_hash" ]; then
	echo "Candidate hash mismatch" >&2
	echo "source=$source_hash" >&2
	echo "target=$target_hash" >&2
	exit 1
fi

docker run --rm --pull=never \
	--network none \
	--read-only \
	--cap-drop ALL \
	--cap-add DAC_READ_SEARCH \
	--security-opt no-new-privileges:true \
	--entrypoint /bin/sh \
	--volume "$target_volume:/candidate:ro" \
	"$image" -ec '
		config=
		for candidate in /candidate/gitea/conf/app.ini /candidate/gitsss/conf/app.ini; do
			if [ -f "$candidate" ]; then
				config=$candidate
				break
			fi
		done
		if [ -z "$config" ]; then
			echo "No installed Gitea or gitsss app.ini found in candidate" >&2
			exit 1
		fi
		if ! grep -Eiq "^[[:space:]]*INSTALL_LOCK[[:space:]]*=[[:space:]]*true([[:space:]]|$)" "$config"; then
			echo "Candidate app.ini is not an installed instance: $config" >&2
			exit 1
		fi
		if [ ! -d /candidate/git/repositories ] && [ ! -d /candidate/gitea/data/gitea-repositories ] && [ ! -d /candidate/gitsss/data/gitsss-repositories ]; then
			echo "No standard repository root found in candidate" >&2
			exit 1
		fi
		printf "config=%s\n" "$config"
		printf "files=%s\n" "$(find /candidate -xdev -type f | wc -l)"
	'

target_created=false
trap - EXIT

echo "Takeover candidate prepared"
echo "source_volume=$source_volume"
echo "target_volume=$target_volume"
echo "content_sha256=$target_hash"
echo "Start gitsss with GITSSS_DATA_VOLUME=$target_volume; keep $source_volume unchanged for rollback."
