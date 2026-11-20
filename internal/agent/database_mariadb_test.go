package agent

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestMariaDBDataDirectoryMustRemainInsideTermux(t *testing.T) {
	prefix := t.TempDir()
	got, err := mariaDataDirectory(prefix, 4, "$PREFIX/var/lib/yevqo/mariadb/server-4")
	if err != nil { t.Fatal(err) }
	if got != filepath.Join(prefix, "var", "lib", "yevqo", "mariadb", "server-4") { t.Fatalf("unexpected path: %s", got) }
	if _, err := mariaDataDirectory(prefix, 4, "/tmp/outside"); err == nil { t.Fatal("expected path outside Termux PREFIX to be rejected") }
	if _, err := mariaDataDirectory(prefix, 4, ""); err == nil { t.Fatal("expected empty path to be rejected") }
}

func TestRewriteMariaDBDumpTargetsNewDatabase(t *testing.T) {
	input := "-- MariaDB dump\nUSE `old_database`;\nCREATE TABLE sample(id INT);\nINSERT INTO sample VALUES (1);\n"
	var output bytes.Buffer
	if err := rewriteMariaDBDump(strings.NewReader(input), &output, "restored_database"); err != nil { t.Fatal(err) }
	got := output.String()
	if strings.Contains(got, "old_database") { t.Fatalf("dump still references source database: %s", got) }
	if !strings.Contains(got, "USE `restored_database`;") { t.Fatalf("target USE statement missing: %s", got) }
	if !strings.Contains(got, "INSERT INTO sample") { t.Fatalf("dump data was not preserved: %s", got) }
}

func TestRewriteMariaDBDumpRejectsInvalidTarget(t *testing.T) {
	if err := rewriteMariaDBDump(strings.NewReader("USE old;"), &bytes.Buffer{}, "../outside"); err == nil { t.Fatal("expected invalid target to be rejected") }
}

func TestMariaDBBackupFilenamePattern(t *testing.T) {
	if !mariaBackupFilePattern.MatchString("app_db-12-20261120T194300.123456Z.sql") { t.Fatal("valid MariaDB backup name rejected") }
	for _, name := range []string{"../app_db-12-20261120T194300.123456Z.sql", "app_db-12-20261120T194300.123456Z.dump", "app_db.sql"} {
		if mariaBackupFilePattern.MatchString(name) { t.Fatalf("unsafe or unsupported backup name accepted: %s", name) }
	}
}
