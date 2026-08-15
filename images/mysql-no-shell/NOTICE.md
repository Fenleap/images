# Third-party notices — mysql-no-shell

The original work in this repository (the `dbclient` launcher, Dockerfiles,
scripts and tests) is © Fenleap, Apache-2.0. This image additionally
redistributes unmodified upstream binaries:

| Component | Licence | Source |
|---|---|---|
| MySQL Community client (`mysql`) | GPL-2.0-only, with the FOSS License Exception | <https://dev.mysql.com/downloads/mysql/> |
| Oracle Linux runtime libraries (glibc, OpenSSL, ncurses, libstdc++, zlib) | Various (LGPL-2.1+, Apache-2.0, MIT, BSD) | the official `mysql` image |
| Base image | Apache-2.0 | `gcr.io/distroless/static-debian12` |

The MySQL client is **GPL-2.0-only**. Publishing this image is a distribution of
that binary and carries an obligation to make the corresponding source
available. Oracle publishes it at the URL above; the exact version is recorded
in the `org.opencontainers.image.version` label and the `MYSQL_IMAGE` build
argument. The binary is copied unmodified.

Inspect what is inside any published image:

```
docker buildx imagetools inspect ghcr.io/fenleap/mysql-no-shell:latest
```
