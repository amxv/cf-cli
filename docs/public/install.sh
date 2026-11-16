#!/usr/bin/env bash
set -euo pipefail

repo="amxv/cf-cli"
binary_name="${CF_BINARY_NAME:-cf}"
install_dir="${1:-${CF_INSTALL_DIR:-$HOME/.local/bin}}"
os_override="${CF_INSTALL_OS:-}"
arch_override="${CF_INSTALL_ARCH:-}"
tmp_dir="$(mktemp -d)"
cleanup() {
  /bin/rm -rf "$tmp_dir"
}
trap cleanup EXIT

detect_os() {
  if [ -n "$os_override" ]; then
    printf '%s\n' "$os_override"
    return
  fi

  case "$(uname -s)" in
    Linux)
      printf 'linux\n'
      ;;
    Darwin)
      printf 'darwin\n'
      ;;
    *)
      return 1
      ;;
  esac
}

detect_arch() {
  if [ -n "$arch_override" ]; then
    printf '%s\n' "$arch_override"
    return
  fi

  case "$(uname -m)" in
    x86_64|amd64)
      printf 'amd64\n'
      ;;
    arm64|aarch64)
      printf 'arm64\n'
      ;;
    *)
      return 1
      ;;
  esac
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command '$1' is not available." >&2
    exit 1
  fi
}

require_cmd curl
require_cmd install

if ! os="$(detect_os)"; then
  echo "Error: unsupported operating system: $(uname -s)" >&2
  echo "Supported platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64." >&2
  exit 1
fi

if ! arch="$(detect_arch)"; then
  echo "Error: unsupported architecture: $(uname -m)" >&2
  echo "Supported platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64." >&2
  exit 1
fi

asset_name="${binary_name}-${os}-${arch}"
download_url="https://github.com/${repo}/releases/latest/download/${asset_name}"
target_path="$tmp_dir/$binary_name"

mkdir -p "$install_dir"

echo "Downloading ${binary_name} for ${os}/${arch} from ${download_url}..."
if ! curl --fail --location --silent --show-error --output "$target_path" "$download_url"; then
  echo "Error: failed to download release asset '${asset_name}' from ${repo}." >&2
  echo "The platform may be unsupported or the latest release may not include this asset yet." >&2
  exit 1
fi

install -m 0755 "$target_path" "$install_dir/$binary_name"

echo "Installed $binary_name to $install_dir/$binary_name"

case ":$PATH:" in
  *":$install_dir:"*)
    ;;
  *)
    echo "Note: $install_dir is not currently on PATH."
    echo "Add this to your shell profile if needed:"
    echo "  export PATH=\"$install_dir:\$PATH\""
    ;;
esac
