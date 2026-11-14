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
