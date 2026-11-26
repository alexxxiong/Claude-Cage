#!/usr/bin/env bash
# Standalone entry point for the Linux release bundle. The desktop client
# invokes the same lifecycle script over SSH.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
case "${1:-}" in
  --help|-h) echo 'Usage: bash install.sh [--verify-only]'; exit 0 ;;
  --verify-only|'') ;;
  *) echo 'Usage: bash install.sh [--verify-only]' >&2; exit 2 ;;
esac
(( $# <= 1 )) || exit 2
digest=$(cat cloud-linux-amd64.tar.sha256)
[[ "$digest" =~ ^[0-9a-f]{64}$ ]] || { echo 'Invalid deployment kit checksum' >&2; exit 1; }
printf '%s  cloud-linux-amd64.tar\n' "$digest" | sha256sum -c -
[[ ${1:-} != --verify-only ]] || exit 0
config=${CLAUDE_CAGE_CONFIG:-"$HOME/.claude-cage/config.json"}
bash ./lifecycle.sh install cage "$config" "$digest" <cloud-linux-amd64.tar
echo "Cage is ready for SSH connections using $config"
