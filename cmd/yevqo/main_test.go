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
				w.Header().Set("Content-Type", "application/json")
				if command == "logs" {
					_, _ = w.Write([]byte(`{"ok":true,"payload":{"content":"log line\n"}}`))
				} else {
					_, _ = w.Write([]byte(`{"ok":true,"message":"done"}`))
				}
			}))
			defer server.Close()
			var stdout, stderr strings.Builder
			code := run([]string{"--server",server.URL,"--session","test-session",command,"12","demo"}, func(string)string{return ""}, &stdout, &stderr)
			if code != 0 { t.Fatalf("exit=%d stderr=%s",code,stderr.String()) }
			if !called { t.Fatal("Control Plane was not called") }
			if command == "logs" && !strings.Contains(stdout.String(),"log line") { t.Fatalf("logs output missing: %q",stdout.String()) }
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
