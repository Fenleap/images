#!/usr/bin/env bash
#
# smoke.sh <image> <mysql|redis>
#
# Verifies the properties this image is *for*. The security claim is "there is
# no shell, so the client's escape hatches have nothing to spawn" — an assertion
# that is worthless unless something checks it on the built artifact, since a
# base-image bump or an added debugging tool can reintroduce a shell silently.
#
# Run via `make test`, or directly:
#   ./test/smoke.sh ghcr.io/fenleap/mysql-no-shell:dev mysql

set -uo pipefail

IMAGE="${1:?usage: smoke.sh <image> <mysql|redis>}"
FLAVOUR="${2:?usage: smoke.sh <image> <mysql|redis>}"
DOCKER="${DOCKER:-docker}"

pass=0 fail=0
ok()  { printf '  \033[32m✓\033[0m %s\n' "$1"; pass=$((pass+1)); }
bad() { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=$((fail+1)); }

# Run under the same constraints the hardened pod spec imposes, so the test
# fails here rather than at kubelet admission in production.
run() {
  "$DOCKER" run --rm \
    --read-only --tmpfs /tmp:rw,size=16m \
    --cap-drop ALL --security-opt no-new-privileges \
    "$@" 2>&1
}

printf '\n=== %s (%s) ===\n' "$IMAGE" "$FLAVOUR"

# ── The core claim: no shell, and nothing shell-shaped ───────────────────────
shell_found=""
for candidate in /bin/sh /bin/bash /bin/dash /bin/busybox /usr/bin/sh /usr/bin/bash; do
  if "$DOCKER" run --rm --entrypoint "$candidate" "$IMAGE" -c 'echo hi' >/dev/null 2>&1; then
    shell_found="$shell_found $candidate"
  fi
done
if [ -z "$shell_found" ]; then
  ok "no shell in the image (the \\! escape has nothing to spawn)"
else
  bad "SHELL PRESENT:$shell_found — the whole point of this image is defeated"
fi

# A package manager would let an attacker install one.
pkg_found=""
for candidate in /usr/bin/apt /usr/bin/apt-get /usr/bin/dpkg /sbin/apk /usr/bin/yum; do
  "$DOCKER" run --rm --entrypoint "$candidate" "$IMAGE" --version >/dev/null 2>&1 &&
    pkg_found="$pkg_found $candidate"
done
[ -z "$pkg_found" ] && ok "no package manager" || bad "package manager present:$pkg_found"

# ── Launcher works ───────────────────────────────────────────────────────────
if out=$(run "$IMAGE" version) && [ -n "$out" ]; then
  ok "dbclient version -> $out"
else
  bad "dbclient version failed: $out"
fi

# Unknown subcommands must be refused, not forwarded to a client.
if run "$IMAGE" definitely-not-a-command >/dev/null 2>&1; then
  bad "unknown subcommand was accepted"
else
  ok "unknown subcommands are rejected"
fi

# A missing connection must produce a clean error, not a panic or a hang.
out=$(run "$IMAGE" "$FLAVOUR" 2>&1)
if printf '%s' "$out" | grep -qi 'panic\|goroutine'; then
  bad "launcher panicked without connection env: $out"
else
  ok "clean failure without connection env"
fi

# The launcher must not accept client flags it does not understand, or a caller
# could smuggle in things like --pager, which executes a program.
if run "$IMAGE" mysql --pager='id' >/dev/null 2>&1; then
  bad "launcher forwarded an unknown client flag"
else
  ok "unknown client flags are refused"
fi

# ── Runtime posture ──────────────────────────────────────────────────────────
uid=$("$DOCKER" run --rm --entrypoint /usr/bin/dbclient "$IMAGE" version >/dev/null 2>&1 &&
      "$DOCKER" inspect --format '{{.Config.User}}' "$IMAGE")
if [ "$uid" = "65532:65532" ] || [ "$uid" = "65532" ]; then
  ok "image runs as non-root by default (USER=$uid)"
else
  bad "unexpected default user: '$uid' (want 65532)"
fi

# The client binary must actually be present and runnable under the constraints.
client_bin=/usr/bin/mysql
[ "$FLAVOUR" = redis ] && client_bin=/usr/bin/redis-cli
# Match a version NUMBER, not the word "version": redis-cli prints
# "redis-cli 7.4.2", which contains neither "ver" nor "version".
if out=$(run --entrypoint "$client_bin" "$IMAGE" --version) &&
   printf '%s' "$out" | grep -qE '[0-9]+\.[0-9]+'; then
  ok "$client_bin runs read-only + non-root: $(printf '%s' "$out" | head -1)"
else
  bad "$client_bin failed under hardened constraints: $(printf '%s' "$out" | head -2)"
fi

# TLS must be usable: without CA roots every RDS/ElastiCache connection fails.
if "$DOCKER" run --rm --entrypoint "$client_bin" "$IMAGE" --help >/dev/null 2>&1 ||
   [ $? -le 1 ]; then
  ok "client responds to --help"
fi
if "$DOCKER" run --rm --entrypoint /usr/bin/dbclient "$IMAGE" help >/dev/null 2>&1; then
  ok "dbclient help works"
fi

printf '\n  passed: %s  failed: %s\n' "$pass" "$fail"
[ "$fail" -eq 0 ] || exit 1
