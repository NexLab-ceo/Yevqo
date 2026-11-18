package agent

import (
	"fmt
	"os"
	"path/filepath"
	"testing"
)

func project(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil { t.Fatal(err) }
	}
	return root
}

func TestAnalyzeProject(t *testing.T) {
	cases := []struct{ name string; files []string; provider, stack string; needsConfig bool }{
		{"Docker wins", []string{"Dockerfile","package.json"}, "dockerfile","container",false},
		{"pnpm", []string{"package.json","pnpm-lock.yaml"}, "auto","nodejs",false},
		{"Python", []string{"pyproject.toml"}, "auto","python",false},
		{"Rust", []string{"Cargo.toml"}, "auto","rust",false},
		{"Go", []string{"go.mod"}, "auto","go",false},
		{"Static", []string{"index.html"}, "static","static",false},
		{"CMake C/C++", []string{"CMakeLists.txt"}, "auto","cmake",false},
		{"Makefile", []string{"Makefile"}, "auto","make",false},
		{"Swift", []string{"Package.swift"}, "auto","swift",false},
		{"Elixir", []string{"mix.exs"}, "auto","elixir",false},
		{"Scala", []string{"build.sbt"}, "auto","scala",false},
		{"Zig", []string{"build.zig"}, "auto","zig",false},
		{"Dart", []string{"pubspec.yaml","bin/main.dart"}, "auto","dart",false},
		{"Deno", []string{"deno.json"}, "auto","deno",false},
		{"Erlang", []string{"rebar.config"}, "auto","erlang",false},
		{"OCaml", []string{"dune-project"}, "auto","ocaml",false},
		{"Haskell", []string{"sample.cabal"}, "auto","haskell",false},
		{"Nim", []string{"sample.nimble"}, "auto","nim",false},
		{"Lua", []string{"sample.rockspec"}, "auto","lua",false},
		{"Unknown", []string{"README.md","src/main.xyz"}, "auto","unknown",true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AnalyzeProject(project(t, tc.files...))
			if err != nil { t.Fatal(err) }
			if got.Provider != tc.provider || got.Stack != tc.stack || got.NeedsConfig != tc.needsConfig {
				t.Fatalf("unexpected plan: %+v", got)
			}
		})
	}
}


func TestAnalyzeNodeProjectRespectsLockfileAndScripts(t *testing.T) {
 root:=project(t,"package.json")
 if err:=os.WriteFile(filepath.Join(root,"package.json"),[]byte(`{"scripts":{"start":"node server.js"}}`),0644);err!=nil{t.Fatal(err)}
 plan,err:=AnalyzeProject(root)
 if err!=nil{t.Fatal(err)}
 if plan.Install!="npm install --no-audit --no-fund"||plan.Build!=""||plan.Start!="npm run start"{t.Fatalf("unexpected no-lock/no-build plan: %+v",plan)}
 if err:=os.WriteFile(filepath.Join(root,"package-lock.json"),[]byte("{}"),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"package.json"),[]byte(`{"scripts":{"build":"vite build"}}`),0644);err!=nil{t.Fatal(err)}
 plan,err=AnalyzeProject(root)
 if err!=nil{t.Fatal(err)}
 if plan.Install!="npm ci"||plan.Build!="npm run build"||plan.Start!=""{t.Fatalf("unexpected locked plan: %+v",plan)}
}

func TestAnalyzeCustomYevqoConfigEnablesUnknownLanguage(t *testing.T) {
 root:=project(t,"src/main.nim")
 manifest:="version: 1\nbuild:\n  provider: custom\n  install: nimble install -y\n  command: nim c -o app src/main.nim\n"
 if err:=os.WriteFile(filepath.Join(root,"yevqo.yml"),[]byte(manifest),0644);err!=nil{t.Fatal(err)}
 plan,err:=AnalyzeProject(root)
 if err!=nil{t.Fatal(err)}
 if plan.Stack!="custom"||plan.NeedsConfig||plan.Install!="nimble install -y"||plan.Build!="nim c -o app src/main.nim"{t.Fatalf("custom language manifest was not applied: %+v",plan)}
}

func TestAnalyzeProjectRejectsFileRoot(t *testing.T) {
	root := project(t, "README.md")
	if err := os.WriteFile(filepath.Join(root,"not-dir"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	if _, err := AnalyzeProject(filepath.Join(root,"not-dir")); err == nil { t.Fatal("expected non-directory root to fail") }
}

func TestLegacyVeltrixConfigIsIgnored(t *testing.T) {
 root := t.TempDir()
 legacy := "version: 1\nbuild:\n  provider: custom\n  command: nim c app.nim\n"
 if err := os.WriteFile(filepath.Join(root, "veltrix.yaml"), []byte(legacy), 0600); err != nil { t.Fatal(err) }
 plan, err := AnalyzeProject(root)
 if err != nil { t.Fatal(err) }
 if plan.Stack != "unknown" || !plan.NeedsConfig { t.Fatalf("legacy veltrix.yaml must not configure builds: %+v", plan) }
 for _, evidence := range plan.Evidence { if evidence == "veltrix.yaml" { t.Fatal("legacy config was treated as active") } }
}

func TestYevqoConfigMetadataAndStrictValidation(t *testing.T) {
 root:=project(t,"package.json")
 body:="version: 1\nproject:\n  name: demo\n  runtime: nodejs\n  root: .\nbuild:\n  provider: auto\nrun:\n  port: 3210\n"
 if err:=os.WriteFile(filepath.Join(root,"yevqo.yml"),[]byte(body),0600);err!=nil{t.Fatal(err)}
 plan,err:=AnalyzeProject(root);if err!=nil{t.Fatal(err)}
 if plan.ProjectName!="demo"||plan.Runtime!="nodejs"||plan.Port!=3210{t.Fatalf("metadata/config not applied: %+v",plan)}
}

func TestYevqoConfigRejectsUnsupportedAndMalformedFields(t *testing.T) {
 cases:=[]string{
  "version: 2\nproject:\n  root: .\n",
  "version: 1\nproject:\n  root: .\nhealthcheck:\n  path: /health\n",
  "version: 1\nproject:\n  root: .\nbuild:\n  command: npm run build\n  command: npm run test\n",
  "version: 1\nproject:\n\troot: .\n",
 }
 for i,body:=range cases{t.Run(fmt.Sprintf("case-%d",i),func(t *testing.T){
  root:=t.TempDir();if err:=os.WriteFile(filepath.Join(root,"yevqo.yml"),[]byte(body),0600);err!=nil{t.Fatal(err)}
  if _,err:=AnalyzeProject(root);err==nil{t.Fatalf("expected invalid config to fail: %q",body)}
 })}
}
