# 16: Release workflow

**What to build:** The maintainer publishes with `git tag vX.Y.Z && git push --tags`. The workflow triggered by the tag runs the tests and, only if they pass, builds with `CGO_ENABLED=0`, `-trimpath` and the tag's version injected for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64. Names follow `kdbx-secrets-<goos>-<goarch>[.exe]`. It generates `SHA256SUMS`, creates a provenance attestation per binary (`actions/attest-build-provenance`) and publishes everything as release assets. It lands when slice 2 (`render` and `adapter`) is usable, per the spec.

**Blocked by:** 09, 10

**Status:** done

- [ ] The workflow triggers only on `v*.*.*` tags
- [ ] The publishing job depends on the tests job
- [ ] The 5 binaries have the pattern's names, and `version` prints the tag
- [ ] `SHA256SUMS` covers all binaries
- [ ] Each binary has an attestation (with the workflow's `id-token` and `attestations` permissions)
- [ ] Validated locally with `actionlint`, if available; the first real run is recorded as verification
