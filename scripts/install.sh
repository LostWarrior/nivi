#!/bin/sh

set -eu

REPO_SLUG="LostWarrior/nivi"
INSTALL_DIR="${NIVI_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${NIVI_VERSION:-latest}"

log() {
  printf '%s\n' "$*" >&2
}

fail() {
  log "error: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

main() {
  require_cmd curl
  require_cmd install

  os="$(detect_os)"
  arch="$(detect_arch)"

  # Maintainer wiring notes:
  # - This script is intended to back a one-line installer once public releases exist.
  # - Keep release URL resolution and asset naming here, not in the README.
  # - Do not hardcode unpublished asset URLs or naming conventions.
  # - This scaffold currently assumes the resolved asset is a raw executable.
  #   If releases ship archives instead, unpack them here once the format is finalized.
  if [ -z "${NIVI_DOWNLOAD_URL:-}" ]; then
    fail "installer scaffold is not wired to a published release asset yet for ${REPO_SLUG} ${VERSION} (${os}/${arch}); set NIVI_DOWNLOAD_URL after release assets exist"
  fi

  tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t nivi-install)"
  trap 'rm -rf "$tmpdir"' EXIT INT TERM

  target="$tmpdir/nivi"
  log "downloading ${REPO_SLUG} ${VERSION} for ${os}/${arch}"
  curl -fsSL "$NIVI_DOWNLOAD_URL" -o "$target"

  chmod +x "$target"
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$target" "$INSTALL_DIR/nivi"

  log "installed nivi to $INSTALL_DIR/nivi"
  log 'set NVIDIA_API_KEY, then run: nivi'
}

main "$@"
