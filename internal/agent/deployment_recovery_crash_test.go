package agent

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestDeploymentRecoveryCrashBoundaries exercises the durable states that can
// be observed when the Agent is killed between deployment activation steps.
// Each subtest starts a fresh manager, so recovery follows the same startup path
// used after a real Agent restart.
func TestDeploymentRecoveryCrashBoundaries(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for previous-release recovery")
	}

	for _, point := range []string{
		"journal-written-before-stop",
		"previous-stopped-before-candidate",
		"candidate-recorded-before-commit",
		"commit-recorded-before-journal-cleanup",
	} {
		t.Run(point, func(t *testing.T) {
			root := t.TempDir()
			siteRoot := filepath.Join(root, "sites", "demo")
			oldRelease := filepath.Join(siteRoot, ".yevqo", "releases", "old")
			if err := osMkdirAll(oldRelease); err != nil { t.Fatal(err) }

			portListener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil { t.Fatal(err) }
			port := portListener.Addr().(*net.TCPAddr).Port
			_ = portListener.Close()

			registry, err := NewProcessRegistry(root)
			if err != nil { t.Fatal(err) }

			startHelper := func(command string, args ...string) (*exec.Cmd, ProcessRecord) {
				cmd := exec.Command(command, args...)
				cmd.SysProcAttr = procAttr()
				if err := cmd.Start(); err != nil { t.Fatalf("start helper process: %v", err) }
				if command == "python3" { if err := waitForTCP(port, 5*time.Second); err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatalf("wait for previous release: %v", err) } }
				token, err := processStartToken(cmd.Process.Pid)
				if err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
				return cmd, ProcessRecord{SiteID:"demo", PID:cmd.Process.Pid, ProcessGroup:cmd.Process.Pid,
					Port:port, Runtime:"static", WorkingDir:oldRelease, LogPath:filepath.Join(root,"demo.log"),
					StartedAt:time.Now().UTC(), StartToken:token,
					RunPath:filepath.Join("sites","demo",".yevqo","releases","old"),
					StartCommand:fmt.Sprintf("python3 -m http.server %d --bind 127.0.0.1",port)}
			}

			previousCmd, previous := startHelper("python3", "-m", "http.server", fmt.Sprint(port), "--bind", "127.0.0.1")
			defer func(){ if previousCmd.Process != nil { _ = previousCmd.Process.Kill(); _ = previousCmd.Wait() } }()
			candidateCmd, candidate := startHelper("sh", "-c", "sleep 60")
			defer func(){ if candidateCmd.Process != nil { _ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait() } }()

			tx := deploymentTransaction{Version:1, SiteID:"demo", State:"activating",
				CandidateCommit:"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", HadPrevious:true, Previous:previous}

			switch point {
			case "journal-written-before-stop":
				if err := registry.Save(previous); err != nil { t.Fatal(err) }
				_ = candidateCmd.Process.Kill()
				_ = candidateCmd.Wait()
			case "previous-stopped-before-candidate":
				if err := registry.Save(previous); err != nil { t.Fatal(err) }
				_ = previousCmd.Process.Kill()
				_ = previousCmd.Wait()
				_ = candidateCmd.Process.Kill()
				_ = candidateCmd.Wait()
			case "candidate-recorded-before-commit":
				if err := registry.Save(candidate); err != nil { t.Fatal(err) }
				_ = previousCmd.Process.Kill()
				_ = previousCmd.Wait()
			case "commit-recorded-before-journal-cleanup":
				if err := registry.Save(candidate); err != nil { t.Fatal(err) }
				_ = previousCmd.Process.Kill()
				_ = previousCmd.Wait()
				tx.State = "committed"
			}

			manager := &ProcessManager{DataRoot:root, registry:registry, procs:make(map[string]*Process),
				intentionalStops:make(map[string]bool), restartAttempts:make(map[string]int)}
			if err := manager.createDeploymentTransaction(tx); err != nil { t.Fatal(err) }

			restarted := NewProcessManager(root)
			got, exists := restarted.registry.Get("demo")
			if !exists { t.Fatal("startup recovery left the site without a durable process record") }
			if point == "commit-recorded-before-journal-cleanup" {
				if got.PID != candidate.PID { t.Fatalf("committed candidate was not retained: got pid %d want %d",got.PID,candidate.PID) }
			} else {
				if got.RunPath != previous.RunPath || got.PID == candidate.PID {
					t.Fatalf("last confirmed release was not restored: %#v",got)
				}
				if !verifyProcessRecord(got) || !restarted.IsSiteHealthy("demo") {
					t.Fatal("restored previous release is not healthy")
				}
				if err := restarted.StopSite("demo"); err != nil { t.Fatal(err) }
			}
			if _, exists, err := restarted.readDeploymentTransaction("demo"); err != nil || exists {
				t.Fatalf("recovery journal should be finalized: exists=%v err=%v",exists,err)
			}
			if point == "commit-recorded-before-journal-cleanup" {
				if !verifyProcessRecord(got) { t.Fatal("committed candidate process identity was not preserved") }
				if err := restarted.StopSite("demo"); err != nil { t.Fatal(err) }
			}
		})
	}
}

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o700)
}
