#!/bin/sh
# Run a Makefile target on the host when Docker is unavailable or
# GITSSS_TASK_NATIVE=1. Otherwise use the reproducible task container.
set -eu

target=$1
mode=${2:-privileged}
root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
cd "$root"

if [ "${GITSSS_TASK_NATIVE:-}" = "1" ] || ! docker info >/dev/null 2>&1; then
	echo "gitsss: make ${target} on the host"
	exec make "$target"
fi

image=${BUILD_IMAGE:-gitsss/gitsss:latest}
if [ "$mode" = "unprivileged" ]; then
	exec docker run --rm --user git --entrypoint /bin/bash \
		--volume "${root}:/src" \
		--workdir /src \
		--env HOME=/tmp/git-home \
		--env GOPATH=/tmp/go \
		--env GOCACHE=/tmp/go-build \
		--env GITSSS_CUSTOM= \
		--env GIT_CONFIG_COUNT=1 \
		--env GIT_CONFIG_KEY_0=safe.directory \
		--env GIT_CONFIG_VALUE_0=/src \
		--env npm_config_registry \
		--env TAGS \
		--env RACE_ENABLED \
		--env GOTESTFLAGS \
		--env GITEA_TEST_E2E_FLAGS \
		--env VERSION \
		--env GITSSS_VERSION \
		--env DOCKER_IMAGE \
		--env DOCKER_TAG \
		"$image" \
		-lc "mkdir -p /tmp/git-home && git config --global --add safe.directory /src && make ${target}"
fi

exec docker run --rm --entrypoint /bin/bash \
	--volume "${root}:/src" \
	--volume gitsss-task-node-modules:/src/node_modules \
	--volume gitsss-task-pnpm-store:/root/.local/share/pnpm/store \
	--volume gitsss-task-go-mod:/go/pkg/mod \
	--volume gitsss-task-go-build:/root/.cache/go-build \
	--workdir /src \
	--env npm_config_registry \
	--env TAGS \
	--env RACE_ENABLED \
	--env GOTESTFLAGS \
	--env GITEA_TEST_E2E_FLAGS \
	--env VERSION \
	--env GITSSS_VERSION \
	--env DOCKER_IMAGE \
	--env DOCKER_TAG \
	"$image" \
	-lc "make ${target}"
