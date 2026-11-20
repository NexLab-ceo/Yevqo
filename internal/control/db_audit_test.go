package control

import "testing"

func TestAuditOutcome(t *testing.T) {
	tests := []struct {
		name string
		action string
		details map[string]interface{}
		want string
	}{
		{name:"successful deploy",action:"site_deploy",want:"success"},
		{name:"failed deploy action",action:"site_deploy_failed",want:"failure"},
		{name:"failed result flag",action:"database_restore",details:map[string]interface{}{"ok":false},want:"failure"},
		{name:"permission denied",action:"workspace_access_denied",want:"failure"},
		{name:"nil details",action:"project_created",want:"success"},
	}
	for _,tc:=range tests {
		t.Run(tc.name,func(t *testing.T){
			if got:=auditOutcome(tc.action,tc.details);got!=tc.want{t.Fatalf("auditOutcome(%q)=%q, want %q",tc.action,got,tc.want)}
		})
	}
}
