package agent

import (
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
)

const (
 maxSiteLogBytes = 5 << 20
 retainedSiteLogs = 3
)

// rotateSiteLog keeps the active log bounded while retaining the three most
// recent complete files. Call only after the previous process has stopped.
func rotateSiteLog(path string) error {
 info, err := os.Stat(path)
 if os.IsNotExist(err) { return nil }
 if err != nil { return fmt.Errorf("inspect site log: %w", err) }
 if info.Size() < maxSiteLogBytes { return nil }
 oldest := fmt.Sprintf("%s.%d", path, retainedSiteLogs)
 if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) { return fmt.Errorf("remove oldest site log: %w", err) }
 for index := retainedSiteLogs - 1; index >= 1; index-- {
  from := fmt.Sprintf("%s.%d", path, index)
  to := fmt.Sprintf("%s.%d", path, index+1)
  if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) { return fmt.Errorf("rotate site log %d: %w", index, err) }
 }
 if err := os.Rename(path, filepath.Clean(path+".1")); err != nil { return fmt.Errorf("rotate active site log: %w", err) }
 return nil
}


// readSiteLogs returns the newest requested lines across rotated and active logs.
func readSiteLogs(dataRoot, siteID string, lines int) (string, error) {
 if !validSiteID(siteID) { return "", fmt.Errorf("invalid site id") }
 if lines <= 0 { lines = 50 }
 if lines > 5000 { lines = 5000 }
 base := filepath.Join(dataRoot, "logs", siteID+".log")
 chunks := make([]string, 0, retainedSiteLogs+1)
 for index := retainedSiteLogs; index >= 1; index-- {
  path := fmt.Sprintf("%s.%d", base, index)
  data, err := os.ReadFile(path)
  if err == nil { chunks = append(chunks, string(data)) } else if !os.IsNotExist(err) { return "", fmt.Errorf("read rotated site log: %w", err) }
 }
 data, err := os.ReadFile(base)
 if err == nil { chunks = append(chunks, string(data)) } else if !os.IsNotExist(err) { return "", fmt.Errorf("read site log: %w", err) }
 if len(chunks) == 0 { return "", fmt.Errorf("log not found: %s", base) }
 all := strings.Join(chunks, "")
 rows := strings.Split(strings.TrimRight(all, "\n"), "\n")
 if len(rows) > lines { rows = rows[len(rows)-lines:] }
 if len(rows) == 1 && rows[0] == "" { return "", nil }
 return strings.Join(rows, "\n"), nil
}


// rotateActiveSiteLog uses copy-truncate so the running child keeps writing to
// the same inode. This is best-effort rotation; the active file remains bounded.
func rotateActiveSiteLog(path string) error {
 info, err := os.Lstat(path)
 if os.IsNotExist(err) { return nil }
 if err != nil { return err }
 if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() { return fmt.Errorf("active site log is not a regular file") }
 if info.Size() < maxSiteLogBytes { return nil }
 oldest := fmt.Sprintf("%s.%d", path, retainedSiteLogs)
 if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) { return err }
 for index:=retainedSiteLogs-1; index>=1; index-- {
  from:=fmt.Sprintf("%s.%d",path,index); to:=fmt.Sprintf("%s.%d",path,index+1)
  if err:=os.Rename(from,to);err!=nil&&!os.IsNotExist(err){return err}
 }
 source, err := os.Open(path); if err != nil { return err }
 temp := path+".1.tmp"
 target, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
 if err != nil { source.Close(); return err }
 _, copyErr := io.Copy(target, source)
 closeTargetErr := target.Close()
 closeSourceErr := source.Close()
 if copyErr != nil { _ = os.Remove(temp); return copyErr }
 if closeTargetErr != nil { _ = os.Remove(temp); return closeTargetErr }
 if closeSourceErr != nil { _ = os.Remove(temp); return closeSourceErr }
 if err := os.Rename(temp, path+".1"); err != nil { _ = os.Remove(temp); return err }
 active, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
 if err != nil { return err }
 return active.Close()
}
