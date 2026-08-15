#!/bin/sh
#
# client-version.sh <image-ref> <flavour>
#
# Print the upstream client version baked into a built image.
#
# Read out of the binary rather than taken from a build argument on purpose: the
# build argument says which upstream image was requested (`mysql:8.4`), while
# this says what actually landed (`8.4.11`). Those differ every time upstream
# publishes a patch under a moving tag, and the tag people pull should describe
# the artifact, not the request.
set -eu

image="${1:?usage: client-version.sh <image-ref> <flavour>}"
flavour="${2:?usage: client-version.sh <image-ref> <flavour>}"
DOCKER="${DOCKER:-docker}"

case "$flavour" in
  mysql) binary=/usr/bin/mysql ;;
  redis) binary=/usr/bin/redis-cli ;;
  *) echo "client-version: unknown flavour '$flavour'" >&2; exit 1 ;;
esac

# mysql:     "mysql  Ver 8.4.11 for Linux on aarch64 (MySQL Community Server - GPL)"
# redis-cli: "redis-cli 7.4.2"
raw=$("$DOCKER" run --rm --entrypoint "$binary" "$image" --version 2>/dev/null || true)
version=$(printf '%s' "$raw" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)

if [ -z "$version" ]; then
  echo "client-version: could not parse a version from: $raw" >&2
  exit 1
fi

printf '%s\n' "$version"
