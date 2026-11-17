package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeTestProcStat(t *testing.T, root string, pid, group int, name string, userTicks, systemTicks uint64, rssPages int64) {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprint(pid))
	if err := os.MkdirAll(dir, 0700); err != nil { t.Fatal(err) }
	// Fields following comm: state, ppid, pgrp, session, tty, tpgid,
	// flags, minflt, cminflt, majflt, cmajflt, utime, stime, ...
	fields := []any{"S", 1, group, group, 0, -1, 0, 0, 0, 0, 0, userTicks, systemTicks, 0, 0, 20, 0, 1, 0, 0, 0, rssPages}
	line := fmt.Sprintf("%d (%s) ", pid, name)
	for i, field := range fields {
		if i > 0 { line += " " }
		line += fmt.Sprint(field)
	}
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(line), 0600); err != nil { t.Fatal(err) }
}

func TestParseLinuxProcessStatHandlesParenthesesInCommand(t *testing.T) {
	pid, group, counters, err := parseLinuxProcessStat("123 (worker (node)) S 1 120 120 0 0 0 0 0 0 0 12 8 0 0 20 0 1 0 0 0 7")
	if err != nil { t.Fatal(err) }
	if pid != 123 || group != 120 { t.Fatalf("identity = pid %d group %d", pid, group) }
	if counters.cpu != 20 || counters.rss != 7*uint64(os.Getpagesize()) { t.Fatalf("unexpected counters: %#v", counters) }
}

func TestReadLinuxProcessGroupCountersAggregatesOnlyMatchingGroup(t *testing.T) {
	root := t.TempDir()
	writeTestProcStat(t, root, 101, 100, "node", 10, 5, 4)
	writeTestProcStat(t, root, 102, 100, "worker (node)", 20, 5, 6)
	writeTestProcStat(t, root, 103, 200, "unrelated", 100, 100, 100)
	if err := os.MkdirAll(filepath.Join(root, "not-a-pid"), 0700); err != nil { t.Fatal(err) }
	got, err := readLinuxProcessGroupCountersAt(root, 100)
	if err != nil { t.Fatal(err) }
	if got.count != 2 { t.Fatalf("process count = %d, want 2", got.count) }
	if got.cpu != 40 { t.Fatalf("CPU ticks = %d, want 40", got.cpu) }
	wantRSS := uint64(10) * uint64(os.Getpagesize())
	if got.rss != wantRSS { t.Fatalf("RSS = %d, want %d", got.rss, wantRSS) }
}

func TestReadLinuxProcessGroupCountersRejectsMissingGroup(t *testing.T) {
	if _, err := readLinuxProcessGroupCountersAt(t.TempDir(), 123); err == nil {
		t.Fatal("expected missing process group to return an error")
	}
}
