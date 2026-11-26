#!/data/data/com.termux/files/usr/bin/sh
set -eu

SOURCE_DIR="${1:?usage: sync-migrations.sh SOURCE_DIR DESTINATION_DIR}"
DEST_DIR="${2:?usage: sync-migrations.sh SOURCE_DIR DESTINATION_DIR}"

[ -d "$SOURCE_DIR" ] || { echo "Packaged migrations directory is missing: $SOURCE_DIR" >&2; exit 1; }
[ ! -L "$DEST_DIR" ] || { echo "Refusing migration destination symlink: $DEST_DIR" >&2; exit 1; }
mkdir -p "$DEST_DIR"
[ -d "$DEST_DIR" ] && [ ! -L "$DEST_DIR" ] || { echo "Unsafe migration destination: $DEST_DIR" >&2; exit 1; }

for source in "$SOURCE_DIR"/*.sql; do
  [ -f "$source" ] || continue
  name="${source##*/}"
  destination="$DEST_DIR/$name"
  if [ -L "$destination" ]; then
    echo "Refusing migration file symlink: $destination" >&2
    exit 1
  fi
  if [ -e "$destination" ]; then
    [ -f "$destination" ] || { echo "Migration destination is not a regular file: $destination" >&2; exit 1; }
    if ! cmp -s "$source" "$destination"; then
      echo "Migration conflict; existing file was preserved: $destination" >&2
      exit 1
    fi
    continue
  fi
  install -m 600 "$source" "$destination"
  echo "Staged migration: $name"
done
