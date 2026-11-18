package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"veltrix/internal/shared"
)

func TestBuildHistoryPersistsNewestRecordsWithRetention(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < buildHistoryRetention+3; i++ {
		finished := time.Date(2026, 11, 18, 10, 0, i, 0, time.UTC)
		result := AndroidBuildResult{
			SiteID: "history-demo", Provider: "android-local", Stack: "custom",
			Success: i%2 == 0, StartedAt: finished.Add(-time.Second), FinishedAt: finished,
			DurationMillis: 1000, Output: fmt.Sprintf("build-%02d", i),
		}
		if err := persistBuildHistory(root, "history-demo", result); err != nil {
			t.Fatalf("persist build %d: %v", i, err)
		}
	}
	records, err := ListBuildHistory(root, "history-demo", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != buildHistoryRetention {
		t.Fatalf("history contains %d records, want %d", len(records), buildHistoryRetention)
	}
	if records[0].Result.Output != "build-12" || records[len(records)-1].Result.Output != "build-03" {
		t.Fatalf("history is not newest-first or retention failed: newest=%q oldest=%q", records[0].Result.Output, records[len(records)-1].Result.Output)
	}
	dir := filepath.Join(root, "build-history", "history-demo")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("history directory permissions are too broad: %o", info.Mode().Perm())
	}
}


func TestListBuildHistoryAgentCommand(t *testing.T) {
 root:=t.TempDir()
 result:=AndroidBuildResult{SiteID:"command-demo",Provider:"android-local",Stack:"nim",Success:true,FinishedAt:time.Now().UTC(),Output:"nim build passed"}
 if err:=persistBuildHistory(root,"command-demo",result);err!=nil{t.Fatal(err)}
 payload,err:=shared.Marshal(shared.ListBuildHistory{SiteId:"command-demo",Limit:5})
 if err!=nil{t.Fatal(err)}
 handler:=NewCommandHandler(&Config{DataRoot:root})
 response:=handler.HandleCommand(&shared.Command{Id:"history-1",Action:"list_build_history",Payload:payload})
 if !response.Ok||len(response.Payload)==0{t.Fatalf("history command failed: %#v",response)}
 var body struct{Records []BuildHistoryRecord `json:"records"`}
 if err:=shared.Unmarshal(response.Payload,&body);err!=nil{t.Fatal(err)}
 if len(body.Records)!=1||body.Records[0].Result.Output!="nim build passed"{t.Fatalf("unexpected history response: %#v",body)}
}

func TestBuildHistoryRejectsInvalidSiteAndSymlink(t *testing.T) {
	root := t.TempDir()
	if _, err := ListBuildHistory(root, "../escape", 10); err == nil {
		t.Fatal("invalid site ID was accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "build-history")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := persistBuildHistory(root, "safe-site", AndroidBuildResult{SiteID: "safe-site"}); err == nil {
		t.Fatal("symlinked history root was accepted")
	}
}
