package control

import (
 "encoding/json"
 "testing"

 "veltrix/internal/shared"
)

func TestParsePayloadBuildEngineActions(t *testing.T) {
 cases:=[]struct{action string;raw string;want any}{
  {"list_build_history",`{"site_id":"demo","limit":5}`,&shared.ListBuildHistory{}},
  {"get_build_history",`{"site_id":"demo","id":"20261118T120000.000000000Z"}`,&shared.GetBuildHistory{}},
  {"list_build_artifacts",`{"site_id":"demo","build_id":"20261118T120000.000000000Z"}`,&shared.ListBuildArtifacts{}},
  {"read_build_artifact_chunk",`{"site_id":"demo","build_id":"20261118T120000.000000000Z","path":"dist/app.js","offset":0,"limit":1024}`,&shared.ReadBuildArtifactChunk{}},
  {"cancel_build",`{}`,&struct{}{}},
 }
 for _,tc:=range cases{t.Run(tc.action,func(t *testing.T){
  got,err:=parsePayload(tc.action,json.RawMessage(tc.raw));if err!=nil{t.Fatal(err)}
  switch tc.want.(type){case *shared.ListBuildHistory:if _,ok:=got.(*shared.ListBuildHistory);!ok{t.Fatalf("payload type %T",got)}
  case *shared.GetBuildHistory:if _,ok:=got.(*shared.GetBuildHistory);!ok{t.Fatalf("payload type %T",got)}
  case *shared.ListBuildArtifacts:if _,ok:=got.(*shared.ListBuildArtifacts);!ok{t.Fatalf("payload type %T",got)}
  case *shared.ReadBuildArtifactChunk:if _,ok:=got.(*shared.ReadBuildArtifactChunk);!ok{t.Fatalf("payload type %T",got)}
  case *struct{}:if _,ok:=got.(*struct{});!ok{t.Fatalf("payload type %T",got)}
  }
 })}
}


func TestParsePayloadAgentControlActions(t *testing.T) {
 cases := []struct {
  action string
  raw string
  want any
 }{
  {"get_site_restart_policy", `{"site_id":"demo"}`, &shared.GetSiteRestartPolicy{}},
  {"set_site_restart_policy", `{"site_id":"demo","policy":"on-failure"}`, &shared.SetSiteRestartPolicy{}},
  {"cancel_deployment_job", `{"site_id":"demo","job_id":"job-123"}`, &shared.DeploymentJobAction{}},
 }
 for _, tc := range cases {
  t.Run(tc.action, func(t *testing.T) {
   got, err := parsePayload(tc.action, json.RawMessage(tc.raw))
   if err != nil { t.Fatal(err) }
   switch tc.want.(type) {
   case *shared.GetSiteRestartPolicy:
    if _, ok := got.(*shared.GetSiteRestartPolicy); !ok { t.Fatalf("payload type %T", got) }
   case *shared.SetSiteRestartPolicy:
    if _, ok := got.(*shared.SetSiteRestartPolicy); !ok { t.Fatalf("payload type %T", got) }
   case *shared.DeploymentJobAction:
    if _, ok := got.(*shared.DeploymentJobAction); !ok { t.Fatalf("payload type %T", got) }
   }
  })
 }
}


func TestParsePayloadDeviceDiagnosticsActions(t *testing.T) {
 cases:=[]struct{action,raw string;want any}{
  {"get_device_diagnostics",`{}`,&shared.GetDeviceDiagnostics{}},
  {"check_device_port",`{"port":8080}`,&shared.CheckDevicePort{}},
  {"restart_agent",`{}`,&shared.RestartAgent{}},
  {"update_agent",`{}`,&struct{}{}},
 }
 for _,tc:=range cases{t.Run(tc.action,func(t *testing.T){
  got,err:=parsePayload(tc.action,json.RawMessage(tc.raw));if err!=nil{t.Fatal(err)}
  switch tc.want.(type){
  case *shared.GetDeviceDiagnostics:if _,ok:=got.(*shared.GetDeviceDiagnostics);!ok{t.Fatalf("payload type %T",got)}
  case *shared.CheckDevicePort:if _,ok:=got.(*shared.CheckDevicePort);!ok{t.Fatalf("payload type %T",got)}
  case *shared.RestartAgent:if _,ok:=got.(*shared.RestartAgent);!ok{t.Fatalf("payload type %T",got)}
  case *struct{}:if _,ok:=got.(*struct{});!ok{t.Fatalf("payload type %T",got)}
  }
 })}
}
