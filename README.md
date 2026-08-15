# Fenleap Images

Minimal, hardened OCI images — no shell, no package manager, non-root by default.

Every image here exists because a widely used one has a property we did not want
in production. Each README says plainly what the image removes, what it does
**not** fix, and how that was verified.

## Catalogue

| Image | What it is | Size |
|---|---|---|
| [`mysql-no-shell`](images/mysql-no-shell) | MySQL 8.4 client with no shell | ~24 MB |
| [`redis-no-shell`](images/redis-no-shell) | Redis 7.4 client (TLS) with no shell | ~19 MB |

```
docker pull ghcr.io/fenleap/mysql-no-shell:8.4     # MySQL 8.4 client
docker pull ghcr.io/fenleap/redis-no-shell:7.4     # Redis 7.4 client
```

### Tags

Tags name the **client version**, because that is what you actually depend on —
our packaging number tells you nothing about whether your server will talk to it.

| Tag | Means | Moves? |
|---|---|---|
| `8.4.11` | exactly this client version | no |
| `8.4` | current patch of the 8.4 client series | yes |
| `8.4.11-1.0.0` | this client, packaged by this repo at revision 1.0.0 | **never** |
| `1.0.0` | the packaging release on its own | no |
| `latest` | newest of everything | yes |

Pin `8.4.11-1.0.0` when you want a bit-for-bit identical artifact — it is the
only tag that pins both halves. `8.4` is the sensible default for a client you
want kept patched. The client version is read out of the built binary during
release, so a tag always describes what shipped rather than what was requested.

## Why "no shell"

If you give people a database console backed by the official `mysql` image, you
have given them a shell on that pod and a copy of your database password —
whatever your application's permission model says. The MySQL client interprets
`\!` itself, before anything reaches the server:

```
mysql> \! env | grep MYSQL_PWD
MYSQL_PWD=<your production password>

mysql> \! sh
$ whoami
```

No server-side guard can stop this. It is not SQL, so a statement allowlist
never sees it. It fires in non-interactive `--batch -e` mode too, on a
continuation line, indented, and `--disable-named-commands` does not disable the
backslash forms.

The only reliable fix is to remove what `\!` spawns. Verified side by side
against a live MySQL 8.4 server, in a hardened container, with a canary variable
in the environment:

| payload | `mysql-no-shell` | official `mysql:8.4` |
|---|---|---|
| `SELECT id,email FROM users` | rows returned | rows returned |
| `\! env` | nothing — no shell to run it | environment printed |
| `\! sh -c 'env \| grep SECRET'` | SQL syntax error | `SECRET=...` leaked |

## What replaces the shell

A shell was doing two real jobs: keeping a container alive between sessions, and
expanding Secret-backed environment variables into client flags at exec time.
Both are handled by `dbclient`, the only other binary in these images:

```
dbclient idle                             keep the container alive
dbclient mysql                            interactive session
dbclient mysql --batch --statement <SQL>  one statement, TSV on stdout
dbclient redis                            interactive session
dbclient redis -- GET mykey               one command, raw output
```

It has a fixed subcommand set, never treats its arguments as a command, and
refuses flags it does not recognise — so a caller cannot smuggle in client
options such as `--pager`, which executes a program. It `execve`s the client, so
the client keeps PID 1, the TTY and signal handling. That also means the client
can *be* the pod, attached with `kubectl attach` instead of `kubectl exec`.

Connection details come from the environment and never reach a command line,
where they would be readable via `/proc/<pid>/cmdline`. See each image's README.

## Design rules

Everything here follows the same four:

1. **No shell, no package manager.** Enforced by a smoke test on the built
   artifact, not by intent — a base-image bump that reintroduces one fails CI.
2. **Non-root by default**, UID 65532, and expected to run with a read-only root
   filesystem and all capabilities dropped.
3. **The staged runtime is the only runtime.** Clients are copied into
   `distroless/static` with their libraries resolved by `ldd`, so there is
   exactly one libc and one loader in the filesystem.
4. **Honest limits.** Each README has a "what this does not fix" section. An
   image that oversells itself is worse than no image.

## Using them

```yaml
spec:
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    seccompProfile: { type: RuntimeDefault }
  containers:
    - name: db
      image: ghcr.io/fenleap/mysql-no-shell:latest
      args: ["idle"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities: { drop: ["ALL"] }
      volumeMounts: [{ name: tmp, mountPath: /tmp }]
  volumes:
    - name: tmp
      emptyDir: { medium: Memory, sizeLimit: 16Mi }
```

## Building

```
make list                        # the catalogue
make lint                        # gofmt + vet + launcher unit tests
make build IMAGE=mysql-no-shell
make test  IMAGE=mysql-no-shell  # build, then assert there is no shell
make test-all
```

`make test` is the meaningful target. It asserts on the built image that no
shell and no package manager are present, that the client still starts under
`--read-only --cap-drop ALL` as a non-root user, and that the launcher refuses
unknown subcommands and flags.

## Adding an image

Create `images/<name>/Dockerfile` and a `README.md` beside it. The Makefile and
both workflows discover images from the directory listing, so nothing else needs
editing. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Verifying what you pulled

Every published image carries an SBOM and a signed build provenance
attestation, so you can check it was built by this repository from the tag it
claims — rather than trusting the name:

```bash
gh attestation verify oci://ghcr.io/fenleap/mysql-no-shell:1.0.0 --owner Fenleap
docker buildx imagetools inspect ghcr.io/fenleap/mysql-no-shell:1.0.0
```

For an image whose entire value is what it does *not* contain, being auditable
is most of the point: the Dockerfile, the launcher source and the test that
enforces the claim are all in this repository.

## Releases

Images are versioned independently — a rebuild of one should not bump another's
version. Tag `<image>-v<semver>`:

```
git tag -a mysql-no-shell-v1.0.0 -m "mysql-no-shell 1.0.0"
git push origin mysql-no-shell-v1.0.0
```

That builds `linux/amd64` and `linux/arm64`, runs the smoke test as a release
gate, publishes to GHCR with an SBOM and provenance attestation, and cuts a
GitHub Release carrying the digest and verification instructions.

`workflow_dispatch` republishes an image without cutting a release — useful for
rebuilding on a base-image CVE without inventing a new version.

## Repository rules

These images are published publicly and make a security claim, so the paths
that change what people pull are deliberately narrow:

| | |
|---|---|
| `main` | Protected. Pull request only, with an approving review from the owner ([CODEOWNERS](.github/CODEOWNERS)); all CI checks must pass and be up to date; linear history; no force pushes or deletions. Direct pushes are restricted to the owner. |
| Release tags (`*-v*`) | A ruleset restricts creation, update and deletion to repository admins. |
| Publishing | The release workflow refuses to run for anyone outside `RELEASE_ACTORS`, so `workflow_dispatch` cannot be used to publish by another route. |
| Fork PRs | Workflows from external contributors require approval before they run. |
| Workflow token | Read-only by default; each job requests only what it needs. |

Contributions are welcome by pull request — see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

The original work here — the `dbclient` launcher, Dockerfiles, scripts and tests
— is [Apache-2.0](LICENSE). The images redistribute upstream binaries under
their own licences; each image's `NOTICE.md` records which, and what obligations
come with them.

Security policy: [SECURITY.md](SECURITY.md).
