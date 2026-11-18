package agent

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strconv"
 "sort"
 "strings"
 "sync"
 "syscall"
 "time"
)

type AndroidBuildEnvironment struct {
 Android bool `json:"android"`
 Termux bool `json:"termux"`
 Architecture string `json:"architecture"`
 CPUs int `json:"cpus"`
 TotalMemoryBytes uint64 `json:"total_memory_bytes,omitempty"`
 AvailableMemoryBytes uint64 `json:"available_memory_bytes,omitempty"`
 FreeDiskBytes uint64 `json:"free_disk_bytes,omitempty"`
 RootAvailable bool `json:"root_available"`
 DockerAvailable bool `json:"docker_available"`
 PodmanAvailable bool `json:"podman_available"`
 NixpacksAvailable bool `json:"nixpacks_available"`
 AvailableTools []string `json:"available_tools"`
 Warnings []string `json:"warnings,omitempty"`
}

// DetectAndroidBuildEnvironment performs read-only host inspection.
func DetectAndroidBuildEnvironment(dataDir string) AndroidBuildEnvironment {
 e:=AndroidBuildEnvironment{Android:runtime.GOOS=="android"||fileContains("/proc/version","Android"),Termux:strings.Contains(os.Getenv("PREFIX"),"com.termux"),Architecture:runtime.GOARCH,CPUs:runtime.NumCPU(),RootAvailable:os.Geteuid()==0,AvailableTools:[]string{}}
 for _,n:=range []string{
  "git","node","npm","pnpm","yarn","bun","deno",
  "python","python3","pip","pip3","uv","poetry","pipenv",
  "go","cargo","rustc","gcc","g++","clang","clang++","make","cmake","ninja","pkg-config",
  "php","composer","ruby","gem","bundle","perl","cpanm","R",
  "java","javac","mvn","gradle","sbt","kotlinc",
  "dotnet","swift","dart","elixir","mix","erl","rebar3","ghc","cabal",
  "zig","nim","nimble","crystal","shards","lua","luarocks","ocaml","dune","julia",
 } {if _,err:=exec.LookPath(n);err==nil{e.AvailableTools=append(e.AvailableTools,n)}}
 _,dockerErr:=exec.LookPath("docker"); e.DockerAvailable=dockerErr==nil
 _,podmanErr:=exec.LookPath("podman"); e.PodmanAvailable=podmanErr==nil
 _,nixpacksErr:=exec.LookPath("nixpacks"); e.NixpacksAvailable=nixpacksErr==nil
 e.TotalMemoryBytes,e.AvailableMemoryBytes=readMemoryInfo()
 if dataDir!="" {var st syscall.Statfs_t;if syscall.Statfs(dataDir,&st)==nil{e.FreeDiskBytes=st.Bavail*uint64(st.Bsize)}}
 if e.Android&&!e.RootAvailable {e.Warnings=append(e.Warnings,"Non-root Android: cgroups and kernel-enforced per-build CPU/RAM limits are unavailable.")}
 if e.Android {e.Warnings=append(e.Warnings,"Build scripts run with the Agent user's permissions. This is not a security sandbox; build trusted repositories only.")}
 if e.DockerAvailable||e.PodmanAvailable {e.Warnings=append(e.Warnings,"Container CLI presence does not establish Android container support; container builds are disabled.")}
 if e.NixpacksAvailable {e.Warnings=append(e.Warnings,"Nixpacks is intentionally disabled in the Android Build Engine.")}
 return e
}
func fileContains(path,needle string)bool{b,e:=os.ReadFile(path);return e==nil&&strings.Contains(strings.ToLower(string(b)),strings.ToLower(needle))}
func readMemoryInfo()(total,available uint64){b,e:=os.ReadFile("/proc/meminfo");if e!=nil{return};for _,line:=range strings.Split(string(b),"\n"){f:=strings.Fields(line);if len(f)<2{continue};n,er:=strconv.ParseUint(f[1],10,64);if er!=nil{continue};n*=1024;switch strings.TrimSuffix(f[0],":"){case "MemTotal":total=n;case "MemAvailable":available=n}};return}

type AndroidBuildRequest struct {
 SiteID string `json:"site_id"`
 SiteRoot string `json:"site_root"`
 RootDir string `json:"root_directory,omitempty"`
 Stack string `json:"stack"`
 PackageTool string `json:"package_tool,omitempty"`
 InstallCommand string `json:"install_command,omitempty"`
 BuildCommand string `json:"build_command,omitempty"`
 Timeout time.Duration `json:"-"`
 MaxLogBytes int `json:"max_log_bytes,omitempty"`
 MinAvailableMemoryBytes uint64 `json:"min_available_memory_bytes,omitempty"`
 MinFreeDiskBytes uint64 `json:"min_free_disk_bytes,omitempty"`
 MinFreeDiskBytes uint64 `json:"min_free_disk_bytes,omitempty"`
 AllowProjectCode bool `json:"allow_project_code"`
 Environment map[string]string `json:"environment,omitempty"`
 OnStep func(name string, index, total int) `json:"-"`
}
type AndroidBuildResult struct {
 SiteID string `json:"site_id"`
 Success bool `json:"success"`
 Provider string `json:"provider"`
 Stack string `json:"stack"`
 StartedAt time.Time `json:"started_at"`
 FinishedAt time.Time `json:"finished_at"`
 DurationMillis int64 `json:"duration_ms"`
 Output string `json:"output"`
 OutputTruncated bool `json:"output_truncated"`
 ArtifactDir string `json:"artifact_dir,omitempty"`
 Error string `json:"error,omitempty"`
 HistoryWarning string `json:"history_warning,omitempty"`
}
type androidBuildStep struct{name,exe string;args []string}
type AndroidBuildRunner struct{mu sync.Mutex;busy bool;activeCancel context.CancelFunc;DataDir string}
func NewAndroidBuildRunner(dataDir string)*AndroidBuildRunner{return &AndroidBuildRunner{DataDir:dataDir}}

// Run is a serialized, timeout-bounded local runner. It never invokes a shell.
func(r *AndroidBuildRunner)Run(parent context.Context,req AndroidBuildRequest)AndroidBuildResult{
 started:=time.Now();out:=AndroidBuildResult{SiteID:req.SiteID,Provider:"android-local",Stack:req.Stack,StartedAt:started}
 finish:=func(err error,log *boundedBuildLog)AndroidBuildResult{out.FinishedAt=time.Now();out.DurationMillis=out.FinishedAt.Sub(started).Milliseconds();if log!=nil{out.Output=log.String();out.OutputTruncated=log.truncated};if err!=nil{out.Error=err.Error()}else{out.Success=true};return out}
 if !req.AllowProjectCode{return finish(errors.New("build blocked: explicit allow_project_code=true consent is required"),nil)}
 if !validSiteID(req.SiteID){return finish(errors.New("invalid site id"),nil)}
 root,err:=safeAndroidBuildRoot(req.SiteRoot,req.RootDir);if err!=nil{return finish(err,nil)}
 r.mu.Lock();if r.busy{r.mu.Unlock();return finish(errors.New("another local build is already running"),nil)};r.busy=true;r.mu.Unlock();defer func(){r.mu.Lock();r.busy=false;r.mu.Unlock()}()
 env:=DetectAndroidBuildEnvironment(r.DataDir)
 minMem:=req.MinAvailableMemoryBytes;if minMem==0{minMem=256<<20};minDisk:=req.MinFreeDiskBytes;if minDisk==0{minDisk=128<<20}
 if env.AvailableMemoryBytes>0&&env.AvailableMemoryBytes<minMem{return finish(fmt.Errorf("insufficient available memory: %d bytes; need at least %d",env.AvailableMemoryBytes,minMem),nil)}
 if env.FreeDiskBytes>0&&env.FreeDiskBytes<minDisk{return finish(fmt.Errorf("insufficient free disk: %d bytes; need at least %d",env.FreeDiskBytes,minDisk),nil)}
 steps,err:=androidBuildSteps(root,req);if err!=nil{return finish(err,nil)}
 timeout:=req.Timeout;if timeout<=0{timeout=20*time.Minute};if timeout>2*time.Hour{timeout=2*time.Hour}
 ctx,cancel:=context.WithTimeout(parent,timeout)
 r.mu.Lock();r.activeCancel=cancel;r.mu.Unlock()
 defer func(){cancel();r.mu.Lock();r.activeCancel=nil;r.mu.Unlock()}()
 maxLog:=req.MaxLogBytes;if maxLog<=0{maxLog=10<<20};if maxLog>50<<20{maxLog=50<<20};log:=&boundedBuildLog{limit:maxLog}
 for index,s:=range steps{if req.OnStep != nil { req.OnStep(s.name, index, len(steps)) }; if err:=ctx.Err();err!=nil{return finish(err,log)};if _,err:=exec.LookPath(s.exe);err!=nil{return finish(fmt.Errorf("required Termux tool %q is not installed",s.exe),log)}
  fmt.Fprintf(log,"$ %s %s\n",s.exe,strings.Join(s.args," "))
  buildEnv,envErr:=androidBuildEnv(r.DataDir)
  if envErr!=nil{return finish(fmt.Errorf("prepare build caches: %w",envErr),log)}
  for key,value:=range req.Environment {
   if !validManagedEnvironmentKey(key)||strings.IndexByte(value,0)>=0||len(value)>16384{return finish(fmt.Errorf("invalid Yevqo build environment variable %q",key),log)}
   buildEnv=append(buildEnv,key+"="+value)
  }
  cmd:=exec.CommandContext(ctx,s.exe,s.args...);cmd.Dir=root;cmd.Env=buildEnv;cmd.Stdout=log;cmd.Stderr=log
  cmd.SysProcAttr=&syscall.SysProcAttr{Setpgid:true}
  cmd.WaitDelay=3*time.Second
  cmd.Cancel=func()error{
   if cmd.Process==nil||!isAlive(cmd.Process.Pid){return os.ErrProcessDone}
   if killErr:=killProcessGroup(cmd.Process.Pid,cmd.Process.Pid);killErr!=nil {
    if !isAlive(cmd.Process.Pid){return os.ErrProcessDone}
    return killErr
   }
   return nil
  }
  if err:=cmd.Run();err!=nil{if ctx.Err()!=nil{return finish(ctx.Err(),log)};return finish(fmt.Errorf("%s failed: %w",s.name,err),log)}
 }
 out.ArtifactDir=root;return finish(nil,log)
}
// Cancel requests termination of the active build process group.
func(r *AndroidBuildRunner)Cancel()bool{
 r.mu.Lock();cancel:=r.activeCancel;r.mu.Unlock()
 if cancel==nil{return false}
 cancel()
 return true
}

func safeAndroidBuildRoot(siteRoot,rootDir string)(string,error){
 if siteRoot==""{return "",errors.New("site root is required")};site,err:=filepath.Abs(siteRoot);if err!=nil{return "",err};site,err=filepath.EvalSymlinks(site);if err!=nil{return "",fmt.Errorf("resolve site root: %w",err)}
 if rootDir==""{rootDir="."};if filepath.IsAbs(rootDir)||strings.Contains(rootDir,"\\"){return "",errors.New("root directory must be relative")}
 clean:=filepath.Clean(rootDir);if clean==".."||strings.HasPrefix(clean,".."+string(filepath.Separator)){return "",errors.New("root directory escapes site")}
 target,err:=filepath.EvalSymlinks(filepath.Join(site,clean));if err!=nil{return "",fmt.Errorf("resolve project root: %w",err)}
 rel,err:=filepath.Rel(site,target);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",errors.New("project root resolves outside site")}
 st,err:=os.Stat(target);if err!=nil{return "",err};if !st.IsDir(){return "",errors.New("project root is not a directory")};return target,nil
}
func androidBuildSteps(root string,req AndroidBuildRequest)([]androidBuildStep,error){
 if req.InstallCommand != "" || req.BuildCommand != "" {
  steps:=make([]androidBuildStep,0,2)
  if req.InstallCommand!="" { step,err:=parseManifestBuildCommand(root,"install",req.InstallCommand);if err!=nil{return nil,err};steps=append(steps,step) }
  if req.BuildCommand!="" { step,err:=parseManifestBuildCommand(root,"build",req.BuildCommand);if err!=nil{return nil,err};steps=append(steps,step) }
  if len(steps)==0 { return nil,errors.New("custom build requires at least one install or build command") }
  return steps,nil
 }
 switch req.Stack{
 case "static":return nil,nil
 case "nodejs":
  tool:=req.PackageTool;if tool==""{tool="npm"}
  install:=map[string]androidBuildStep{"npm":{"install","npm",[]string{"install","--no-audit","--no-fund"}},"pnpm":{"install","pnpm",[]string{"install","--no-frozen-lockfile"}},"yarn":{"install","yarn",[]string{"install","--non-interactive"}},"bun":{"install","bun",[]string{"install"}}}[tool]
  if install.exe==""{return nil,fmt.Errorf("unsupported Node package manager %q",tool)}
  steps:=[]androidBuildStep{install}
   if data,err:=os.ReadFile(filepath.Join(root,"package.json"));err==nil {
    var manifest struct { Scripts map[string]string `json:"scripts"` }
    if err:=json.Unmarshal(data,&manifest);err!=nil{return nil,fmt.Errorf("parse package.json: %w",err)}
    if strings.TrimSpace(manifest.Scripts["build"])!="" { steps=append(steps,androidBuildStep{"build",tool,[]string{"run","build"}}) }
   } else { return nil,fmt.Errorf("read package.json: %w",err) }
   return steps,nil
 case "python":
  if _,e:=os.Stat(filepath.Join(root,"requirements.txt"));e==nil{return []androidBuildStep{{"install","python",[]string{"-m","pip","install","-r","requirements.txt"}}},nil}
  if _,e:=os.Stat(filepath.Join(root,"pyproject.toml"));e==nil{return []androidBuildStep{{"install","python",[]string{"-m","pip","install","."}}},nil}
  return nil,errors.New("Python project requires requirements.txt or pyproject.toml")
 case "go":return []androidBuildStep{{"build","go",[]string{"build","-o","veltrix-app","."}}},nil
 case "rust":return []androidBuildStep{{"build","cargo",[]string{"build","--release"}}},nil
 case "php":if _,e:=os.Stat(filepath.Join(root,"composer.json"));e==nil{return []androidBuildStep{{"install","composer",[]string{"install","--no-interaction","--prefer-dist"}}},nil};return nil,errors.New("PHP project requires composer.json")
 case "java":
  if _,e:=os.Stat(filepath.Join(root,"mvnw"));e==nil{return []androidBuildStep{{"build","bash",[]string{"./mvnw","-DskipTests","package"}}},nil}
  return []androidBuildStep{{"build","mvn",[]string{"-DskipTests","package"}}},nil
 case "jvm":
  if _,e:=os.Stat(filepath.Join(root,"gradlew"));e==nil{return []androidBuildStep{{"build","bash",[]string{"./gradlew","build"}}},nil}
  return []androidBuildStep{{"build","gradle",[]string{"build"}}},nil
 case "dotnet":
  return []androidBuildStep{{"publish","dotnet",[]string{"publish","-c","Release","-o","./publish"}}},nil
 case "ruby":
  if _,e:=os.Stat(filepath.Join(root,"Gemfile"));e!=nil{return nil,errors.New("Ruby project requires Gemfile")}
  return []androidBuildStep{{"install","bundle",[]string{"install"}}},nil
 default:return nil,fmt.Errorf("unsupported Android build stack %q",req.Stack)
 }
}

// parseManifestBuildCommand accepts a single executable plus argv. It never
// invokes a shell, expands variables, or evaluates redirections. Any installed
// compiler/build tool can be used, while shell launchers remain blocked.
func parseManifestBuildCommand(root, name, raw string) (androidBuildStep,error) {
 if len(raw)>4096 { return androidBuildStep{},fmt.Errorf("yevqo.yml %s command exceeds 4096 bytes",name) }
 if strings.ContainsAny(raw,";|&`$><\r\n") { return androidBuildStep{},fmt.Errorf("yevqo.yml %s command contains unsupported shell syntax",name) }
 fields,err:=splitBuildCommand(raw)
 if err!=nil{return androidBuildStep{},fmt.Errorf("yevqo.yml %s command: %w",name,err)}
 if len(fields)==0 { return androidBuildStep{},fmt.Errorf("yevqo.yml %s command is empty",name) }
 exe:=fields[0]
 if exe=="./gradlew" || exe=="gradlew" || exe=="./mvnw" || exe=="mvnw" {
  wrapper:=strings.TrimPrefix(exe,"./")
  if _,err:=os.Stat(filepath.Join(root,wrapper));err!=nil{return androidBuildStep{},fmt.Errorf("project wrapper %q is missing",wrapper)}
  if _,err:=exec.LookPath("bash");err!=nil{return androidBuildStep{},errors.New("bash is required to run the project wrapper")}
  return androidBuildStep{name:name,exe:"bash",args:append([]string{"./"+wrapper},fields[1:]...)},nil
 }
 if strings.HasPrefix(exe,"./") {
  relative:=filepath.Clean(strings.TrimPrefix(exe,"./"))
  if relative=="."||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){return androidBuildStep{},errors.New("project executable escapes build root")}
  base,err:=filepath.Abs(root);if err!=nil{return androidBuildStep{},err}
  target,err:=filepath.EvalSymlinks(filepath.Join(base,relative));if err!=nil{return androidBuildStep{},fmt.Errorf("resolve project executable: %w",err)}
  rel,err:=filepath.Rel(base,target);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return androidBuildStep{},errors.New("project executable resolves outside build root")}
  info,err:=os.Stat(target);if err!=nil{return androidBuildStep{},err}
  if !info.Mode().IsRegular()||info.Mode().Perm()&0111==0{return androidBuildStep{},errors.New("project executable must be a regular executable file")}
  return androidBuildStep{name:name,exe:target,args:fields[1:]},nil
 }
 if strings.ContainsAny(exe,"/\\") || exe=="." || exe==".." { return androidBuildStep{},fmt.Errorf("executable must be a tool available on Termux PATH") }
 base:=strings.ToLower(filepath.Base(exe))
 switch base {
 case "sh","bash","dash","ash","zsh","fish","ksh","csh","tcsh","cmd","powershell","pwsh":
  return androidBuildStep{},fmt.Errorf("shell executable %q is disabled; provide the compiler or build tool directly",exe)
 }
 for _,arg:=range fields[1:] { if strings.ContainsAny(arg,"\r\n") { return androidBuildStep{},errors.New("arguments must not contain newlines") } }
 return androidBuildStep{name:name,exe:exe,args:fields[1:]},nil
}

func splitBuildCommand(raw string)([]string,error){
 var fields []string
 var b strings.Builder
 var quote rune
 escaped:=false
 token:=false
 for _,r:=range raw{
  if escaped { b.WriteRune(r);escaped=false;token=true;continue }
  if r=='\\' && quote!='\'' { escaped=true;token=true;continue }
  if quote!=0 { if r==quote { quote=0 } else { b.WriteRune(r) };token=true;continue }
  if r=='\''||r=='"' { quote=r;token=true;continue }
  if r==' '||r=='\t' { if token { fields=append(fields,b.String());b.Reset();token=false };continue }
  b.WriteRune(r);token=true
 }
 if escaped { return nil,errors.New("trailing escape") }
 if quote!=0 { return nil,errors.New("unterminated quote") }
 if token { fields=append(fields,b.String()) }
 return fields,nil
}

func androidBuildEnv(dataDir string)([]string,error){
 allow:=map[string]bool{
  "PATH":true,"PREFIX":true,"HOME":true,"TMPDIR":true,"LANG":true,"LC_ALL":true,"TERM":true,
  "ANDROID_ROOT":true,"ANDROID_DATA":true,"ANDROID_HOME":true,"ANDROID_NDK_HOME":true,
  "JAVA_HOME":true,"GOROOT":true,"GOPATH":true,"GOCACHE":true,"GOMODCACHE":true,"CGO_ENABLED":true,
  "CARGO_HOME":true,"RUSTUP_HOME":true,"PKG_CONFIG_PATH":true,"CPATH":true,"LIBRARY_PATH":true,
  "CFLAGS":true,"CXXFLAGS":true,"LDFLAGS":true,"CC":true,"CXX":true,"AR":true,"GOFLAGS":true,"GOOS":true,"GOARCH":true,
  "RUSTFLAGS":true,"CARGO_BUILD_TARGET":true,"PYTHONPATH":true,"PYTHONUSERBASE":true,"VIRTUAL_ENV":true,
  "JAVA_TOOL_OPTIONS":true,"MAVEN_OPTS":true,"GRADLE_OPTS":true,"KOTLIN_HOME":true,"SCALA_HOME":true,
  "MIX_ENV":true,"MIX_HOME":true,"ERL_AFLAGS":true,"DART_SDK":true,"FLUTTER_ROOT":true,
  "LUA_PATH":true,"LUA_CPATH":true,"PERL5LIB":true,"R_LIBS_USER":true,"CRYSTAL_PATH":true,
  "GEM_HOME":true,"GEM_PATH":true,"BUNDLE_PATH":true,"BUNDLE_WITHOUT":true,
  "COMPOSER_HOME":true,"COMPOSER_CACHE_DIR":true,"PIP_CACHE_DIR":true,"NPM_CONFIG_CACHE":true,"PNPM_HOME":true,"BUN_INSTALL":true,
  "DENO_DIR":true,"DOTNET_ROOT":true,"DOTNET_CLI_HOME":true,"MAVEN_HOME":true,"GRADLE_USER_HOME":true,"PUB_CACHE":true,"JULIA_DEPOT_PATH":true,
 }
 values:=map[string]string{}
 for _,v:=range os.Environ(){k,val,ok:=strings.Cut(v,"=");if ok&&allow[k]{values[k]=val}}
 if dataDir!="" {
  cacheRoot:=filepath.Join(dataDir,"build-cache")
  caches:=map[string]string{
   "GOCACHE":"go-build","GOMODCACHE":"go-mod","CARGO_HOME":"cargo","NPM_CONFIG_CACHE":"npm",
   "PIP_CACHE_DIR":"pip","DENO_DIR":"deno","GRADLE_USER_HOME":"gradle","COMPOSER_CACHE_DIR":"composer",
   "BUNDLE_PATH":"ruby","PUB_CACHE":"dart","DOTNET_CLI_HOME":"dotnet","JULIA_DEPOT_PATH":"julia",
  }
  for key,dir:=range caches {
   if values[key]!=""{continue}
   path:=filepath.Join(cacheRoot,dir)
   if err:=os.MkdirAll(path,0o700);err!=nil{return nil,fmt.Errorf("create %s cache: %w",key,err)}
   values[key]=path
  }
 }
 keys:=make([]string,0,len(values))
 for key:=range values{keys=append(keys,key)}
 sort.Strings(keys)
 env:=make([]string,0,len(keys))
 for _,key:=range keys{env=append(env,key+"="+values[key])}
 return env,nil
}
type boundedBuildLog struct{mu sync.Mutex;data []byte;limit int;truncated bool}
func(w *boundedBuildLog)Write(p []byte)(int,error){w.mu.Lock();defer w.mu.Unlock();n:=len(p);left:=w.limit-len(w.data);if left>0{if len(p)>left{w.data=append(w.data,p[:left]...);w.truncated=true}else{w.data=append(w.data,p...)}}else{w.truncated=true};return n,nil}
func(w *boundedBuildLog)String()string{w.mu.Lock();defer w.mu.Unlock();return string(w.data)}
