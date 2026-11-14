package agent

import (
 "context"
 "errors"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strconv"
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
 for _,n:=range []string{"git","node","npm","pnpm","yarn","bun","python","python3","pip","go","cargo","php","composer","java","gradle"} {if _,err:=exec.LookPath(n);err==nil{e.AvailableTools=append(e.AvailableTools,n)}}
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
 AllowProjectCode bool `json:"allow_project_code"`
}
type AndroidBuildResult struct {
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
}
type androidBuildStep struct{name,exe string;args []string}
type AndroidBuildRunner struct{mu sync.Mutex;busy bool;DataDir string}
func NewAndroidBuildRunner(dataDir string)*AndroidBuildRunner{return &AndroidBuildRunner{DataDir:dataDir}}

// Run is a serialized, timeout-bounded local runner. It never invokes a shell.
func(r *AndroidBuildRunner)Run(parent context.Context,req AndroidBuildRequest)AndroidBuildResult{
 started:=time.Now();out:=AndroidBuildResult{Provider:"android-local",Stack:req.Stack,StartedAt:started}
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
 ctx,cancel:=context.WithTimeout(parent,timeout);defer cancel()
 maxLog:=req.MaxLogBytes;if maxLog<=0{maxLog=10<<20};if maxLog>50<<20{maxLog=50<<20};log:=&boundedBuildLog{limit:maxLog}
 for _,s:=range steps{if err:=ctx.Err();err!=nil{return finish(err,log)};if _,err:=exec.LookPath(s.exe);err!=nil{return finish(fmt.Errorf("required Termux tool %q is not installed",s.exe),log)}
  fmt.Fprintf(log,"$ %s %s\n",s.exe,strings.Join(s.args," "))
  cmd:=exec.CommandContext(ctx,s.exe,s.args...);cmd.Dir=root;cmd.Env=androidBuildEnv();cmd.Stdout=log;cmd.Stderr=log
  if err:=cmd.Run();err!=nil{if ctx.Err()!=nil{return finish(ctx.Err(),log)};return finish(fmt.Errorf("%s failed: %w",s.name,err),log)}
 }
 out.ArtifactDir=root;return finish(nil,log)
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
  if req.InstallCommand!="" { step,err:=parseManifestBuildCommand("install",req.InstallCommand);if err!=nil{return nil,err};steps=append(steps,step) }
  if req.BuildCommand!="" { step,err:=parseManifestBuildCommand("build",req.BuildCommand);if err!=nil{return nil,err};steps=append(steps,step) }
  return steps,nil
 }
 switch req.Stack{
 case "static":return nil,nil
 case "nodejs":
  tool:=req.PackageTool;if tool==""{tool="npm"}
  install:=map[string]androidBuildStep{"npm":{"install","npm",[]string{"install","--no-audit","--no-fund"}},"pnpm":{"install","pnpm",[]string{"install","--no-frozen-lockfile"}},"yarn":{"install","yarn",[]string{"install","--non-interactive"}},"bun":{"install","bun",[]string{"install"}}}[tool]
  if install.exe==""{return nil,fmt.Errorf("unsupported Node package manager %q",tool)}
  return []androidBuildStep{install,{"build",tool,[]string{"run","build"}}},nil
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

func parseManifestBuildCommand(name, raw string) (androidBuildStep,error) {
 if strings.ContainsAny(raw,";|&`$><\r\n") { return androidBuildStep{},fmt.Errorf("veltrix.yaml %s command contains unsupported shell syntax",name) }
 fields:=strings.Fields(raw)
 if len(fields)==0 { return androidBuildStep{},fmt.Errorf("veltrix.yaml %s command is empty",name) }
 exe:=filepath.Base(fields[0])
 allowed:=map[string]bool{"npm":true,"npx":true,"pnpm":true,"yarn":true,"bun":true,"python":true,"python3":true,"pip":true,"pip3":true,"go":true,"cargo":true,"composer":true,"php":true,"bundle":true,"mvn":true,"gradle":true,"dotnet":true}
 if !allowed[exe] { return androidBuildStep{},fmt.Errorf("veltrix.yaml %s command executable %q is not allowlisted",name,exe) }
 return androidBuildStep{name:name,exe:exe,args:fields[1:]},nil
}

func androidBuildEnv()[]string{allow:=map[string]bool{"PATH":true,"PREFIX":true,"HOME":true,"TMPDIR":true,"LANG":true,"LC_ALL":true,"TERM":true,"ANDROID_ROOT":true,"ANDROID_DATA":true};var env []string;for _,v:=range os.Environ(){k,_,_:=strings.Cut(v,"=");if allow[k]{env=append(env,v)}};return env}
type boundedBuildLog struct{mu sync.Mutex;data []byte;limit int;truncated bool}
func(w *boundedBuildLog)Write(p []byte)(int,error){w.mu.Lock();defer w.mu.Unlock();n:=len(p);left:=w.limit-len(w.data);if left>0{if len(p)>left{w.data=append(w.data,p[:left]...);w.truncated=true}else{w.data=append(w.data,p...)}}else{w.truncated=true};return n,nil}
func(w *boundedBuildLog)String()string{w.mu.Lock();defer w.mu.Unlock();return string(w.data)}
