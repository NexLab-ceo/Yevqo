package control

import "testing"

func TestValidProjectEnvironmentScope(t *testing.T) {
	for _, tc := range []struct{ projectID, environmentID int64; want bool }{
		{0,0,true},{1,2,true},{1,0,false},{0,2,false},{-1,2,false},{1,-1,false},
	} {
		if got:=validProjectEnvironmentScope(tc.projectID,tc.environmentID);got!=tc.want {
			t.Errorf("validProjectEnvironmentScope(%d,%d)=%v, want %v",tc.projectID,tc.environmentID,got,tc.want)
		}
	}
}
