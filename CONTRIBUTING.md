# Contributing

## Adding an image

1. Create `images/<name>/Dockerfile`. The build context is the **repository
   root**, so shared pieces are available at `shared/launcher/` and
   `shared/scripts/`.
2. Add `images/<name>/README.md` covering: what the image is, what it removes,
   configuration, and a **"what this does not fix"** section. That last part is
   not optional — an image that oversells itself is worse than no image.
3. Add `images/<name>/NOTICE.md` if it redistributes upstream binaries, naming
   each licence and any obligation that comes with it.
4. Run `make test IMAGE=<name>`.

The Makefile and both workflows discover images from `images/*/Dockerfile`, so
there is nothing else to register.

## House rules

- **No shell, no package manager** in a final image, unless the image's README
  explains why and the smoke test is adjusted deliberately.
- **Non-root by default** (UID 65532), working under `--read-only` with
  `--cap-drop ALL`.
- **Pin upstream versions** with build arguments, and surface them in the OCI
  labels so `imagetools inspect` answers "what is in here".
- **Resolve libraries with `ldd`**, never a hardcoded list. Hardcoded lists miss
  privately bundled copies and fail only at runtime.
- **Prefer a base with no libc** when the payload brings its own. Two libcs in
  one filesystem means the loader and libc can resolve from different
  distributions, which is unsupported and fails obscurely.

## Testing

```
make lint                    # gofmt + go vet + launcher unit tests
make test IMAGE=<name>       # build, then assert the claims on the artifact
make test-all
```

CI runs the same targets. A change that reintroduces a shell fails there.

## Releasing

Images version independently. Tag `<image>-v<semver>`:

```
git tag -a mysql-no-shell-v1.0.0 -m "mysql-no-shell 1.0.0"
git push origin mysql-no-shell-v1.0.0
```
