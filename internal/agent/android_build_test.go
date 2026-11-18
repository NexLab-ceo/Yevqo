package agent

import (
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)
func TestSafeAndroidBuildRoot(t *testing.T){
 base:=t.TempDir();site:=filepath.Join(base,"site");outside:=filepath.Join(base,"outside")
 if err:=os.MkdirAll(site,0755);err!=nil{t.Fatal(err)};if err:=os.MkdirAll(outside,0755);err!=nil{t.Fatal(err)}
 if _,err:=safeAndroidBuildRoot(site,"../outside");err==nil{t.Fatal("expected traversal rejection")}
 if err:=os.Symlink(outside,filepath.Join(site,"escape"));err==nil{if _,err:=safeAndroidBuildRoot(site,"escape");err==nil{t.Fatal("expected symlink escape rejection")}}
}
func TestAndroidBuildRequiresConsent(t *testing.T){
 r:=NewAndroidBuildRunner(t.TempDir());got:=r.Run(context.Background(),AndroidBuildRequest{SiteID:"demo",SiteRoot:t.TempDir(),Stack:"static"})
 if got.Success||!strings.Contains(got.Error,"allow_project_code"){t.Fatalf("unexpected result: %+v",got)}
}
func TestAndroidStaticBuildAndLogLimit(t *testing.T){
 root:=t.TempDir();got:=NewAndroidBuildRunner(t.TempDir()).Run(context.Background(),AndroidBuildRequest{SiteID:"demo",SiteRoot:root,Stack:"static",AllowProjectCode:true,Timeout:time.Second})
 if !got.Success{t.Fatalf("static build failed: %+v",got)}
 w:=&boundedBuildLog{limit:4};_,_=w.Write([]byte("123456"));if w.String()!="1234"||!w.truncated{t.Fatal("log was not bounded")}
}
func TestDockerProviderDisabled(t *testing.T){if _,err:=androidBuildSteps(t.TempDir(),AndroidBuildRequest{Stack:"dockerfile"});err==nil{t.Fatal("Docker must not execute on Android provider")}}

func TestAndroidBuildProviderCommands(t *testing.T) {
 root:=t.TempDir()
 if err:=os.WriteFile(filepath.Join(root,"build.gradle.kts"),[]byte("plugins {}"),0600);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"gradlew"),[]byte("#!/bin/sh\nexit 0\n"),0700);err!=nil{t.Fatal(err)}
 steps,err:=androidBuildSteps(root,AndroidBuildRequest{Stack:"jvm"})
 if err!=nil{t.Fatal(err)}
 if len(steps)!=1||steps[0].exe!="bash"||len(steps[0].args)<2||steps[0].args[0]!="./gradlew"{t.Fatalf("unexpected Gradle wrapper plan: %#v",steps)}
 if err:=os.Remove(filepath.Join(root,"gradlew"));err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"pom.xml"),[]byte("<project/>"),0600);err!=nil{t.Fatal(err)}
 steps,err=androidBuildSteps(root,AndroidBuildRequest{Stack:"java"})
 if err!=nil{t.Fatal(err)}
 if len(steps)!=1||steps[0].exe!="mvn"{t.Fatalf("unexpected Maven plan: %#v",steps)}
 if err:=os.WriteFile(filepath.Join(root,"Gemfile"),[]byte("source 'https://rubygems.org'"),0600);err!=nil{t.Fatal(err)}
 steps,err=androidBuildSteps(root,AndroidBuildRequest{Stack:"ruby"})
 if err!=nil||len(steps)!=1||steps[0].exe!="bundle"{t.Fatalf("unexpected Ruby plan: %#v, %v",steps,err)}
 steps,err=androidBuildSteps(root,AndroidBuildRequest{Stack:"dotnet"})
 if err!=nil||len(steps)!=1||steps[0].exe!="dotnet"{t.Fatalf("unexpected .NET plan: %#v, %v",steps,err)}
}

func TestVeltrixManifestAndSafeCommands(t *testing.T) {
 root:=t.TempDir();app:=filepath.Join(root,"app")
 if err:=os.MkdirAll(app,0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(app,"package.json"),[]byte(`{"scripts":{"build":"vite build"}}`),0600);err!=nil{t.Fatal(err)}
 manifest:= "version: 1\nproject:\n  name: demo\n  root: ./app\nbuild:\n  provider: auto\n  install: npm install\n  command: npm run build\nrun:\n  command: npm run start\n  port: 3000\n"
 if err:=os.WriteFile(filepath.Join(root,"veltrix.yaml"),[]byte(manifest),0600);err!=nil{t.Fatal(err)}
 plan,err:=AnalyzeProject(root);if err!=nil{t.Fatal(err)}
 if plan.Stack!="nodejs"||plan.Root!="app"||plan.Install!="npm install"||plan.Build!="npm run build"||plan.Start!="npm run start"||plan.Port!=3000{t.Fatalf("manifest was not applied: %+v",plan)}
 step,err:=parseManifestBuildCommand(root,"build","npm run build")
 if err!=nil||step.exe!="npm"||len(step.args)!=2{t.Fatalf("valid command rejected: %#v %v",step,err)}
 if _,err:=parseManifestBuildCommand(root,"build","npm run build && curl bad");err==nil{t.Fatal("shell operators must be rejected")}
 if _,err:=parseManifestBuildCommand(root,"build","sh -c id");err==nil{t.Fatal("shell executables must be rejected")}
}



func TestAndroidBuildEnvironmentUsesPersistentCachesAndFiltersSecrets(t *testing.T) {
 data:=t.TempDir()
 t.Setenv("BUILD_SECRET_SHOULD_NOT_PASS","do-not-leak")
 t.Setenv("RUSTFLAGS","-C opt-level=2")
 env,err:=androidBuildEnv(data)
 if err!=nil{t.Fatal(err)}
 joined:=strings.Join(env,"\n")
 if strings.Contains(joined,"BUILD_SECRET_SHOULD_NOT_PASS"){t.Fatal("unapproved environment variable leaked into build")}
 if !strings.Contains(joined,"RUSTFLAGS=-C opt-level=2"){t.Fatal("configured language toolchain flags were not preserved")}
 for _,key:=range []string{"GOCACHE=","GOMODCACHE=","CARGO_HOME=","NPM_CONFIG_CACHE=","PIP_CACHE_DIR=","GRADLE_USER_HOME="} {
  if !strings.Contains(joined,key){t.Errorf("persistent cache %s missing from build environment",key)}
 }
 if _,err:=os.Stat(filepath.Join(data,"build-cache","go-build"));err!=nil{t.Fatalf("Go build cache was not created: %v",err)}
}


func TestAndroidBuildCanBeCancelled(t *testing.T) {
 root:=t.TempDir()
 bin:=t.TempDir()
 tool:=filepath.Join(bin,"slowcompiler")
 if err:=os.WriteFile(tool,[]byte("#!/bin/sh\nexec sleep 30\n"),0700);err!=nil{t.Fatal(err)}
 t.Setenv("PATH",bin+string(os.PathListSeparator)+os.Getenv("PATH"))
 runner:=NewAndroidBuildRunner(t.TempDir())
 done:=make(chan AndroidBuildResult,1)
 go func(){done<-runner.Run(context.Background(),AndroidBuildRequest{SiteID:"cancel-demo",SiteRoot:root,Stack:"custom",BuildCommand:"slowcompiler",AllowProjectCode:true,Timeout:time.Minute})}()
 deadline:=time.Now().Add(3*time.Second)
 active:=false
 for time.Now().Before(deadline){
  runner.mu.Lock();active=runner.activeCancel!=nil;runner.mu.Unlock()
  if active{break}
  time.Sleep(10*time.Millisecond)
 }
 if !active{t.Fatal("build did not enter running state")}
 if !runner.Cancel(){t.Fatal("active build was not cancellable")}
 select{
 case result:=<-done:
  if result.Success||!strings.Contains(result.Error,"context canceled"){t.Fatalf("unexpected cancellation result: %+v",result)}
 case <-time.After(6*time.Second):
  t.Fatal("cancelled build did not exit promptly")
 }
}

func TestUniversalBuildCommandSupportsInstalledToolchains(t *testing.T) {
 root:=t.TempDir()
 bin:=t.TempDir()
 tool:=filepath.Join(bin,"nim")
 if err:=os.WriteFile(tool,[]byte("#!/bin/sh\nprintf '%s\\n' \"custom compiler ran\"\n"),0700);err!=nil{t.Fatal(err)}
 t.Setenv("PATH",bin+string(os.PathListSeparator)+os.Getenv("PATH"))
 step,err:=parseManifestBuildCommand(root,"build",`nim c -o "build/my app" src/main.nim`)
 if err!=nil{t.Fatal(err)}
 if step.exe!="nim"||len(step.args)!=4||step.args[2]!="build/my app"{t.Fatalf("unexpected argv: %#v",step)}
 result:=NewAndroidBuildRunner(t.TempDir()).Run(context.Background(),AndroidBuildRequest{
  SiteID:"universal",SiteRoot:root,Stack:"custom",BuildCommand:"nim compile src/main.nim",
  AllowProjectCode:true,Timeout:time.Second,MaxLogBytes:1024,
 })
 if !result.Success||!strings.Contains(result.Output,"custom compiler ran"){t.Fatalf("custom toolchain did not run: %+v",result)}
}


func TestUniversalBuildSupportsProjectLocalExecutable(t *testing.T) {
 root:=t.TempDir()
 script:=filepath.Join(root,"build.sh")
 if err:=os.WriteFile(script,[]byte("#!/bin/sh\nprintf 'local build ran\\n'\n"),0755);err!=nil{t.Fatal(err)}
 step,err:=parseManifestBuildCommand(root,"build","./build.sh --release")
 if err!=nil{t.Fatal(err)}
 if step.exe!=script||len(step.args)!=1||step.args[0]!="--release"{t.Fatalf("unexpected local executable step: %#v",step)}
 if _,err:=parseManifestBuildCommand(root,"build","./../outside");err==nil{t.Fatal("project executable traversal was accepted")}
}

func TestUniversalBuildCommandRejectsShellAndMalformedQuotes(t *testing.T) {
 root:=t.TempDir()
 for _,command:=range []string{"sh -c id","bash -c id","npm run build && rm -rf .","nim \"unterminated"} {
  if _,err:=parseManifestBuildCommand(root,"build",command);err==nil{t.Errorf("unsafe or malformed command accepted: %q",command)}
 }
}

func TestNodeBuildSkipsMissingBuildScript(t *testing.T) {
	root:=t.TempDir()
	if err:=os.WriteFile(filepath.Join(root,"package.json"),[]byte(`{"scripts":{"start":"node server.js"}}`),0600);err!=nil{t.Fatal(err)}
	steps,err:=androidBuildSteps(root,AndroidBuildRequest{Stack:"nodejs",PackageTool:"npm"})
	if err!=nil{t.Fatal(err)}
	if len(steps)!=1||steps[0].name!="install"{t.Fatalf("expected install-only plan, got %#v",steps)}
	if err:=os.WriteFile(filepath.Join(root,"package.json"),[]byte(`{"scripts":{"build":"vite build"}}`),0600);err!=nil{t.Fatal(err)}
	steps,err=androidBuildSteps(root,AndroidBuildRequest{Stack:"nodejs",PackageTool:"npm"})
	if err!=nil{t.Fatal(err)}
	if len(steps)!=2||steps[1].name!="build"{t.Fatalf("expected install and build steps, got %#v",steps)}
}


func TestAndroidBuildReportsFailedProviderStep(t *testing.T) {
 root := t.TempDir()
 bin := t.TempDir()
 fakeNPM := filepath.Join(bin, "npm")
 if err := os.WriteFile(fakeNPM, []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil { t.Fatal(err) }
 t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
 result := NewAndroidBuildRunner(t.TempDir()).Run(context.Background(), AndroidBuildRequest{
  SiteID:"failed-build", SiteRoot:root, Stack:"custom", InstallCommand:"npm install",
  AllowProjectCode:true, Timeout:time.Second, MaxLogBytes:1024,
 })
 if result.Success { t.Fatalf("expected failed provider step, got success: %+v", result) }
 if !strings.Contains(result.Error, "install failed") { t.Fatalf("unexpected build error: %q", result.Error) }
}
