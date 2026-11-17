package agent

import (
 "os"
 "path/filepath"
 "testing"
)

func TestParseCustomStartCommandUsesSiteExecutableWithoutShell(t *testing.T) {
 root := t.TempDir()
 bin := filepath.Join(root, "bin")
 if err := os.MkdirAll(bin, 0700); err != nil { t.Fatal(err) }
 executable := filepath.Join(bin, "server")
 if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil { t.Fatal(err) }
 cmd, err := parseCustomStartCommand(root, "./bin/server --port 8080")
 if err != nil { t.Fatal(err) }
 if cmd.Path != executable || len(cmd.Args) != 2 || cmd.Args[1] != "--port" { t.Fatalf("unexpected argv: %#v", cmd.Args) }
}

func TestParseCustomStartCommandRejectsShellAndTraversal(t *testing.T) {
 root := t.TempDir()
 for _, raw := range []string{"npm run start; touch /tmp/pwned", "sh -c id", "../outside", "/bin/sh -c id", "node server.js | cat"} {
  if _, err := parseCustomStartCommand(root, raw); err == nil { t.Errorf("expected unsafe command %q to be rejected", raw) }
 }
}
