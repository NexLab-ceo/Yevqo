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

func TestProjectEnvironmentDatabaseMigrationIsAdditive(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "022_project_environment_databases.sql")
	content, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read project environment database migration: %v", err) }
	statements := splitSQL(string(content))
	if len(statements) != 1 { t.Fatalf("migration has %d statements, want 1", len(statements)) }
	normalized := strings.ToLower(strings.TrimSpace(statements[0]))
	if !strings.HasPrefix(normalized, "create table if not exists project_environment_database_servers ") {
		t.Fatalf("migration must create the environment database assignment table additively: %q", statements[0])
	}
	for _, required := range []string{"primary key", "foreign key (project_id)", "foreign key (environment_id)", "foreign key (database_server_id)", "on delete cascade"} {
		if !strings.Contains(normalized, required) { t.Errorf("migration is missing %q", required) }
	}
	for _, forbidden := range []string{"drop table", "truncate table", "delete from", "drop column", "alter table"} {
		if strings.Contains(normalized, forbidden) { t.Errorf("migration contains non-additive SQL %q", forbidden) }
	}
}

func TestP2MigrationIsAdditiveAndContainsPlatformTables(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "020_p2_platform_foundations.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read P2 migration: %v", err)
	}
	statements := splitSQL(string(content))
	wantTables := []string{
		"preview_deployments",
		"build_executors",
		"build_jobs",
		"app_templates",
		"workspace_members",
		"audit_logs",
	}
	if len(statements) != len(wantTables) {
		t.Fatalf("P2 migration has %d statements, want %d", len(statements), len(wantTables))
	}
	for i, table := range wantTables {
		normalized := strings.ToLower(strings.TrimSpace(statements[i]))
		if !strings.HasPrefix(normalized, "create table if not exists "+table+" ") {
			t.Errorf("statement %d must create %s additively, got %q", i+1, table, statements[i])
		}
		for _, forbidden := range []string{"drop table", "truncate table", "delete from", "drop column"} {
			if strings.Contains(normalized, forbidden) {
				t.Errorf("statement %d contains destructive SQL %q", i+1, forbidden)
			}
		}
	}
}

func TestProjectEnvironmentDatabaseInstancesMigrationIsAdditiveAndScoped(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "024_project_environment_database_instances.sql")
	content, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read project environment database instances migration: %v", err) }
	statements := splitSQL(string(content))
	if len(statements) != 3 { t.Fatalf("migration has %d statements, want 3", len(statements)) }
	normalized := strings.ToLower(strings.Join(statements, "\n"))
	for _, required := range []string{"alter table project_environments","add unique key uq_project_environments_project_id_id (project_id, id)","create table if not exists project_environment_databases","primary key (project_id, database_id)","foreign key (project_id, environment_id)","references project_environments (project_id, id)","foreign key (database_id)","having count(distinct eds.environment_id) = 1"} {
		if !strings.Contains(normalized, required) { t.Errorf("migration is missing %q", required) }
	}
	for _, forbidden := range []string{"drop table","truncate table","delete from","drop column"} { if strings.Contains(normalized, forbidden) { t.Errorf("migration contains destructive SQL %q", forbidden) } }
}
