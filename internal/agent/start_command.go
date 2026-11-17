package agent

import (
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
)

// parseCustomStartCommand parses argv without a shell. Executables are either
// allowlisted tools from PATH or executable files located inside the site root.
func parseCustomStartCommand(root, raw string) (*exec.Cmd, error) {
 raw = strings.TrimSpace(raw)
 if raw == "" { return nil, fmt.Errorf("custom start command is empty") }
 if len(raw) > 512 || strings.ContainsAny(raw, ";|&`$><\r\n\x00") {
  return nil, fmt.Errorf("custom start command contains unsupported shell syntax")
 }
 fields := strings.Fields(raw)
 if len(fields) == 0 { return nil, fmt.Errorf("custom start command is empty") }
 executable := fields[0]
 if strings.Contains(executable, "/") {
  if filepath.IsAbs(executable) || strings.Contains(executable, "\\") {
   return nil, fmt.Errorf("custom executable path must be relative to the site")
  }
  clean := filepath.Clean(executable)
  if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
   return nil, fmt.Errorf("custom executable must stay inside the site")
  }
  resolvedRoot, err := filepath.EvalSymlinks(root)
  if err != nil { return nil, fmt.Errorf("resolve site root: %w", err) }
  resolvedExe, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, clean))
  if err != nil { return nil, fmt.Errorf("resolve custom executable: %w", err) }
  rel, err := filepath.Rel(resolvedRoot, resolvedExe)
  if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
   return nil, fmt.Errorf("custom executable resolves outside the site")
  }
  info, err := os.Stat(resolvedExe)
  if err != nil { return nil, err }
  if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
   return nil, fmt.Errorf("custom executable is not an executable regular file")
  }
  executable = resolvedExe
 } else {
  allowed := map[string]bool{"npm":true,"node":true,"npx":true,"pnpm":true,"yarn":true,"bun":true,"python":true,"python3":true,"php":true,"go":true,"java":true,"ruby":true,"cargo":true,"dotnet":true,"composer":true,"bundle":true}
  if !allowed[executable] { return nil, fmt.Errorf("custom start executable %q is not allowlisted", executable) }
  resolved, err := exec.LookPath(executable)
  if err != nil { return nil, fmt.Errorf("custom start executable %q is not installed", executable) }
  executable = resolved
 }
 return exec.Command(executable, fields[1:]...), nil
}
