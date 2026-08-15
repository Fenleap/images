# syntax=docker/dockerfile:1
#
# redis-no-shell — a Redis client image with no shell.
#
# redis-cli has no shell-escape command of its own, so this image is not closing
# the same hole as its MySQL sibling. It is here so both datastores are reached
# through one hardened, auditable base: no shell means no interactive foothold
# if a future client feature, a bug, or a sidecar ever provides one, and it lets
# the same pod spec (read-only rootfs, no ServiceAccount token, dropped
# capabilities) apply unchanged to both.
#
# redis-cli is built from source rather than taken from Debian, because
# ElastiCache in-transit encryption needs a client compiled with BUILD_TLS=yes
# and distro packaging of that flag varies by release.
#
# Build:
#   docker build -f images/redis-no-shell/Dockerfile -t redis-no-shell .
# (context is docker/no-shell-clients/, not this directory)

ARG GO_VERSION=1.22
ARG DEBIAN_RELEASE=bookworm
ARG REDIS_VERSION=7.4.2

# ── 1. Launcher ──────────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS launcher
WORKDIR /src
COPY shared/launcher/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/dbclient .
RUN go vet ./... && go test ./...

# ── 2. Client ────────────────────────────────────────────────────────────────
FROM debian:${DEBIAN_RELEASE}-slim AS client
ARG REDIS_VERSION
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      build-essential ca-certificates curl libssl-dev pkg-config; \
    rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    curl -fsSL "https://download.redis.io/releases/redis-${REDIS_VERSION}.tar.gz" -o /tmp/redis.tar.gz; \
    mkdir -p /usr/src/redis; \
    tar -xzf /tmp/redis.tar.gz -C /usr/src/redis --strip-components=1; \
    # The top-level Makefile forwards unknown targets to src/, so this builds
    # redis-cli and its vendored deps (hiredis, linenoise) without the server.
    make -C /usr/src/redis BUILD_TLS=yes redis-cli -j"$(nproc)"; \
    install -m0755 /usr/src/redis/src/redis-cli /usr/bin/redis-cli; \
    redis-cli --version

COPY shared/scripts/collect-libs.sh /usr/local/bin/collect-libs
RUN chmod +x /usr/local/bin/collect-libs && collect-libs /usr/bin/redis-cli /staging

# ── 3. Final: distroless, no shell, no package manager ───────────────────────
#
# `static`, not `cc`: collect-libs stages redis-cli's complete runtime including
# its loader, so a base that also ships glibc would only add a second copy to
# reason about. Keeping the staged runtime the only runtime matches the MySQL
# image and makes the image's contents exactly what the SBOM says.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=client /staging /
COPY --from=launcher /out/dbclient /usr/bin/dbclient

USER 65532:65532

ENV HOME=/tmp \
    REDISCLI_HISTFILE=/dev/null

ARG VERSION=dev
# The client version, discovered from the built binary by the release workflow
# and fed back in. Empty on a local build, where it is not worth a second pass.
ARG CLIENT_VERSION=""
LABEL org.opencontainers.image.title="redis-no-shell" \
      org.opencontainers.image.description="Shell-less Redis client for database consoles, built with TLS for ElastiCache." \
      org.opencontainers.image.version="${CLIENT_VERSION}" \
      com.fenleap.packaging.version="${VERSION}" \
      org.opencontainers.image.vendor="Fenleap" \
      org.opencontainers.image.licenses="Apache-2.0 AND BSD-3-Clause" \
      org.opencontainers.image.source="https://github.com/Fenleap/images" \
      org.opencontainers.image.documentation="https://github.com/Fenleap/fenwave-templates/tree/main/docker/no-shell-clients"

ENTRYPOINT ["/usr/bin/dbclient"]
CMD ["idle"]
