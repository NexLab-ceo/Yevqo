package control

import (
	"strings"
	"testing"
)

func TestMySQLClientDefaultsFileUsesRealLines(t *testing.T) {
	got := mysqlClientDefaultsFile("yevqo", "p\\"ass", "127.0.0.1", "3306")
	lines := strings.Split(got, "\n")
	if len(lines) != 6 || lines[0] != "[client]" || lines[1] != "user=\"yevqo\"" ||
		lines[2] != "password=\"p\\\\\\\"ass\"" || lines[3] != "host=\"127.0.0.1\"" ||
		lines[4] != "port=\"3306\"" || lines[5] != "protocol=tcp" {
		t.Fatalf("unexpected MySQL defaults file lines: %#v", lines)
	}
}
