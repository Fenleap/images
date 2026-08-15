# Security policy

## Reporting a vulnerability

Report privately via [GitHub Security Advisories](https://github.com/Fenleap/images/security/advisories/new),
or by email to **security@fenleap.com**. Please do not open a public issue for
an unfixed vulnerability.

Include the image and tag, the digest if you have it (`docker buildx imagetools
inspect ghcr.io/fenleap/<image>:<tag>`), and how to reproduce.

## What these images promise

Each image states its claim in its own README, and a smoke test enforces it on
the built artifact. For the `-no-shell` family the claim is narrow and precise:

- The image contains **no shell and no package manager**, so a client-side
  escape such as the MySQL client's `\!` has nothing to spawn.
- The image runs as **non-root (UID 65532)** and works under a read-only root
  filesystem with all capabilities dropped.

If you can obtain a shell, run an arbitrary program, or escalate to root inside
one of these images, that is a vulnerability in the image and we want to hear
about it.

## What they do not promise

Being explicit here is part of the security posture, not a disclaimer:

- **They do not hide credentials from the client's own user.** The client must
  authenticate, and `\.` (source) and `\T` (tee) do file I/O inside the client
  without any shell. Short-lived credentials are what make a leaked one
  worthless.
- **They do not make a read-write database user read-only.** That has to come
  from the database: a `SELECT`-only grant, or a Redis ACL.
- **They do not restrict the network.** Egress control is a NetworkPolicy's job.

## Supply chain

Every published image carries an SBOM and a build provenance attestation:

```
docker buildx imagetools inspect ghcr.io/fenleap/<image>:<tag>
gh attestation verify oci://ghcr.io/fenleap/<image>:<tag> --owner Fenleap
```

Upstream binaries are pinned by build argument and recorded in the image labels.

## Supported versions

The most recent tag of each image receives fixes. Older tags stay available for
reproducibility but are not rebuilt.
