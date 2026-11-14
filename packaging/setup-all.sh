#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
printf '\nVELTRIX Dock — choose what to configure\n'
printf '  1) Control Plane (server)\n'
printf '  2) Agent (connect this phone to a panel)\n'
printf '  3) Both (master/developer phone)\n'
read -r -p 'Selection [3]: ' choice
choice="${choice:-3}"
case "$choice" in
  1) "$PREFIX/bin/veltrix-setup-control" ;;
  2) "$PREFIX/bin/veltrix-setup-agent" ;;
  3) "$PREFIX/bin/veltrix-setup-control"; "$PREFIX/bin/veltrix-setup-agent" ;;
  *) echo "Invalid selection" >&2; exit 2 ;;
esac
