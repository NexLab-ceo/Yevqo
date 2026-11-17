package agent

import (
 "fmt"
 "os"
 "path/filepath"
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
