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


func TestRotateActiveSiteLogPreservesOpenWriterInode(t *testing.T) {
 root := t.TempDir()
 path := filepath.Join(root, "active.log")
 original := strings.Repeat("log-line\n", maxSiteLogBytes/9+1)
 if err := os.WriteFile(path, []byte(original), 0600); err != nil { t.Fatal(err) }
 writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
 if err != nil { t.Fatal(err) }
 defer writer.Close()
 before, err := os.Stat(path); if err != nil { t.Fatal(err) }
 if err := rotateActiveSiteLog(path); err != nil { t.Fatal(err) }
 after, err := writer.Stat(); if err != nil { t.Fatal(err) }
 if !os.SameFile(before, after) { t.Fatal("active log inode changed; running writer would keep writing to rotated file") }
 if after.Size() != 0 { t.Fatalf("active log size after rotation = %d, want 0", after.Size()) }
 rotated, err := os.ReadFile(path+".1"); if err != nil { t.Fatal(err) }
 if len(rotated) != len(original) { t.Fatalf("rotated bytes = %d, want %d", len(rotated), len(original)) }
}
