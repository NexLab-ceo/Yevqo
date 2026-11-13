package agent

import "testing"

func TestPostgresSQLLiteralEscapesQuotes(t *testing.T) {
	got := postgresSQLLiteral("p'ass\\word")
	if got != "'p''ass\\word'" {
		t.Fatalf("unexpected SQL literal: %q", got)
	}
}

func TestPostgresRoleNameValidation(t *testing.T) {
	for _, name := range []string{"app_user", "A1", "role_63"} {
		if !postgresRoleName.MatchString(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	for _, name := range []string{"", "1user", "user-name", "user name", "a"+string(make([]byte, 63))} {
		if postgresRoleName.MatchString(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
