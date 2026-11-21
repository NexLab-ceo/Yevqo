#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
# Package upgrades or data-root migrations can invalidate the caller\x27s cwd.
cd "$HOME" 2>/dev/null || cd "$PREFIX"
printf '\nYevqo — choose what to configure\n'
printf '  1) Control Plane (server)\n'
printf '  2) Agent (connect this phone to a panel)\n'
printf '  3) Both (master/developer phone)\n'
read -r -p 'Selection [3]: ' choice
choice="${choice:-3}"
case "$choice" in
  1) "$PREFIX/bin/yevqo-setup-control" ;;
  2) "$PREFIX/bin/yevqo-setup-agent" ;;
  3) "$PREFIX/bin/yevqo-setup-control"; "$PREFIX/bin/yevqo-setup-agent" ;;
  *) echo "Invalid selection" >&2; exit 2 ;;
esac
