#!/bin/sh

set -eu

REPO_SLUG="LostWarrior/nivi"
INSTALL_DIR="${NIVI_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${NIVI_VERSION:-latest}"
PROJECT_NAME="nivi"

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

checksum_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf 'sha256sum'
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    printf 'shasum'
    return
  fi

  fail "missing checksum tool: install sha256sum or shasum"
}

release_base_url() {
  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download' "$REPO_SLUG"
    return
  fi

  printf 'https://github.com/%s/releases/download/%s' "$REPO_SLUG" "$VERSION"
}

verify_checksum() {
  checksum_file="$1"
  archive_name="$2"
  archive_path="$3"
  tool="$(checksum_cmd)"

  expected_sum="$(awk -v archive_name="$archive_name" '$2 == archive_name { print $1; exit }' "$checksum_file")"
  [ -n "$expected_sum" ] || fail "checksum entry for ${archive_name} not found"

  case "$tool" in
    sha256sum)
      actual_sum="$(sha256sum "$archive_path" | awk '{print $1}')"
      ;;
    shasum)
      actual_sum="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
      ;;
    *)
      fail "unsupported checksum tool: $tool"
      ;;
  esac

  [ "$expected_sum" = "$actual_sum" ] || fail "checksum verification failed for ${archive_name}"
}

path_contains() {
  case ":$PATH:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

main() {
  require_cmd curl
  require_cmd install
  require_cmd tar
  require_cmd awk

  os="$(detect_os)"
  arch="$(detect_arch)"
  archive_name="${PROJECT_NAME}_${os}_${arch}.tar.gz"
  checksum_name="${PROJECT_NAME}_checksums.txt"
  base_url="$(release_base_url)"
  archive_url="${base_url}/${archive_name}"
  checksum_url="${base_url}/${checksum_name}"

  tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t nivi-install)"
  trap 'rm -rf "$tmpdir"' EXIT INT TERM

  archive_path="$tmpdir/$archive_name"
  checksum_path="$tmpdir/$checksum_name"
  extract_dir="$tmpdir/extract"

  log "downloading ${REPO_SLUG} ${VERSION} for ${os}/${arch}"
  curl -fsSL "$archive_url" -o "$archive_path"
  curl -fsSL "$checksum_url" -o "$checksum_path"

  verify_checksum "$checksum_path" "$archive_name" "$archive_path"

  mkdir -p "$extract_dir"
  tar -xzf "$archive_path" -C "$extract_dir"
  [ -f "$extract_dir/$PROJECT_NAME" ] || fail "archive did not contain ${PROJECT_NAME}"

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$extract_dir/$PROJECT_NAME" "$INSTALL_DIR/$PROJECT_NAME"

  log "installed ${PROJECT_NAME} to $INSTALL_DIR/$PROJECT_NAME"
  if ! path_contains "$INSTALL_DIR"; then
    log "warning: $INSTALL_DIR is not on PATH"
    log "add this to your shell profile: export PATH=\"$INSTALL_DIR:\$PATH\""
  fi
  log 'set NVIDIA_API_KEY, then run: nivi'
}

main "$@"
