#!/bin/sh
# kdbx-secrets bootstrap (Linux and macOS).
#   curl -fsSL https://raw.githubusercontent.com/costaluu/kdbx-secrets/main/install.sh | sh
# Variables: KDBX_SECRETS_BIN_DIR (destination), KDBX_SECRETS_BASE_URL (tests only).
set -eu

REPO="costaluu/kdbx-secrets"
BASE="${KDBX_SECRETS_BASE_URL:-https://github.com/$REPO/releases/latest/download}"
BIN_DIR="${KDBX_SECRETS_BIN_DIR:-$HOME/.local/bin}"

die() { echo "install.sh: $*" >&2; exit 1; }

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) die "unsupported system: $(uname -s) (on Windows use install.ps1)" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac
asset="kdbx-secrets-$os-$arch"

tmp=$(mktemp -d) || die "mktemp failed"
trap 'rm -rf "$tmp"' EXIT INT TERM

fetch() { curl -fsSL -o "$tmp/$1" "$BASE/$1" || die "download failed: $BASE/$1"; }
fetch "$asset"
fetch SHA256SUMS

want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$want" ] || die "$asset is not listed in SHA256SUMS"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
else
	got=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
fi
[ "$got" = "$want" ] || die "SHA-256 mismatch (expected $want, got $got); nothing was installed"
echo "SHA-256 ok"

if command -v gh >/dev/null 2>&1; then
	gh attestation verify "$tmp/$asset" --repo "$REPO" || die "invalid attestation; nothing was installed"
else
	echo "gh not found; to verify provenance by hand:" >&2
	echo "  gh attestation verify $asset --repo $REPO" >&2
fi

mkdir -p "$BIN_DIR"
chmod 755 "$tmp/$asset"
# temp file in the destination + rename: the install is atomic
dest_tmp="$BIN_DIR/.kdbx-secrets.$$"
cp "$tmp/$asset" "$dest_tmp" && mv -f "$dest_tmp" "$BIN_DIR/kdbx-secrets" || { rm -f "$dest_tmp"; die "failed to install into $BIN_DIR"; }
echo "installed to $BIN_DIR/kdbx-secrets"

case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*) echo "warning: $BIN_DIR is not in PATH" >&2 ;;
esac
