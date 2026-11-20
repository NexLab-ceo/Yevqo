package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCLICommandsCallAuthenticatedControlPlane(t *testing.T) {
	for _, command := range []string{"deploy", "stop", "logs"} {
		t.Run(command, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if cookie, err := r.Cookie("veltrix_session"); err != nil || cookie.Value != "test-session" {
					t.Fatalf("session cookie missing: %v", err)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil { t.Fatal(err) }
				if command == "logs" {
					if body["action"] != "get_logs" { t.Fatalf("unexpected logs action: %#v", body) }
				} else if body["action"] != command {
					t.Fatalf("unexpected action: %#v", body)
				}
				if command == "deploy" && body["allow_project_code"] != true { t.Fatalf("deploy consent flag missing: %#v", body) }
				w.Header().Set("Content-Type", "application/json")
				if command == "logs" {
					_, _ = w.Write([]byte(`{"ok":true,"payload":"eyJjb250ZW50IjoibG9nIGxpbmVcbiJ9"}`))
				} else {
					_, _ = w.Write([]byte(`{"ok":true,"message":"done"}`))
				}
			}))
			defer server.Close()
			var stdout, stderr strings.Builder
			args := []string{"--server",server.URL,"--session","test-session",command,"12","demo"}
			if command == "deploy" { args = append(args,"--allow-project-code") }
			code := run(args, func(string)string{return ""}, &stdout, &stderr)
			if code != 0 { t.Fatalf("exit=%d stderr=%s",code,stderr.String()) }
			if !called { t.Fatal("Control Plane was not called") }
			if command == "logs" && !strings.Contains(stdout.String(),"log line") { t.Fatalf("logs output missing: %q",stdout.String()) }
		})
	}
}

func TestCLIInventoryAndBackupCommands(t *testing.T) {
	for _, command := range []string{"projects", "devices", "sites", "databases", "database-servers", "backups", "backup-schedules", "deployments", "monitoring"} {
		t.Run(command,func(t *testing.T){
			called:=false
			server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
				called=true
				if r.Method!=http.MethodGet { t.Fatalf("method=%s, want GET",r.Method) }
				if _,err:=r.Cookie("veltrix_session");err!=nil{t.Fatalf("session cookie missing: %v",err)}
				w.Header().Set("Content-Type","application/json")
				_,_=w.Write([]byte(`{"ok":true,"items":[]}`))
			}))
			defer server.Close()
			var stdout,stderr strings.Builder
			if code:=run([]string{"--server",server.URL,"--session","test-session",command},func(string)string{return ""},&stdout,&stderr);code!=0{t.Fatalf("exit=%d stderr=%s",code,stderr.String())}
			if !called||!strings.Contains(stdout.String(),"\"ok\""){t.Fatalf("request/output missing: called=%v output=%q",called,stdout.String())}
		})
	}
	for _,tc:=range []struct{command string;args []string;want map[string]any}{
		{"backup-site",[]string{"12","demo"},map[string]any{"device_id":float64(12),"site_id":"demo"}},
		{"backup-db",[]string{"42"},map[string]any{"database_id":float64(42)}},
	}{
		t.Run(tc.command,func(t *testing.T){
			called:=false
			server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
				called=true
				if r.URL.Path!="/api/backups"||r.Method!=http.MethodPost{t.Fatalf("unexpected request: %s %s",r.Method,r.URL.Path)}
				var body map[string]any
				if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Fatal(err)}
				for key,want:=range tc.want{if body[key]!=want{t.Fatalf("%s=%#v, want %#v",key,body[key],want)}}
				w.Header().Set("Content-Type","application/json")
				_,_=w.Write([]byte(`{"ok":true,"message":"backup queued"}`))
			}))
			defer server.Close()
			var stdout,stderr strings.Builder
			args:=append([]string{"--server",server.URL,"--session","test-session",tc.command},tc.args...)
			if code:=run(args,func(string)string{return ""},&stdout,&stderr);code!=0{t.Fatalf("exit=%d stderr=%s",code,stderr.String())}
			if !called||!strings.Contains(stdout.String(),"backup queued"){t.Fatalf("request/output missing: called=%v output=%q",called,stdout.String())}
		})
	}
}

func TestCLIRequiresSessionAndRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr strings.Builder
	if code:=run([]string{"--server","http://localhost:8080","deploy","1","site"},func(string)string{return ""},&stdout,&stderr);code==0 {
		t.Fatal("missing session unexpectedly succeeded")
	}
	stderr.Reset()
	if code:=run([]string{"--server","http://localhost:8080","--session","x","unknown","1","site"},func(string)string{return ""},&stdout,&stderr);code!=2 {
		t.Fatalf("unknown command exit=%d, want 2",code)
	}
}

func TestCLIDeployRequiresExplicitProjectCodeConsent(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ called=true }))
	defer server.Close()
	var stdout,stderr strings.Builder
	code:=run([]string{"--server",server.URL,"--session","test-session","deploy","12","demo"},func(string)string{return ""},&stdout,&stderr)
	if code!=2 || called || !strings.Contains(stderr.String(),"--allow-project-code") {
		t.Fatalf("deploy without consent: exit=%d called=%v stderr=%q",code,called,stderr.String())
	}
}
