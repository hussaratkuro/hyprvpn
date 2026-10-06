#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d)"
trap 'rm -rf -- "$build_dir"' EXIT

cd "$project_dir"
go build -trimpath -tags novulkan -o "$build_dir/hyprvpn" .
install -Dm755 "$build_dir/hyprvpn" "$HOME/.local/bin/hyprvpn"
install -Dm644 hyprvpn.png "$HOME/.local/share/icons/hyprvpn/hyprvpn.png"
sed "s|__HOME__|$HOME|g" hyprvpn.desktop > "$build_dir/hyprvpn.desktop"
install -Dm644 "$build_dir/hyprvpn.desktop" "$HOME/.local/share/applications/hyprvpn.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$HOME/.local/share/applications"
fi

echo "Installed hyprvpn in $HOME/.local/bin"
