package control

import "testing"

func TestCSVSafeCellPreventsFormulaInjection(t *testing.T) {
	tests := []struct{ input, want string }{
		{"=1+1", "'=1+1"},
		{"+SUM(A1:A2)", "'+SUM(A1:A2)"},
		{"-cmd", "'-cmd"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\t=1+1", "'\t=1+1"},
		{"  =1+1", "'  =1+1"},
		{"\r@SUM(A1)", "'\r@SUM(A1)"},
		{"  ordinary text", "  ordinary text"},
		{"ordinary text", "ordinary text"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := csvSafeCell(tt.input); got != tt.want {
			t.Errorf("csvSafeCell(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
