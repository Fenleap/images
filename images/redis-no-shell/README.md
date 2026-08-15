# redis-no-shell

The Redis 7.4 client, built with TLS, with no shell.

```
docker pull ghcr.io/fenleap/redis-no-shell:latest
```

## What it removes

`redis-cli` has no shell-escape command of its own, so — unlike its MySQL
sibling — this image is not closing an open hole. It exists so both datastores
are reached through one hardened base: no shell means no interactive foothold if
a future client feature, a bug, or a sidecar ever provides one, and the same pod
spec (read-only root filesystem, no ServiceAccount token, dropped capabilities)
applies unchanged to both.

`redis-cli` is compiled from the official source with `BUILD_TLS=yes`, because
ElastiCache in-transit encryption needs a TLS-enabled client and distro
packaging of that flag varies by release.

## Configuration

| Variable | Purpose |
|---|---|
| `REDIS_HOST`, `REDIS_PORT`, `REDIS_USER` | Discrete connection fields |
| `REDIS_DSN` | `redis://` URL, passed straight to `redis-cli -u` |
| `REDISCLI_AUTH` | Password — read by the client itself |
| `REDIS_TLS=true` | In-transit encryption |
| `REDIS_CLUSTER=true` | Follow redirects on cluster-mode caches (`-c`) |

With `REDIS_TLS` and a CA bundle at `/etc/db-tools/ca.pem`, the client verifies
against it (`--cacert`). Without a bundle it connects encrypted but
unauthenticated (`--insecure`), which is what reaching an in-VPC ElastiCache
endpoint requires, since the image carries no root for its certificate.

## Usage

```
dbclient redis                interactive session
dbclient redis -- GET mykey   one command, raw output
dbclient idle                 keep the container alive
```

Command tokens after `--` are forwarded as separate argv entries and never
concatenated into a string, so `;`, quotes and `$(...)` in a key name are inert.

```bash
docker run --rm -it \
  -e REDIS_HOST=cache.internal -e REDISCLI_AUTH=... -e REDIS_TLS=true \
  --read-only --tmpfs /tmp --cap-drop ALL \
  ghcr.io/fenleap/redis-no-shell:latest redis
```

Non-interactive output is raw — one value per line, no `1)` index prefixes —
which is what makes it easy to pipe into a file or a CSV converter.

## What this does not fix

- **`KEYS` still blocks the server.** It walks the entire keyspace; prefer
  `SCAN` on a busy cache. The image cannot protect you from an expensive
  command.
- **It does not make a writable ACL read-only.** Use a Redis ACL such as
  `+@read -@write -@dangerous`.
- **It does not restrict the network.**

## Contents

`redis-cli` built from the official Redis source tarball, unmodified apart from
the TLS build flag, copied with its complete runtime into `distroless/static`.
See [NOTICE.md](NOTICE.md) — BSD-3-Clause.
