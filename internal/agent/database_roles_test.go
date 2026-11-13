package agent

import "testing"

func TestParsePostgresRoles(t *testing.T) {
	roles, err := parsePostgresRoles(`[
		{"name":"app_user","can_login":true,"is_superuser":false,"is_replication":false,"bypass_rls":false,"inherit":true,"connection_limit":12},
		{"name":"admin_role","can_login":true,"is_superuser":true,"is_replication":false,"bypass_rls":true,"inherit":true,"connection_limit":-1}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("got %d roles, want 2", len(roles))
	}
	if roles[0].Name != "app_user" || !roles[0].CanLogin || roles[0].IsSuperuser || roles[0].ConnectionLimit != 12 {
		t.Fatalf("unexpected application role: %#v", roles[0])
	}
	if roles[1].Name != "admin_role" || !roles[1].IsSuperuser || !roles[1].BypassRLS || roles[1].ConnectionLimit != -1 {
		t.Fatalf("unexpected privileged role: %#v", roles[1])
	}
}

func TestParsePostgresRolesEmpty(t *testing.T) {
	roles, err := parsePostgresRoles("[]")
	if err != nil {
		t.Fatal(err)
	}
	if roles == nil || len(roles) != 0 {
		t.Fatalf("expected an empty non-nil role list, got %#v", roles)
	}
}

func TestParsePostgresRolesRejectsInvalidJSON(t *testing.T) {
	if _, err := parsePostgresRoles("not-json"); err == nil {
		t.Fatal("expected invalid PostgreSQL role output to be rejected")
	}
}
