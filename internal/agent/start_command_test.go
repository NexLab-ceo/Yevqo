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
 if cmd.Path != executable || len(cmd.Args) != 3 || cmd.Args[1] != "--port" { t.Fatalf("unexpected argv: %#v", cmd.Args) }
}


func TestCustomStartCommandAcceptsInstalledLanguageRuntime(t *testing.T) {
 root:=t.TempDir()
 bin:=t.TempDir()
 tool:=filepath.Join(bin,"elixir")
 if err:=os.WriteFile(tool,[]byte("#!/bin/sh\nexit 0\n"),0700);err!=nil{t.Fatal(err)}
 t.Setenv("PATH",bin+string(os.PathListSeparator)+os.Getenv("PATH"))
 cmd,err:=parseCustomStartCommand(root,"elixir --version")
 if err!=nil{t.Fatal(err)}
 if cmd.Path!=tool||len(cmd.Args)!=2||cmd.Args[1]!="--version"{t.Fatalf("unexpected custom runtime argv: %#v",cmd.Args)}
}

func TestParseCustomStartCommandRejectsShellAndTraversal(t *testing.T) {
 root := t.TempDir()
 outside := filepath.Join(t.TempDir(), "outside")
 if err := os.WriteFile(outside, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil { t.Fatal(err) }
 if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil { t.Fatal(err) }
 for _, raw := range []string{"npm run start; touch /tmp/pwned", "sh -c id", "../outside", "/bin/sh -c id", "node server.js | cat", "./escape"} {
  if _, err := parseCustomStartCommand(root, raw); err == nil { t.Errorf("expected unsafe command %q to be rejected", raw) }
 }
}
