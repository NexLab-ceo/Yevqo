package agent

import (
	"bufio"
	"fmt"
	"strconv"
	"os"
	"path/filepath"
	"strings"
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
	Capabilities BuildCapabilities `json:"capabilities"`
	AndroidEnvironment AndroidBuildEnvironment `json:"android_environment"`
}

// AnalyzeProject inspects only the immediate project root. It does not execute
// scripts, parse arbitrary build plugins, or follow symlinks.
type veltrixManifest struct { Root, Provider, Install, Build, Start string; Port int }

// AnalyzeProject honors a small, explicit veltrix.yaml subset before auto-detection.
func AnalyzeProject(root string) (BuildPlan, error) {
 abs, err := filepath.Abs(root)
 if err != nil { return BuildPlan{}, fmt.Errorf("resolve project root: %w", err) }
 manifest, found, err := readVeltrixManifest(abs)
 if err != nil { return BuildPlan{}, err }
 if !found { return analyzeProjectMarkers(abs) }
 if manifest.Provider != "" {
  switch manifest.Provider { case "auto","veltrix","custom","static","android-local","dockerfile","nixpacks","railpack": default: return BuildPlan{}, fmt.Errorf("unsupported veltrix.yaml build.provider %q", manifest.Provider) }
 }
 manifestRoot := manifest.Root
 if manifestRoot == "" { manifestRoot = "." }
 if filepath.IsAbs(manifestRoot) || strings.Contains(manifestRoot, "\\\\") { return BuildPlan{}, fmt.Errorf("veltrix.yaml project.root must be relative") }
 clean := filepath.Clean(manifestRoot)
 if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) { return BuildPlan{}, fmt.Errorf("veltrix.yaml project.root escapes project directory") }
 projectRoot, err := filepath.EvalSymlinks(filepath.Join(abs, clean))
 if err != nil { return BuildPlan{}, fmt.Errorf("resolve veltrix.yaml project.root: %w", err) }
 rel, err := filepath.Rel(abs, projectRoot)
 if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return BuildPlan{}, fmt.Errorf("veltrix.yaml project.root resolves outside project directory") }
 plan, err := analyzeProjectMarkers(projectRoot)
 if err != nil { return BuildPlan{}, err }
 plan.Root = clean
 plan.Evidence = append(plan.Evidence, "veltrix.yaml")
 if manifest.Provider != "" && manifest.Provider != "auto" {
  plan.Provider = manifest.Provider
  if (manifest.Provider == "dockerfile" || manifest.Provider == "nixpacks" || manifest.Provider == "railpack") && (plan.AndroidEnvironment.Android || plan.AndroidEnvironment.Termux) {
   plan.NeedsConfig = true
   plan.Warnings = append(plan.Warnings, manifest.Provider+" builds are disabled on the Android local provider")
  }
 }
 if manifest.Provider == "static" { plan.Stack = "static"; plan.NeedsConfig = false }
 if manifest.Install != "" { plan.Install = manifest.Install }
 if manifest.Build != "" { plan.Build = manifest.Build }
 if manifest.Start != "" { plan.Start = manifest.Start }
 if manifest.Port > 0 { plan.Port = manifest.Port }
 if plan.Stack == "unknown" && (manifest.Install != "" || manifest.Build != "") { plan.Stack = "custom"; plan.NeedsConfig = false }
 if manifest.Install != "" || manifest.Build != "" { plan.NeedsConfig = false }
 return plan, nil
}

func readVeltrixManifest(root string) (veltrixManifest, bool, error) {
 var m veltrixManifest
 path := filepath.Join(root, "veltrix.yaml")
 if _, err := os.Stat(path); err != nil {
  if os.IsNotExist(err) { path = filepath.Join(root, "veltrix.yml"); if _, e := os.Stat(path); os.IsNotExist(e) { return m, false, nil } else if e != nil { return m, false, e } } else { return m, false, err }
 }
 f, err := os.Open(path); if err != nil { return m, false, err }; defer f.Close()
 scanner := bufio.NewScanner(f); scanner.Buffer(make([]byte, 4096), 64*1024)
 section := ""
 for scanner.Scan() {
  line := strings.TrimSpace(scanner.Text())
  if line == "" || strings.HasPrefix(line, "#") || line == "version: 1" { continue }
  if !strings.Contains(line, ":") { continue }
  parts := strings.SplitN(line, ":", 2)
  key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
  if value == "" && !strings.HasPrefix(scanner.Text(), " ") { section = key; continue }
  value = strings.TrimSpace(strings.SplitN(value, " #", 2)[0])
  value = strings.Trim(value, "\"'")
  switch section {
  case "project": if key == "root" { m.Root = value }
  case "build": switch key { case "provider": m.Provider = value; case "install": m.Install = value; case "command": m.Build = value }
  case "run": switch key { case "command": m.Start = value; case "port": n, e := strconv.Atoi(value); if e != nil || n < 1 || n > 65535 { return m, false, fmt.Errorf("veltrix.yaml run.port must be between 1 and 65535") }; m.Port = n }
  }
 }
 if err := scanner.Err(); err != nil { return m, false, err }
 return m, true, nil
}

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
	if has("railpack.json") {\n\t\tplan.Stack, plan.Confidence = "detected-by-railpack", "high"\n\t\tplan.Evidence = append(plan.Evidence, "railpack.json")\n\t\tplan.Provider, plan.NeedsConfig = "railpack", true\n\t\tplan.Warnings = append(plan.Warnings, "Railpack builds require a registered remote Linux build executor; they are not run on Android/Termux.")\n\t\treturn plan, nil\n\t}\n\tif has("nixpacks.toml") {
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
		plan.Install = map[string]string{"npm":"npm ci","pnpm":"pnpm install --frozen-lockfile","yarn":"yarn install --frozen-lockfile","bun":"bun install --frozen-lockfile"}[plan.PackageTool]
		plan.Build, plan.Start = "npm run build", "npm run start"
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
		plan.Build, plan.Start, plan.Port = "go build -o ./bin/app .", "./bin/app", 8080
		plan.Evidence = append(plan.Evidence, "go.mod")
		return plan, nil
	}
	if has("pom.xml") {
		plan.Stack, plan.PackageTool, plan.Confidence, plan.NeedsConfig = "java", "maven", "high", false
		plan.Build, plan.Start, plan.Port = "mvn -DskipTests package", "java -jar <built-jar>", 8080
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
