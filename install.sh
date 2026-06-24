#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
install_dir="${1:-${CF_INSTALL_DIR:-$HOME/.local/bin}}"
binary_name="${CF_BINARY_NAME:-cf}"
tmp_dir="$(mktemp -d)"
cleanup() {
  /bin/rm -rf "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$install_dir"

echo "Building ${binary_name} from ${repo_root}..."
(
  cd "$repo_root"
  go build -o "$tmp_dir/$binary_name" .
)

install -m 0755 "$tmp_dir/$binary_name" "$install_dir/$binary_name"

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
