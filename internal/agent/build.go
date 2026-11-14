package agent

import (
	"fmt"
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
}

// AnalyzeProject inspects only the immediate project root. It does not execute
// scripts, parse arbitrary build plugins, or follow symlinks.
func AnalyzeProject(root string) (BuildPlan, error) {
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
	plan := BuildPlan{Provider:"auto", Stack:"unknown", Root:".", Confidence:"low", NeedsConfig:true, Evidence:[]string{}}

	// Explicit configuration always wins over heuristic detection.
	if has("veltrix.yaml", "veltrix.yml") {
		plan.Provider, plan.Confidence = "veltrix", "high"
		plan.Evidence = append(plan.Evidence, "veltrix.yaml")
		plan.Warnings = append(plan.Warnings, "Manifest parsing is handled by the VELTRIX configuration layer; detection does not execute commands.")
		return plan, nil
	}
	if has("dockerfile") {
		plan.Provider, plan.Stack, plan.Confidence, plan.NeedsConfig = "dockerfile", "container", "high", false
		plan.Evidence = append(plan.Evidence, "Dockerfile")
		return plan, nil
	}
	if has("nixpacks.toml") {
		plan.Provider, plan.Stack, plan.Confidence, plan.NeedsConfig = "nixpacks", "detected-by-nixpacks", "high", false
		plan.Evidence = append(plan.Evidence, "nixpacks.toml")
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

	plan.Warnings = append(plan.Warnings, "No known project markers found. Choose Dockerfile, Nixpacks, Static Hosting, or provide custom install/build/start commands.")
	return plan, nil
}
