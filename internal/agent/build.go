package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"os"
	"path/filepath"
	"strings"
 "time"
)

// BuildPlan is a deterministic, non-executing description of how a project
// should be built. Detection never runs project-provided code.
type BuildPlan struct {
		Provider     string   `json:"provider"`
		Stack        string   `json:"stack"`
		PackageTool  string   `json:"package_tool,omitempty"`
		Install      string   `json:"install_command,omitempty"`
		Build        string   `json:"build_command,omitempty"`
		Start        string   `json:"start_command,omitempty"`
		Port         int      `json:"port,omitempty"`
		Root         string   `json:"root_directory"`
		Confidence   string   `json:"confidence"`
		NeedsConfig  bool     `json:"needs_configuration"`
		Evidence     []string `json:"evidence"`
		Warnings     []string `json:"warnings,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Runtime string `json:"runtime,omitempty"`
 Output string `json:"build_output,omitempty"`
	Capabilities BuildCapabilities `json:"capabilities"`
	AndroidEnvironment AndroidBuildEnvironment `json:"android_environment"`
}

// AnalyzeProject inspects only the immediate project root. It does not execute
// scripts, parse arbitrary build plugins, or follow symlinks.
type yevqoManifest struct {
 Name, Runtime, Root, Provider, Install, Build, Output, Start string
 Port int
 Environment map[string]string
 HealthcheckPath string
 HealthcheckInterval time.Duration
 MemoryBytes uint64
 DeployRestart bool
 DeployRestartSet bool
 ArtifactsInclude, ArtifactsExclude, CachePaths []string
 CacheEnabled bool
}

// AnalyzeProject honors the canonical yevqo.yml configuration before auto-detection.
func AnalyzeProject(root string) (BuildPlan, error) {
 abs, err := filepath.Abs(root)
 if err != nil { return BuildPlan{}, fmt.Errorf("resolve project root: %w", err) }
 manifest, found, err := readYevqoManifest(abs)
 if err != nil { return BuildPlan{}, err }
 if !found { return analyzeProjectMarkers(abs) }
 if manifest.Provider != "" {
  switch manifest.Provider { case "auto","custom","static","android-local","dockerfile","nixpacks","railpack": default: return BuildPlan{}, fmt.Errorf("unsupported yevqo.yml build.provider %q", manifest.Provider) }
 }
 manifestRoot := manifest.Root
 if manifestRoot == "" { manifestRoot = "." }
 if filepath.IsAbs(manifestRoot) || strings.Contains(manifestRoot, "\\\\") { return BuildPlan{}, fmt.Errorf("yevqo.yml project.root must be relative") }
 clean := filepath.Clean(manifestRoot)
 if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) { return BuildPlan{}, fmt.Errorf("yevqo.yml project.root escapes project directory") }
 projectRoot, err := filepath.EvalSymlinks(filepath.Join(abs, clean))
 if err != nil { return BuildPlan{}, fmt.Errorf("resolve yevqo.yml project.root: %w", err) }
 rel, err := filepath.Rel(abs, projectRoot)
 if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return BuildPlan{}, fmt.Errorf("yevqo.yml project.root resolves outside project directory") }
 plan, err := analyzeProjectMarkers(projectRoot)
 if err != nil { return BuildPlan{}, err }
 plan.Root = clean
 plan.Evidence = append(plan.Evidence, "yevqo.yml")
 if manifest.Provider != "" && manifest.Provider != "auto" {
  plan.Provider = manifest.Provider
  if (manifest.Provider == "dockerfile" || manifest.Provider == "nixpacks" || manifest.Provider == "railpack") && (plan.AndroidEnvironment.Android || plan.AndroidEnvironment.Termux) {
   plan.NeedsConfig = true
   plan.Warnings = append(plan.Warnings, manifest.Provider+" builds are disabled on the Android local provider")
  }
 }
 if manifest.Provider == "static" { plan.Stack = "static"; plan.NeedsConfig = false }
 plan.ProjectName = manifest.Name
 plan.Runtime = manifest.Runtime
 plan.Output = manifest.Output
 if manifest.Install != "" { plan.Install = manifest.Install }
 if manifest.Build != "" { plan.Build = manifest.Build }
 if manifest.Start != "" { plan.Start = manifest.Start }
 if manifest.Port > 0 { plan.Port = manifest.Port }
 if plan.Stack == "unknown" && (manifest.Install != "" || manifest.Build != "") { plan.Stack = "custom"; plan.NeedsConfig = false }
 if manifest.Install != "" || manifest.Build != "" { plan.NeedsConfig = false }
 return plan, nil
}

func readYevqoManifest(root string) (yevqoManifest, bool, error) {
 var m yevqoManifest
 path:=filepath.Join(root,"yevqo.yml");info,err:=os.Lstat(path)
 if os.IsNotExist(err){return m,false,nil};if err!=nil{return m,false,err}
 if !info.Mode().IsRegular(){return m,false,fmt.Errorf("yevqo.yml must be a regular file")}
 f,err:=os.Open(path);if err!=nil{return m,false,err};defer f.Close()
 m.Environment=map[string]string{};m.CacheEnabled=true
 sc:=bufio.NewScanner(f);sc.Buffer(make([]byte,4096),64*1024)
 section,listKey:="","";seen:=map[string]bool{};version:=false;line:=0
 for sc.Scan(){line++;raw:=sc.Text();v:=strings.TrimSpace(raw);if v==""||strings.HasPrefix(v,"#"){continue}
  if strings.Contains(raw,"\t"){return m,false,fmt.Errorf("yevqo.yml:%d: tabs are forbidden",line)}
  ind:=len(raw)-len(strings.TrimLeft(raw," "));if ind!=0&&ind!=2&&ind!=4{return m,false,fmt.Errorf("yevqo.yml:%d: use 0, 2 or 4 spaces",line)}
  if ind==0{p:=strings.SplitN(v,":",2);if len(p)!=2{return m,false,fmt.Errorf("yevqo.yml:%d: expected key: value",line)};k,val:=strings.TrimSpace(p[0]),strings.TrimSpace(p[1])
   if k=="version"{if version||strings.Trim(val,"\"'")!="1"{return m,false,fmt.Errorf("yevqo.yml requires exactly one version: 1")};version=true;continue}
   if val!=""{return m,false,fmt.Errorf("yevqo.yml:%d: section value not allowed",line)}
   switch k{case "project","build","run","environment","healthcheck","artifacts","cache","resources","deploy":default:return m,false,fmt.Errorf("yevqo.yml:%d: unsupported section %q",line,k)}
   section=k;listKey="";continue
  }
  if ind==4{
   if !strings.HasPrefix(v,"- ")||listKey==""{return m,false,fmt.Errorf("yevqo.yml:%d: expected list item",line)}
   x:=strings.Trim(strings.TrimSpace(strings.SplitN(strings.TrimPrefix(v,"- ")," #",2)[0]),"\"'");if x==""{return m,false,fmt.Errorf("yevqo.yml:%d: empty list item",line)}
   switch listKey{case "artifacts.include":m.ArtifactsInclude=append(m.ArtifactsInclude,x);case "artifacts.exclude":m.ArtifactsExclude=append(m.ArtifactsExclude,x);case "cache.paths":m.CachePaths=append(m.CachePaths,x)}
   continue
  }
  if ind!=2{return m,false,fmt.Errorf("yevqo.yml:%d: invalid indentation",line)}
  p:=strings.SplitN(v,":",2);if len(p)!=2{return m,false,fmt.Errorf("yevqo.yml:%d: expected key: value",line)};k,val:=strings.TrimSpace(p[0]),strings.TrimSpace(p[1])
  if section=="environment"{if !validYevqoEnvironmentKey(k){return m,false,fmt.Errorf("invalid or reserved environment key %q",k)};if _,ok:=m.Environment[k];ok{return m,false,fmt.Errorf("duplicate environment key %q",k)};m.Environment[k]=strings.Trim(strings.TrimSpace(strings.SplitN(val," #",2)[0]),"\"'");continue}
  full:=section+"."+k;if seen[full]{return m,false,fmt.Errorf("yevqo.yml:%d: duplicate key %q",line,full)};seen[full]=true;val=strings.Trim(strings.TrimSpace(strings.SplitN(val," #",2)[0]),"\"'")
  switch full{
  case "project.name":m.Name=val
  case "project.runtime":m.Runtime=val
  case "project.root":m.Root=val
  case "build.provider":m.Provider=val
  case "build.install":m.Install=val
  case "build.command":m.Build=val
  case "build.output":m.Output=val
  case "run.command":m.Start=val
  case "run.port":n,e:=strconv.Atoi(val);if e!=nil||n<1||n>65535{return m,false,fmt.Errorf("run.port must be 1..65535")};m.Port=n
  case "healthcheck.path":m.HealthcheckPath=val
  case "healthcheck.interval":d,e:=time.ParseDuration(val);if e!=nil||d<time.Second||d>time.Minute{return m,false,fmt.Errorf("healthcheck.interval must be 1s..1m")};m.HealthcheckInterval=d
  case "resources.memory":n,e:=parseYevqoMemory(val);if e!=nil{return m,false,e};m.MemoryBytes=n
  case "deploy.restart":x,e:=strconv.ParseBool(val);if e!=nil{return m,false,fmt.Errorf("deploy.restart must be boolean")};m.DeployRestart=x;m.DeployRestartSet=true
  case "cache.enabled":x,e:=strconv.ParseBool(val);if e!=nil{return m,false,fmt.Errorf("cache.enabled must be boolean")};m.CacheEnabled=x
  case "artifacts.include","artifacts.exclude","cache.paths":if val!=""{return m,false,fmt.Errorf("%s must be a YAML list",full)};listKey=full
  default:return m,false,fmt.Errorf("yevqo.yml:%d: unsupported key %q",line,full)
  }
 }
 if e:=sc.Err();e!=nil{return m,false,e};if !version{return m,false,fmt.Errorf("yevqo.yml must declare version: 1")};if len(seen)==0&&len(m.Environment)==0{return m,false,fmt.Errorf("yevqo.yml has no configuration")}
 if m.HealthcheckPath!=""&&(!validHealthCheckPath(m.HealthcheckPath)||strings.ContainsAny(m.HealthcheckPath,"?#")){return m,false,fmt.Errorf("healthcheck.path must be an absolute path without query or fragment")}
 for _,p:=range append(append(append(append([]string{},m.CachePaths...),m.ArtifactsInclude...),m.ArtifactsExclude...),[]string{m.Output}...){if p==""{continue};if filepath.IsAbs(p)||strings.Contains(p,"\\")||filepath.Clean(p)=="."||filepath.Clean(p)==".."||strings.HasPrefix(filepath.Clean(p),".."+string(filepath.Separator)){return m,false,fmt.Errorf("yevqo.yml paths must stay inside project root")}}
 return m,true,nil
}
func validYevqoEnvironmentKey(k string)bool{if k==""||len(k)>64{return false};for i,r:=range k{if !((r>='A'&&r<='Z')||r=='_'||(i>0&&r>='0'&&r<='9')){return false}};switch k{case "PATH","HOME","PREFIX","TMPDIR","LD_PRELOAD","LD_LIBRARY_PATH","PORT","HOST":return false};return !strings.HasPrefix(k,"DB_")&&!strings.HasPrefix(k,"VELTRIX_")}
func parseYevqoMemory(raw string)(uint64,error){v:=strings.ToUpper(strings.TrimSpace(raw));var mult uint64=1;for suffix,factor:=range map[string]uint64{"KIB":1<<10,"MIB":1<<20,"GIB":1<<30,"KB":1000,"MB":1000000,"GB":1000000000}{if strings.HasSuffix(v,suffix){mult=factor;v=strings.TrimSpace(strings.TrimSuffix(v,suffix));break}};n,e:=strconv.ParseUint(v,10,64);if e!=nil||n==0||n>^uint64(0)/mult{return 0,fmt.Errorf("resources.memory must be a positive size, e.g. 512MiB")};return n*mult,nil}

func analyzeProjectMarkers(root string) (BuildPlan, error) {
	abs, err := filepath.Abs(root)
	if err != nil { return BuildPlan{}, fmt.Errorf("resolve project root: %w", err) }
	info, err := os.Stat(abs)
	if err != nil { return BuildPlan{}, fmt.Errorf("inspect project root: %w", err) }
	if !info.IsDir() { return BuildPlan{}, fmt.Errorf("project root is not a directory") }
	entries, err := os.ReadDir(abs)
	if err != nil { return BuildPlan{}, fmt.Errorf("read project root: %w", err) }
	files := make(map[string]bool, len(entries))
	for _, entry := range entries {
		// Symlinks and directories are deliberately not treated as marker files.
		if entry.Type().IsRegular() { files[strings.ToLower(entry.Name())] = true }
	}
	has := func(names ...string) bool { for _, name := range names { if files[strings.ToLower(name)] { return true } }; return false }
	androidEnv := DetectAndroidBuildEnvironment("")
	provider := "auto"
	if androidEnv.Android || androidEnv.Termux { provider = "android-local" }
	plan := BuildPlan{Provider:provider, Stack:"unknown", Root:".", Confidence:"low", NeedsConfig:true, Evidence:[]string{}, Capabilities: DetectBuildCapabilities(), AndroidEnvironment: androidEnv}

	if has("dockerfile") {
		plan.Stack, plan.Confidence = "container", "high"
		plan.Evidence = append(plan.Evidence, "Dockerfile")
		if androidEnv.Android || androidEnv.Termux {
			plan.NeedsConfig = true
			plan.Warnings = append(plan.Warnings, "Dockerfile execution is disabled in the Android-only Build Engine.")
		} else {
			plan.Provider, plan.NeedsConfig = "dockerfile", false
		}
		return plan, nil
	}
	if has("railpack.json") {
		plan.Stack, plan.Confidence = "detected-by-railpack", "high"
		plan.Evidence = append(plan.Evidence, "railpack.json")
		plan.Provider, plan.NeedsConfig = "railpack", true
		plan.Warnings = append(plan.Warnings, "Railpack builds require a registered remote Linux build executor; they are not run on Android/Termux.")
		return plan, nil
	}
	if has("nixpacks.toml") {
		plan.Stack, plan.Confidence = "detected-by-nixpacks", "high"
		plan.Evidence = append(plan.Evidence, "nixpacks.toml")
		if androidEnv.Android || androidEnv.Termux {
			plan.NeedsConfig = true
			plan.Warnings = append(plan.Warnings, "Nixpacks execution is disabled in the Android-only Build Engine.")
		} else {
			plan.Provider, plan.NeedsConfig = "nixpacks", false
		}
		return plan, nil
	}
	if has("package.json") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "nodejs", "npm", "high", false
		switch {
		case has("bun.lockb", "bun.lock"): plan.PackageTool = "bun"
		case has("pnpm-lock.yaml"): plan.PackageTool = "pnpm"
		case has("yarn.lock"): plan.PackageTool = "yarn"
		}
		plan.Install = map[string]string{"npm":"npm install --no-audit --no-fund","pnpm":"pnpm install --no-frozen-lockfile","yarn":"yarn install --non-interactive","bun":"bun install"}[plan.PackageTool]
		if plan.PackageTool == "npm" && has("package-lock.json", "npm-shrinkwrap.json") { plan.Install = "npm ci" }
		if plan.PackageTool == "pnpm" && has("pnpm-lock.yaml") { plan.Install = "pnpm install --frozen-lockfile" }
		if plan.PackageTool == "yarn" && has("yarn.lock") { plan.Install = "yarn install --frozen-lockfile" }
		if plan.PackageTool == "bun" && has("bun.lock", "bun.lockb") { plan.Install = "bun install --frozen-lockfile" }
		packageRunner := plan.PackageTool
		if packageRunner == "" { packageRunner = "npm" }
		plan.Start = packageRunner + " run start"
		if packageData, readErr := os.ReadFile(filepath.Join(abs, "package.json")); readErr == nil {
			var packageManifest struct { Scripts map[string]string `json:"scripts"` }
			if jsonErr := json.Unmarshal(packageData, &packageManifest); jsonErr == nil {
				if strings.TrimSpace(packageManifest.Scripts["build"]) != "" { plan.Build = packageRunner + " run build" }
				if strings.TrimSpace(packageManifest.Scripts["start"]) == "" { plan.Start = "" }
			}
		}
		plan.Port = 3000
		plan.Evidence = append(plan.Evidence, "package.json", plan.PackageTool+" lockfile/package manager")
		return plan, nil
	}
	if has("requirements.txt", "pyproject.toml", "Pipfile") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "python", "pip", "high", false
		plan.Install, plan.Start, plan.Port = "python -m pip install -r requirements.txt", "python app.py", 8000
		if has("pyproject.toml") { plan.Evidence = append(plan.Evidence, "pyproject.toml"); plan.Install = "python -m pip install ." }
		if has("requirements.txt") { plan.Evidence = append(plan.Evidence, "requirements.txt") }
		if has("manage.py") { plan.Start = "python manage.py runserver 0.0.0.0:8000" }
		if has("main.py") { plan.Start = "python main.py" }
		return plan, nil
	}
	if has("cargo.toml") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "rust", "cargo", "high", false
		plan.Build, plan.Start, plan.Port = "cargo build --release", "./target/release/<binary>", 3000
		plan.Evidence = append(plan.Evidence, "Cargo.toml")
		return plan, nil
	}
	if has("go.mod") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "go", "go", "high", false
		plan.Build, plan.Start, plan.Port = "go build -o yevqo-app .", "./yevqo-app", 8080
		plan.Evidence = append(plan.Evidence, "go.mod")
		return plan, nil
	}
	if has("pom.xml") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "java", "maven", "high", false
		plan.Build, plan.Start, plan.Port = "mvn package", "java -jar <built-jar>", 8080
		if has("mvnw") { plan.Build = "./mvnw package" }
		plan.Evidence = append(plan.Evidence, "pom.xml")
		return plan, nil
	}
	if has("build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "jvm", "gradle", "high", false
		plan.Build, plan.Start, plan.Port = "./gradlew build", "java -jar <built-jar>", 8080
		plan.Evidence = append(plan.Evidence, "Gradle build file")
		return plan, nil
	}
	if has("gemfile") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "ruby", "bundler", "high", false
		plan.Install, plan.Start, plan.Port = "bundle install", "bundle exec rackup -o 0.0.0.0", 9292
		plan.Evidence = append(plan.Evidence, "Gemfile")
		return plan, nil
	}
	if has("composer.json") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "php", "composer", "high", false
		plan.Install, plan.Start, plan.Port = "composer install --no-interaction --prefer-dist", "php -S 0.0.0.0:8080 -t public", 8080
		plan.Evidence = append(plan.Evidence, "composer.json")
		return plan, nil
	}
	if has(".csproj", "global.json") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "dotnet", "dotnet", "high", false
		plan.Build, plan.Start, plan.Port = "dotnet publish -c Release -o ./publish", "dotnet ./publish/<application>.dll", 8080
		plan.Evidence = append(plan.Evidence, ".csproj/global.json")
		return plan, nil
	}
	// Additional ecosystems are detected by their native project manifests.
	// Each plan uses fixed argv commands and requires the corresponding Termux tool.
	if has("cmakelists.txt") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "cmake", "cmake", "high", false
		plan.Install, plan.Build = "cmake -S . -B build", "cmake --build build --parallel 2"
		plan.Evidence = append(plan.Evidence, "CMakeLists.txt")
		return plan, nil
	}
	if has("makefile", "gnumakefile") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "make", "make", "high", false
		plan.Build = "make -j2"
		plan.Evidence = append(plan.Evidence, "Makefile")
		return plan, nil
	}
	if has("package.swift") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "swift", "swift", "high", false
		plan.Build = "swift build -c release"
		plan.Evidence = append(plan.Evidence, "Package.swift")
		return plan, nil
	}
	if has("mix.exs") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "elixir", "mix", "high", false
		plan.Install, plan.Build = "mix deps.get", "mix compile"
		plan.Evidence = append(plan.Evidence, "mix.exs")
		return plan, nil
	}
	if has("build.sbt") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "scala", "sbt", "high", false
		plan.Build = "sbt compile"
		plan.Evidence = append(plan.Evidence, "build.sbt")
		return plan, nil
	}
	if has("build.zig") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "zig", "zig", "high", false
		plan.Build = "zig build -Doptimize=ReleaseFast"
		plan.Evidence = append(plan.Evidence, "build.zig")
		return plan, nil
	}
	if has("pubspec.yaml") {
		if _, statErr := os.Stat(filepath.Join(abs, "bin", "main.dart")); statErr == nil {
			plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "dart", "dart", "high", false
			plan.Install, plan.Build = "dart pub get", "dart compile exe bin/main.dart -o yevqo-app"
			plan.Evidence = append(plan.Evidence, "pubspec.yaml", "bin/main.dart")
			return plan, nil
		}
	}
	if has("deno.json", "deno.jsonc") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "deno", "deno", "high", false
		plan.Build = "deno task build"
		plan.Evidence = append(plan.Evidence, "deno.json")
		return plan, nil
	}
	if has("rebar.config") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "erlang", "rebar3", "high", false
		plan.Build = "rebar3 compile"
		plan.Evidence = append(plan.Evidence, "rebar.config")
		return plan, nil
	}
	if has("dune-project") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "ocaml", "dune", "high", false
		plan.Build = "dune build"
		plan.Evidence = append(plan.Evidence, "dune-project")
		return plan, nil
	}
	if has("shard.yml") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "crystal", "shards", "high", false
		plan.Install, plan.Build = "shards install", "crystal build src/main.cr -o yevqo-app"
		plan.Evidence = append(plan.Evidence, "shard.yml")
		return plan, nil
	}
	if has("cpanfile") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "perl", "cpanm", "high", false
		plan.Install = "cpanm --installdeps ."
		plan.Evidence = append(plan.Evidence, "cpanfile")
		return plan, nil
	}
	for name := range files {
		if strings.HasSuffix(name, ".cabal") {
			plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "haskell", "cabal", "high", false
			plan.Build = "cabal build"
			plan.Evidence = append(plan.Evidence, name)
			return plan, nil
		}
		if strings.HasSuffix(name, ".nimble") {
			plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "nim", "nimble", "high", false
			plan.Build = "nimble build -y"
			plan.Evidence = append(plan.Evidence, name)
			return plan, nil
		}
		if strings.HasSuffix(name, ".rockspec") {
			plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "lua", "luarocks", "high", false
			plan.Build = "luarocks make"
			plan.Evidence = append(plan.Evidence, name)
			return plan, nil
		}
	}
	if has("project.toml") && has("manifest.toml") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "julia", "julia", "medium", true
		plan.Warnings = append(plan.Warnings, "Julia project detected; configure explicit install/build commands for the selected package.")
		plan.Evidence = append(plan.Evidence, "Project.toml", "Manifest.toml")
		return plan, nil
	}
	if has("index.html") {
		plan.Provider, plan.Stack, plan.Confidence, plan.NeedsConfig = "static", "static", "medium", false
		plan.Evidence = append(plan.Evidence, "index.html")
		return plan, nil
	}

	if androidEnv.Android || androidEnv.Termux {
		plan.Warnings = append(plan.Warnings, "No supported Android local build markers found. Docker, Podman, Nixpacks and arbitrary shell commands are disabled.")
	} else {
		plan.Warnings = append(plan.Warnings, "No known project markers found. Configure a supported build provider.")
	}
	return plan, nil
}
