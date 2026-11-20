package control

import (
	"strings"
	"testing"
)

func TestMySQLClientDefaultsFileUsesRealLines(t *testing.T) {
	got := mysqlClientDefaultsFile("yevqo", "safe-pass", "127.0.0.1", "3306")
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	want := []string{
		"[client]",
		"user=\"yevqo\"",
		"password=\"safe-pass\"",
		"host=\"127.0.0.1\"",
		"port=\"3306\"",
		"protocol=tcp",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %#v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i+1, lines[i], want[i])
		}
	}
}

func TestCNFEscapeProtectsQuotesAndBackslashes(t *testing.T) {
	got := cnfEscape(`p\"ass`)
	want := `"p\\\"ass"`
	if got != want {
		t.Fatalf("cnfEscape() = %q, want %q", got, want)
	}
}
