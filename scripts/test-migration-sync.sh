#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SYNC="$ROOT/packaging/sync-migrations.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
SOURCE="$TMP/package"
DEST="$TMP/data/migrations"
mkdir -p "$SOURCE" "$DEST"
printf '%s\n' 'CREATE TABLE first_table (id INT);' > "$SOURCE/021_project_environments.sql"
sh "$SYNC" "$SOURCE" "$DEST"
cmp "$SOURCE/021_project_environments.sql" "$DEST/021_project_environments.sql"
[ "$(stat -c '%a' "$DEST/021_project_environments.sql")" = "600" ]
sh "$SYNC" "$SOURCE" "$DEST"
printf '%s\n' 'CREATE TABLE second_table (id INT);' > "$SOURCE/022_project_environment_databases.sql"
sh "$SYNC" "$SOURCE" "$DEST"
cmp "$SOURCE/022_project_environment_databases.sql" "$DEST/022_project_environment_databases.sql"
printf '%s\n' 'ALTER TABLE first_table ADD COLUMN changed INT;' > "$SOURCE/021_project_environments.sql"
if sh "$SYNC" "$SOURCE" "$DEST"; then
  echo "expected conflicting historical migration to fail" >&2
  exit 1
fi
grep -q 'CREATE TABLE first_table' "$DEST/021_project_environments.sql"
ln -s "$TMP/data" "$TMP/link"
if sh "$SYNC" "$SOURCE" "$TMP/link"; then
  echo "expected symlink migration destination to fail" >&2
  exit 1
fi
echo "Migration sync tests passed."
