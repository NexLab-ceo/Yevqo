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
	if records[0].OutputBytes == 0 || records[len(records)-1].OutputBytes == 0 {
		t.Fatal("history metadata should include log byte counts")
	}
	newest, err := GetBuildHistory(root, "history-demo", records[0].ID)
	if err != nil { t.Fatal(err) }
	oldest, err := GetBuildHistory(root, "history-demo", records[len(records)-1].ID)
	if err != nil { t.Fatal(err) }
	if newest.Result.Output != "build-12" || oldest.Result.Output != "build-03" {
		t.Fatalf("history order/retention failed: newest=%q oldest=%q", newest.Result.Output, oldest.Result.Output)
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
 if len(body.Records)!=1||body.Records[0].Result.Output!=""||body.Records[0].OutputBytes==0{t.Fatalf("history listing should return metadata only: %#v",body)}
 detailPayload,err:=shared.Marshal(shared.GetBuildHistory{SiteId:"command-demo",ID:body.Records[0].ID})
 if err!=nil{t.Fatal(err)}
 detail:=handler.HandleCommand(&shared.Command{Id:"history-2",Action:"get_build_history",Payload:detailPayload})
 if !detail.Ok||len(detail.Payload)==0{t.Fatalf("history detail command failed: %#v",detail)}
 var full BuildHistoryRecord
 if err:=shared.Unmarshal(detail.Payload,&full);err!=nil{t.Fatal(err)}
 if full.Result.Output!="nim build passed"{t.Fatalf("history detail log missing: %#v",full)}
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
