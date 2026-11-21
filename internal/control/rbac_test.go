package control

import "testing"

func TestCanWorkspaceActionRoleMatrix(t *testing.T) {
	cases := []struct{ role, action string; want bool }{
		{"Owner","device.delete",true},
		{"Admin","device.delete",true},
		{"Developer","project.read",true},
		{"Developer","environment.read",true},
		{"Developer","environment.assign",true},
		{"Developer","project.manage",false},
		{"Developer","device.read",true},
		{"Developer","get_device_diagnostics",true},
		{"Developer","check_device_port",true},
		{"Developer","restart_agent",false},
		{"Developer","update_agent",false},
		{"Developer","deploy",true},
		{"Developer","stop_site",true},
		{"Developer","get_site_restart_policy",true},
		{"Developer","set_site_restart_policy",false},
		{"Developer","stop",true},
		{"Developer","rollback",true},
		{"Developer","site.configure",false},
		{"Developer","device.delete",false},
		{"Developer","database.read",false},
		{"Developer","database.manage",false},
		{"Admin","database.read",true},
		{"Admin","database.manage",true},
		{"Developer","workspace.manage",false},
		{"Developer","project.manage",false},
		{"Admin","project.manage",true},
		{"Admin","workspace.manage",true},
		{"Developer","manage_members",false},
		{"viewer","project.read",true},
		{"viewer","environment.read",true},
		{"viewer","device.read",true},
		{"viewer","database.read",true},
		{"viewer","project.manage",false},
		{"viewer","deploy",false},
		{"viewer","device.delete",false},
		{"","deploy",false},
	}
	for _, tc := range cases {
		if got:=CanWorkspaceAction(tc.role,tc.action);got!=tc.want {
			t.Errorf("CanWorkspaceAction(%q,%q)=%v, want %v",tc.role,tc.action,got,tc.want)
		}
	}
}
