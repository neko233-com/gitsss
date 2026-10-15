# Build frontend on the native platform to avoid QEMU-related issues with nodejs ecosystem.
FROM --platform=$BUILDPLATFORM docker.io/library/node:24-bookworm-slim AS frontend-build
ARG NPM_REGISTRY=https://registry.npmjs.org
RUN apt-get -o Acquire::Retries=5 update && \
    apt-get -o Acquire::Retries=5 install -y --no-install-recommends git make && \
    corepack enable && \
    COREPACK_NPM_REGISTRY="$NPM_REGISTRY" corepack prepare pnpm@10.33.0 --activate && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    npm_config_registry="$NPM_REGISTRY" pnpm install --frozen-lockfile
COPY --exclude=.git/ . .
RUN make lint-css && make frontend

# Keep the workflow runtime target-native when buildx produces arm64 images.
FROM docker.io/library/node:24-bookworm-slim AS node-runtime
ARG NPM_REGISTRY=https://registry.npmjs.org
RUN corepack enable && \
    COREPACK_HOME=/opt/corepack COREPACK_NPM_REGISTRY="$NPM_REGISTRY" \
    corepack prepare pnpm@10.33.0 --activate

# Build backend for each target platform
FROM docker.io/library/golang:1.27.2-bookworm AS build-env

ARG GITSSS_VERSION=development
ARG TAGS="sqlite sqlite_unlock_notify"
ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct
ENV TAGS="bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apt-get -o Acquire::Retries=5 update && \
    apt-get -o Acquire::Retries=5 install -y --no-install-recommends build-essential git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR ${GOPATH}/src/github.com/neko233-com/gitsss
COPY go.mod go.sum ./
COPY packages/actions-proto-go/go.mod packages/actions-proto-go/go.sum ./packages/actions-proto-go/
COPY packages/migration-sdk/go.mod packages/migration-sdk/go.sum ./packages/migration-sdk/
COPY tools/gitsss-vet/go.mod tools/gitsss-vet/go.sum ./tools/gitsss-vet/
COPY runner/go.mod runner/go.sum ./runner/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
RUN --mount=type=cache,target=/go/pkg/mod cd runner && go mod download
# Use COPY instead of bind mount as read-only one breaks makefile state tracking and read-write one needs binary to be moved as it's discarded.
# ".git" directory is mounted separately later only for version data extraction.
COPY --exclude=.git/ . .
COPY --from=frontend-build /src/public/assets public/assets

# Build gitsss, .git mount is required for version data
RUN --mount=type=cache,target="/root/.cache/go-build" \
    --mount=type=bind,source=".git/",target=".git/" \
    make backend GITSSS_VERSION="${GITSSS_VERSION}"

RUN --mount=type=cache,target="/root/.cache/go-build" \
    cd runner && \
    go build -trimpath \
      -ldflags "-s -w -X github.com/neko233-com/gitsss/runner/internal/pkg/ver.version=${GITSSS_VERSION}" \
      -o /go/src/github.com/neko233-com/gitsss/gitsss-runner .

COPY docker/root /tmp/local

# Set permissions for builds that made under windows which strips the executable bit from file
RUN chmod 755 /tmp/local/usr/bin/entrypoint \
              /tmp/local/usr/local/bin/* \
              /tmp/local/etc/s6/gitsss/* \
              /tmp/local/etc/s6/actions-runner/* \
              /tmp/local/etc/s6/openssh/* \
              /tmp/local/etc/s6/.s6-svscan/* \
              /go/src/github.com/neko233-com/gitsss/gitsss \
              /go/src/github.com/neko233-com/gitsss/gitsss-runner

FROM docker.io/library/golang:1.26.5-bookworm AS go-1-26-runtime

FROM docker.io/library/golang:1.25.5-bookworm AS go-1-25-runtime

FROM docker.io/library/ubuntu:24.04 AS gitsss

EXPOSE 22 5300

ARG TARGETARCH
ARG UBUNTU_MIRROR
ARG UBUNTU_PORTS_MIRROR
ENV DEBIAN_FRONTEND=noninteractive
RUN mirror="$UBUNTU_MIRROR" && \
    if [ "$TARGETARCH" = arm64 ]; then mirror="$UBUNTU_PORTS_MIRROR"; fi && \
    if [ -n "$mirror" ]; then \
      sed -i \
        -e "s|http://archive.ubuntu.com/ubuntu/|${mirror%/}/|g" \
        -e "s|http://security.ubuntu.com/ubuntu/|${mirror%/}/|g" \
        -e "s|http://ports.ubuntu.com/ubuntu-ports/|${mirror%/}/|g" \
        /etc/apt/sources.list.d/ubuntu.sources; \
    fi && \
    apt-get -o Acquire::Retries=5 update && \
    apt-get -o Acquire::Retries=5 install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    docker.io \
    gettext-base \
    git \
    git-lfs \
    gnupg \
    gosu \
    libpam0g \
    openssh-client \
    openssh-server \
    s6 \
    sqlite3 && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /run/sshd

RUN userdel ubuntu && \
    groupadd --gid 1000 git && \
    useradd --uid 1000 --gid git --home-dir /data/git --shell /bin/bash git && \
    usermod --password '*' git

COPY --from=build-env /tmp/local /
COPY --from=build-env /go/src/github.com/neko233-com/gitsss/gitsss /app/gitsss/gitsss
COPY --from=build-env /go/src/github.com/neko233-com/gitsss/gitsss-runner /app/gitsss/internal/runner
COPY --from=build-env /go/src/github.com/neko233-com/gitsss/actions/builtin /app/gitsss/builtin-actions
COPY --from=build-env /usr/local/go /usr/local/go
COPY --from=go-1-26-runtime /usr/local/go /opt/gitsss/toolchains/go1.26.5
COPY --from=go-1-25-runtime /usr/local/go /opt/gitsss/toolchains/go1.25.5
COPY --from=node-runtime /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
COPY --from=node-runtime /opt/corepack /opt/corepack
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    ln -sf ../lib/node_modules/corepack/dist/corepack.js /usr/local/bin/corepack && \
    ln -sf ../lib/node_modules/corepack/dist/pnpm.js /usr/local/bin/pnpm && \
    ln -sf ../lib/node_modules/corepack/dist/pnpx.js /usr/local/bin/pnpx && \
    ln -sf ../lib/node_modules/corepack/dist/yarn.js /usr/local/bin/yarn && \
    ln -sf ../lib/node_modules/corepack/dist/yarnpkg.js /usr/local/bin/yarnpkg

ENV COREPACK_HOME=/opt/corepack
ENV PATH="/usr/local/go/bin:${PATH}"

ENV USER=git
ENV GITSSS_CUSTOM=/data/gitsss

VOLUME ["/data"]

# HINT: HEALTH-CHECK-ENDPOINT: don't use HEALTHCHECK, search this hint keyword for more information
ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]
