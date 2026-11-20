package control

import "testing"

func TestParseReportPeriodDays(t *testing.T) {
	for _, tc := range []struct{ raw string; want int; wantError bool }{
		{"",30,false},{"7",7,false},{"30",30,false},{"90",90,false},{"14",0,true},{"0",0,true},{"abc",0,true},
	} {
		got,err:=parseReportPeriodDays(tc.raw)
		if (err!=nil)!=tc.wantError || got!=tc.want { t.Errorf("parseReportPeriodDays(%q)=(%d,%v), want (%d,error=%v)",tc.raw,got,err,tc.want,tc.wantError) }
	}
}
