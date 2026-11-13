package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitSQLRespectsQuotedSemicolonsAndComments(t *testing.T) {
	script := `-- leading comment
CREATE TABLE sample (value VARCHAR(32) DEFAULT 'a;b');
/* middle comment; ignored */
INSERT INTO sample(value) VALUES ("x;y"); -- trailing comment
`

	got := splitSQL(script)
	if len(got) != 2 {
		t.Fatalf("splitSQL returned %d statements, want 2: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "'a;b'") || !strings.Contains(got[1], "\"x;y\"") {
		t.Fatalf("quoted semicolons were not preserved: %#v", got)
	}
}

func TestCoreMigrationIsAdditive(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000_core.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read core migration: %v", err)
	}

	statements := splitSQL(string(content))
	if len(statements) != 5 {
		t.Fatalf("core migration has %d statements, want 5", len(statements))
	}

	for i, statement := range statements {
		normalized := strings.ToLower(strings.TrimSpace(statement))
		if !strings.HasPrefix(normalized, "create table if not exists ") {
			t.Errorf("statement %d is not additive CREATE TABLE IF NOT EXISTS: %q", i+1, statement)
		}
		for _, forbidden := range []string{"drop table", "truncate table", "delete from"} {
			if strings.Contains(normalized, forbidden) {
				t.Errorf("statement %d contains destructive SQL %q", i+1, forbidden)
			}
		}
	}
}
