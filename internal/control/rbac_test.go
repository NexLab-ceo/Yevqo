package control

import "testing"

func TestCanWorkspaceActionRoleMatrix(t *testing.T) {
	cases := []struct{ role, action string; want bool }{
		{"Owner","device.delete",true},
		{"Admin","device.delete",true},
		{"Developer","project.read",true},
		{"Developer","environment.read",true},
		{"Developer","project.manage",false},
		{"Developer","device.read",true},
		{"Developer","deploy",true},
		{"Developer","stop_site",true},
		{"Developer","stop",true},
		{"Developer","rollback",true},
		{"Developer","site.configure",false},
		{"Developer","device.delete",false},
		{"Developer","database.read",false},
		{"Developer","database.manage",false},
		{"Admin","database.read",true},
		{"Admin","database.manage",true},
		{"Developer","workspace.manage",false},
		{"Admin","workspace.manage",true},
		{"Developer","manage_members",false},
		{"viewer","device.read",false},
		{"","deploy",false},
	}
	for _, tc := range cases {
		if got:=CanWorkspaceAction(tc.role,tc.action);got!=tc.want {
			t.Errorf("CanWorkspaceAction(%q,%q)=%v, want %v",tc.role,tc.action,got,tc.want)
		}
	}
}
