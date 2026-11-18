# 17: Bootstrap, post-release smoke and README

**What to build:** A new user installs with one line. `install.sh` (POSIX, Linux and macOS) and `install.ps1` (Windows) live in the repository and are downloaded through the raw URL. They detect the OS and architecture and download the asset and the `SHA256SUMS` through `releases/latest/download`, without an API and without JSON. They check the SHA-256 and abort on mismatch. If `gh` exists, they run `gh attestation verify --repo <repo>`; if not, they print the command to verify by hand. They install through a temporary file and `rename` into `~/.local/bin` or `%LOCALAPPDATA%\kdbx-secrets\bin`, with an override via `KDBX_SECRETS_BIN_DIR`, and warn if the folder is not in `PATH`. They do not create the configuration. They follow the taskthing model. A smoke job, after the release is published, runs the scripts on ubuntu, macos and windows and executes `kdbx-secrets version`. The README explains the installation, the manual verification of the binary, the configuration of the first Database, the `paper-freeze` case and the known hardening limits (with the BitLocker recommendation).

**Blocked by:** 16

**Status:** done

- [ ] `install.sh` passes `shellcheck` and runs on POSIX `sh`
- [ ] A divergent checksum aborts without installing anything
- [ ] With `gh`, the attestation is verified; without `gh`, the manual command is printed
- [ ] The installation is atomic, and the `PATH` warning shows up when applicable
- [ ] The smoke job runs after the release on the three OSes and checks `version`
- [ ] The README covers the installation, the manual verification, the first Database, `paper-freeze` and the hardening limits
