package agent

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestRotateSiteLogRetainsBoundedHistory(t *testing.T) {
 root := t.TempDir()
 path := filepath.Join(root, "demo.log")
 if err := os.WriteFile(path, []byte(strings.Repeat("a", maxSiteLogBytes)), 0600); err != nil { t.Fatal(err) }
 if err := rotateSiteLog(path); err != nil { t.Fatal(err) }
 if _, err := os.Stat(path+".1"); err != nil { t.Fatalf("first rotated log missing: %v", err) }
 if _, err := os.Stat(path); !os.IsNotExist(err) { t.Fatalf("active log should have moved, stat err=%v", err) }
 for i:=1; i<=retainedSiteLogs; i++ {
  if err := os.WriteFile(path, []byte(strings.Repeat(string(rune('a'+i)), maxSiteLogBytes)), 0600); err != nil { t.Fatal(err) }
  if err := rotateSiteLog(path); err != nil { t.Fatal(err) }
 }
 if _, err := os.Stat(path+"."+string(rune('0'+retainedSiteLogs+1))); !os.IsNotExist(err) { t.Fatal("rotation retained more generations than configured") }
}

func TestReadSiteLogsCombinesRotatedFilesAndLimitsLines(t *testing.T) {
 root := t.TempDir()
 dir := filepath.Join(root, "logs")
 if err := os.MkdirAll(dir, 0700); err != nil { t.Fatal(err) }
 base := filepath.Join(dir, "demo.log")
 for name, body := range map[string]string{
  base+".2": "oldest\n", base+".1": "middle\n", base: "newest\nlast\n",
 } {
  if err := os.WriteFile(name, []byte(body), 0600); err != nil { t.Fatal(err) }
 }
 got, err := readSiteLogs(root, "demo", 3)
 if err != nil { t.Fatal(err) }
 if got != "middle\nnewest\nlast" { t.Fatalf("unexpected combined log: %q", got) }
 got, err = readSiteLogs(root, "demo", 1)
 if err != nil { t.Fatal(err) }
 if got != "last" { t.Fatalf("line limit not applied: %q", got) }
 if _, err := readSiteLogs(root, "../escape", 10); err == nil { t.Fatal("expected invalid site id") }
}
