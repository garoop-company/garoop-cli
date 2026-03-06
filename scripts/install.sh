#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${REPO_OWNER:-yamashitadaiki}"
REPO_NAME="${REPO_NAME:-garoop-cli}"
GITHUB_API="${GITHUB_API:-https://api.github.com}"
GITHUB_DL_BASE="${GITHUB_DL_BASE:-https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download}"

DEFAULT_BINARIES=("garoop-cli")
ALL_BINARIES=("garoop-cli" "garuchan-cli" "garooptv-cli")

usage() {
  cat <<'EOF'
Usage:
  install.sh [options]

Options:
  -b, --binary <name>      Binary name. One of: garoop-cli, garuchan-cli, garooptv-cli, all
  -d, --dir <path>         Install directory (default: ~/.local/bin or Termux bin)
  -v, --version <tag>      Release tag (default: latest)
  -u, --uninstall          Uninstall selected binary/binaries from install directory
  -f, --force              Overwrite existing binary without prompt
  -h, --help               Show help

Examples:
  ./scripts/install.sh
  ./scripts/install.sh --binary all
  ./scripts/install.sh --binary garuchan-cli --dir /usr/local/bin
  ./scripts/install.sh --version v0.2.0
  ./scripts/install.sh --uninstall --binary all
EOF
}

log() {
  printf '[install] %s\n' "$*"
}

err() {
  printf '[install][error] %s\n' "$*" >&2
}

is_termux() {
  [[ -n "${TERMUX_VERSION:-}" ]] || [[ -d "/data/data/com.termux/files/usr/bin" ]]
}

default_install_dir() {
  if is_termux; then
    printf '/data/data/com.termux/files/usr/bin'
    return
  fi
  printf '%s/.local/bin' "${HOME}"
}

detect_os() {
  local u
  u="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$u" in
    linux*) printf 'linux' ;;
    darwin*) printf 'darwin' ;;
    android*) printf 'android' ;;
    *) printf '%s' "$u" ;;
  esac
}

detect_arch() {
  local a
  a="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$a" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *) printf '%s' "$a" ;;
  esac
}

resolve_platform() {
  local os arch
  os="$(detect_os)"
  arch="$(detect_arch)"

  if is_termux; then
    os='android'
    arch='arm64'
  fi
  printf '%s %s\n' "$os" "$arch"
}

fetch_latest_tag() {
  curl -fsSL "${GITHUB_API}/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n1
}

download_asset() {
  local tag="$1"
  local bin="$2"
  local os="$3"
  local arch="$4"
  local ext="$5"
  local tmp="$6"

  local asset="${bin}_${tag#v}_${os}_${arch}.${ext}"
  local url="${GITHUB_DL_BASE}/${tag}/${asset}"
  log "downloading ${asset}"
  if ! curl -fL "$url" -o "$tmp"; then
    err "failed to download: ${url}"
    err "release assets naming may differ. check: https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/tag/${tag}"
    return 1
  fi
}

extract_binary() {
  local archive="$1"
  local ext="$2"
  local bin="$3"
  local out_dir="$4"

  local work
  work="$(mktemp -d)"
  trap 'rm -rf "$work"' RETURN

  case "$ext" in
    tar.gz) tar -xzf "$archive" -C "$work" ;;
    zip) unzip -q "$archive" -d "$work" ;;
    *) err "unsupported archive: ${ext}"; return 1 ;;
  esac

  local found=""
  if [[ -x "${work}/${bin}" ]]; then
    found="${work}/${bin}"
  else
    found="$(find "$work" -type f -name "$bin" | head -n1 || true)"
  fi
  if [[ -z "$found" ]]; then
    err "binary not found in archive: ${bin}"
    return 1
  fi

  mkdir -p "$out_dir"
  if [[ -e "${out_dir}/${bin}" && "${FORCE_INSTALL:-0}" != "1" ]]; then
    err "already exists: ${out_dir}/${bin}"
    err "use --force to overwrite"
    return 1
  fi
  install -m 0755 "$found" "${out_dir}/${bin}"
  log "installed: ${out_dir}/${bin}"
}

validate_binary() {
  local b="$1"
  if [[ "$b" == "all" ]]; then
    return 0
  fi
  local x
  for x in "${ALL_BINARIES[@]}"; do
    if [[ "$x" == "$b" ]]; then
      return 0
    fi
  done
  return 1
}

main() {
  local binary="garoop-cli"
  local version="latest"
  local uninstall_mode="0"
  FORCE_INSTALL="0"
  local install_dir
  install_dir="$(default_install_dir)"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -b|--binary)
        binary="${2:-}"
        shift 2
        ;;
      -d|--dir)
        install_dir="${2:-}"
        shift 2
        ;;
      -v|--version)
        version="${2:-}"
        shift 2
        ;;
      -u|--uninstall)
        uninstall_mode="1"
        shift
        ;;
      -f|--force)
        FORCE_INSTALL="1"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        err "unknown argument: $1"
        usage
        exit 1
        ;;
    esac
  done

  if ! validate_binary "$binary"; then
    err "invalid --binary: ${binary}"
    usage
    exit 1
  fi

  local tag
  if [[ "$version" == "latest" ]]; then
    tag="$(fetch_latest_tag)"
    if [[ -z "$tag" ]]; then
      err "could not fetch latest tag"
      exit 1
    fi
  else
    tag="$version"
  fi

  local os arch
  read -r os arch < <(resolve_platform)
  log "platform: ${os}/${arch}"
  if [[ "$uninstall_mode" != "1" ]]; then
    log "tag: ${tag}"
  fi

  local bins=()
  if [[ "$binary" == "all" ]]; then
    bins=("${ALL_BINARIES[@]}")
  else
    bins=("$binary")
  fi

  local ext="tar.gz"
  if [[ "$os" == "windows" ]]; then
    ext="zip"
  fi

  if [[ "$uninstall_mode" == "1" ]]; then
    local removed=0
    local b
    for b in "${bins[@]}"; do
      local target="${install_dir}/${b}"
      if [[ -e "$target" ]]; then
        rm -f "$target"
        log "removed: ${target}"
        removed=1
      else
        log "not found: ${target}"
      fi
    done
    if [[ "$removed" == "0" ]]; then
      log "nothing to uninstall"
    fi
    log "done"
    exit 0
  fi

  local b
  for b in "${bins[@]}"; do
    local tmp
    tmp="$(mktemp "/tmp/${b}.XXXXXX.${ext}")"
    download_asset "$tag" "$b" "$os" "$arch" "$ext" "$tmp"
    extract_binary "$tmp" "$ext" "$b" "$install_dir"
    rm -f "$tmp"
  done

  log "done"
  log "if command not found, add to PATH: ${install_dir}"
}

main "$@"
