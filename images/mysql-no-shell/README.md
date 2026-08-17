# mysql-no-shell

The MySQL 8.4 client, with no shell.

```
docker pull ghcr.io/fenleap/mysql-no-shell:latest
```

## What it removes

The MySQL client interprets `\!` itself, before anything reaches the server:

```
mysql> \! env | grep MYSQL_PWD     # prints the database password
mysql> \! sh                       # interactive shell in the container
```

No server-side guard can intercept that — it is not SQL, so a statement
allowlist never sees it, and it happens inside the client on the pod. It also
fires in non-interactive mode, on a continuation line, indented:

```
mysql --batch -e $'SELECT 1\n   \! id'    # runs id
```

`--disable-named-commands` does not disable the backslash forms. This image
contains no `/bin/sh`, no busybox and no package manager, so `\!` has nothing to
execute.

## Configuration

| Variable | Purpose |
|---|---|
| `DB_HOST`, `DB_PORT`, `DB_USER` | Discrete connection fields |
| `DB_DSN` | Connection string, parsed inside the container |
| `MYSQL_PWD` | Password — read by the client itself, never placed on a command line |
| `DB_NAME` | Default schema, when `--database` is not given |

`DB_DSN` accepts both `user:pass@tcp(host:port)/db` and
`[scheme://]user:pass@host:port/db`. It is split on the *first* colon and the
*last* `@`, so passwords containing `:` or `@` survive.

Without a default schema the client connects with none, and an unqualified
query fails with `ERROR 1046 (3D000): No database selected`. Set `DB_NAME`, pass
`--database`, or qualify names as `schema.table`.

A CA bundle at `/etc/db-tools/ca.pem` is detected automatically and switches the
client to `--ssl-ca=... --ssl-mode=VERIFY_CA`. Without one the client still
requires encryption (`--ssl-mode=REQUIRED`) but cannot authenticate the peer —
`--ssl-ca` alone would not verify, since the client's default ssl-mode is
PREFERRED.

## Usage

```
dbclient mysql                            interactive session
dbclient mysql --database <db>            select a default schema (-D)
dbclient mysql --batch --statement <SQL>  one statement, TSV on stdout
dbclient idle                             keep the container alive
```

```bash
docker run --rm -it \
  -e DB_HOST=db.internal -e DB_USER=app -e MYSQL_PWD=... \
  --read-only --tmpfs /tmp --cap-drop ALL \
  ghcr.io/fenleap/mysql-no-shell:latest mysql
```

Batch mode uses `--batch --quick`: tab-separated output with tab, newline and
backslash escaped inside values, so every row stays on one line, streamed rather
than buffered client-side. `--raw` is deliberately not used — it disables that
escaping, and one multi-line value would corrupt every row after it.

## What this does not fix

- **It does not hide the credential from the client's own user.** `\.` (source)
  and `\T` (tee) do file I/O inside the client without any shell; sourcing
  `/proc/self/environ` can still leak fragments through error messages. Use
  short-lived credentials (RDS IAM authentication) so a leak expires.
- **It does not make a read-write user read-only.** Grant `SELECT` only.
- **It does not restrict the network.** That is a NetworkPolicy's job.

## Contents

Oracle's MySQL Community client — not MariaDB's, because RDS certificate
verification depends on `--ssl-mode=VERIFY_CA`, which MariaDB spells
differently. Taken from the official `mysql:8.4` image, which Oracle builds for
both amd64 and arm64, and copied with its complete runtime into
`distroless/static`. See [NOTICE.md](NOTICE.md) — the client is GPL-2.0-only.
