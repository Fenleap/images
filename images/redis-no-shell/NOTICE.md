# Third-party notices — redis-no-shell

The original work in this repository (the `dbclient` launcher, Dockerfiles,
scripts and tests) is © Fenleap, Apache-2.0. This image additionally
redistributes upstream software:

| Component | Licence | Source |
|---|---|---|
| `redis-cli` | BSD-3-Clause (Redis 7.4 and earlier) | <https://download.redis.io/releases/> |
| hiredis, linenoise (vendored by Redis) | BSD-3-Clause | bundled in the Redis source tarball |
| Debian runtime libraries | Various | Debian bookworm |
| Base image | Apache-2.0 | `gcr.io/distroless/static-debian12` |

`redis-cli` is compiled from the official source tarball with `BUILD_TLS=yes`
and is otherwise unmodified.

Note that Redis changed licence after 7.4 (RSALv2 / SSPLv1). The pinned
`REDIS_VERSION` build argument determines which terms apply and should be
reviewed before bumping it.

```
docker buildx imagetools inspect ghcr.io/fenleap/redis-no-shell:latest
```
