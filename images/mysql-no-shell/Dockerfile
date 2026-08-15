# syntax=docker/dockerfile:1
#
# mysql-no-shell — the MySQL client, without a shell.
#
# A normal mysql client image ships /bin/sh, and the MySQL client interprets
# `\!` itself, before anything reaches the server:
#
#     mysql> \! env | grep MYSQL_PWD      # prints the database password
#     mysql> \! sh                        # interactive shell in the container
#
# No server-side guard can stop that — it is not SQL, so a statement allowlist
# never sees it. It fires in non-interactive `--batch -e` mode too, on a
# continuation line, indented, and `--disable-named-commands` does not disable
# the backslash forms. The only reliable fix is to remove what `\!` spawns.
#
# This image contains no shell, no busybox and no package manager. The two jobs
# a shell was doing — idling the container and expanding environment variables
# into client flags — are handled by `dbclient`, which offers exactly those two
# capabilities and no way to run anything else.
#
# Build (context is the repository root):
#   docker build -f images/mysql-no-shell/Dockerfile -t mysql-no-shell .

ARG GO_VERSION=1.22
ARG DEBIAN_RELEASE=bookworm
# The official image is the client source rather than Oracle's APT repo, which
# publishes only i386 and amd64 for Debian and so cannot produce an arm64 image
# at all. The official image is built by Oracle for both linux/amd64 and
# linux/arm64/v8, which also removes a dependency on a signing key that expires
# and silently breaks the build.
ARG MYSQL_IMAGE=mysql:8.4

# ── 1. Launcher ──────────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS launcher
WORKDIR /src
COPY shared/launcher/ ./
ARG VERSION=dev
# CGO off so the result is static and adds no libc dependency of its own.
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/dbclient .
RUN go vet ./... && go test ./...

# ── 2. Client ────────────────────────────────────────────────────────────────
# Oracle's client, not MariaDB's: RDS certificate verification depends on
# `--ssl-mode=VERIFY_CA`, which the MariaDB client spells differently.
FROM ${MYSQL_IMAGE} AS client
COPY shared/scripts/collect-libs.sh /tmp/collect-libs.sh
# Runs as root here only to read the system libraries; nothing from this stage
# except the staged tree reaches the final image.
USER root
RUN sh /tmp/collect-libs.sh /usr/bin/mysql /staging && mysql --version

# ── 3. Final: distroless, no shell, no package manager ───────────────────────
#
# `static`, not `cc`: the client comes from an Oracle Linux image and brings its
# own glibc AND its own dynamic loader. A base that also ships glibc would mean
# two of each in one filesystem, with the loader and libc resolving from
# different distributions — a version pairing that is not supported and fails in
# ways that are miserable to debug. Starting from a base with no libc at all
# makes the staged runtime the only runtime.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=client /staging /
COPY --from=launcher /out/dbclient /usr/bin/dbclient

# The staged libraries live where Oracle Linux puts them, which is not a
# directory Debian's conventions would search. Say so explicitly rather than
# depending on a loader's built-in defaults.
ENV LD_LIBRARY_PATH=/lib64:/usr/lib64

# 65532 is distroless's "nonroot" user. Set in the image as well as in any pod
# spec, so the image is safe by default for anyone running it directly.
USER 65532:65532

# The root filesystem is expected to be mounted read-only, so give the client
# and its history file somewhere harmless to go.
ENV HOME=/tmp \
    MYSQL_HISTFILE=/dev/null

ARG VERSION=dev
LABEL org.opencontainers.image.title="mysql-no-shell" \
      org.opencontainers.image.description="MySQL client with no shell, so the client's \\! escape cannot spawn one." \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.vendor="Fenleap" \
      org.opencontainers.image.licenses="Apache-2.0 AND GPL-2.0-only" \
      org.opencontainers.image.source="https://github.com/Fenleap/images" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian12:nonroot"

ENTRYPOINT ["/usr/bin/dbclient"]
CMD ["idle"]
