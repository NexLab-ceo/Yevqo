package control

import (
	"encoding/json"
	"testing"
)

func TestHostnameBelongsToZone(t *testing.T) {
	for _,host:=range []string{"example.com","app.example.com","a.b.example.com"}{if !hostnameBelongsToZone(host,"example.com"){t.Errorf("%s should belong to zone",host)}}
	for _,host:=range []string{"evil-example.com","example.com.evil.org","other.net",""}{if hostnameBelongsToZone(host,"example.com"){t.Errorf("%s must not belong to zone",host)}}
}

func TestCloudflareTunnelUUIDValidation(t *testing.T) {
	if !validCloudflareTunnelID("12345678-1234-abcd-9876-1234567890ab"){t.Fatal("valid tunnel UUID rejected")}
	for _,id:=range []string{"","1234","12345678-1234-1234-1234-1234567890ag","12345678_1234_abcd_9876_1234567890ab"}{if validCloudflareTunnelID(id){t.Fatalf("invalid tunnel ID %q accepted",id)}}
}

func TestCloudflareZoneIDValidation(t *testing.T) {
	if !validCloudflareZoneID("0123456789abcdef0123456789abcdef") { t.Fatal("valid zone ID rejected") }
	for _,id:=range []string{"", "not-a-zone-id", "0123456789abcdef0123456789abcdef/path", "0123456789abcdef0123456789abcde"} {
		if validCloudflareZoneID(id) { t.Fatalf("invalid zone ID %q accepted", id) }
	}
}

func TestUpdateCloudflareIngressRules(t *testing.T) {
	marshal:=func(v string) json.RawMessage { b,_:=json.Marshal(v);return b }
	rules:=[]map[string]json.RawMessage{
		{"hostname":marshal("other.example.com"),"service":marshal("http://127.0.0.1:8000"),"originRequest":json.RawMessage(`{"noTLSVerify":true}`)},
		{"service":marshal("http_status:404")},
	}
	updated,changed,err:=updateCloudflareIngressRules(rules,"app.example.com","http://127.0.0.1:3000",false,false)
	if err!=nil||!changed{t.Fatalf("add route changed=%v err=%v",changed,err)}
	if len(updated)!=3{t.Fatalf("ingress rules=%d, want 3",len(updated))}
	var host,service string
	_ = json.Unmarshal(updated[1]["hostname"],&host);_ = json.Unmarshal(updated[1]["service"],&service)
	if host!="app.example.com"||service!="http://127.0.0.1:3000"{t.Fatalf("new ingress route=%s -> %s",host,service)}
	if string(updated[0]["originRequest"])!=`{"noTLSVerify":true}`{t.Fatal("existing originRequest was not preserved")}
	updated,changed,err=updateCloudflareIngressRules(updated,"app.example.com","http://127.0.0.1:3100",false,true)
	if err!=nil||!changed{t.Fatalf("update route changed=%v err=%v",changed,err)}
	_ = json.Unmarshal(updated[1]["service"],&service);if service!="http://127.0.0.1:3100"{t.Fatalf("updated service=%q",service)}
	updated,changed,err=updateCloudflareIngressRules(updated,"app.example.com","",true,false)
	if err!=nil||!changed||len(updated)!=2{t.Fatalf("remove route rules=%d changed=%v err=%v",len(updated),changed,err)}
}

func TestUpdateCloudflareIngressRulesRejectsDuplicateHostname(t *testing.T) {
	marshal:=func(v string) json.RawMessage { b,_:=json.Marshal(v);return b }
	rules:=[]map[string]json.RawMessage{{"hostname":marshal("app.example.com"),"service":marshal("http://127.0.0.1:1")},{"hostname":marshal("app.example.com"),"service":marshal("http://127.0.0.1:2")},{"service":marshal("http_status:404")}}
	if _,_,err:=updateCloudflareIngressRules(rules,"app.example.com","http://127.0.0.1:3",false,false);err==nil{t.Fatal("expected duplicate hostname to be rejected")}
}

func TestUpdateCloudflareIngressRulesPreservesPathRulesAndCatchallOrder(t *testing.T) {
	marshal:=func(v string) json.RawMessage { b,_:=json.Marshal(v);return b }
	rules:=[]map[string]json.RawMessage{
		{"path":marshal("/api/*"),"service":marshal("http://127.0.0.1:9000")},
		{"service":marshal("http_status:404")},
	}
	updated,changed,err:=updateCloudflareIngressRules(rules,"app.example.com","http://127.0.0.1:3000",false,false)
	if err!=nil||!changed{t.Fatalf("add route changed=%v err=%v",changed,err)}
	if len(updated)!=3{t.Fatalf("rules=%d, want 3",len(updated))}
	var path,host string
	_ = json.Unmarshal(updated[0]["path"],&path);_ = json.Unmarshal(updated[1]["hostname"],&host)
	if path!="/api/*"||host!="app.example.com"{t.Fatalf("path rule/host order = %q, %q",path,host)}
	if _,ok:=updated[2]["hostname"];ok{t.Fatal("catch-all rule must remain last")}
}

func TestCloudflareTunnelConfigPreservesGlobalOriginRequest(t *testing.T) {
	var current cloudflareTunnelConfig
	if err:=json.Unmarshal([]byte(`{"source":"cloudflare","config":{"ingress":[{"service":"http_status:404"}],"originRequest":{"connectTimeout":30,"noTLSVerify":true}}}`),&current);err!=nil{t.Fatal(err)}
	body:=cloudflareTunnelConfigPayload(current,current.Config.Ingress)
	encoded,err:=json.Marshal(body);if err!=nil{t.Fatal(err)}
	var decoded map[string]json.RawMessage
	if err:=json.Unmarshal(encoded,&decoded);err!=nil{t.Fatal(err)}
	var got struct{OriginRequest map[string]interface{} `json:"originRequest"`} 
	var configDecoded map[string]json.RawMessage
	if err:=json.Unmarshal(decoded["config"],&configDecoded);err!=nil{t.Fatal(err)}
	if err:=json.Unmarshal(configDecoded["originRequest"],&got.OriginRequest);err!=nil{t.Fatal(err)}
	if got.OriginRequest["connectTimeout"]!=float64(30)||got.OriginRequest["noTLSVerify"]!=true{t.Fatalf("global originRequest was not preserved: %#v",got.OriginRequest)}
}

func TestUpdateCloudflareIngressRulesDoesNotOverwriteManualRoute(t *testing.T) {
	marshal:=func(v string) json.RawMessage { b,_:=json.Marshal(v);return b }
	rules:=[]map[string]json.RawMessage{{"hostname":marshal("app.example.com"),"service":marshal("http://127.0.0.1:9000")},{"service":marshal("http_status:404")}}
	if _,_,err:=updateCloudflareIngressRules(rules,"app.example.com","http://127.0.0.1:3000",false,false);err==nil{t.Fatal("expected an unmanaged existing ingress route to be protected")}
	updated,changed,err:=updateCloudflareIngressRules(rules,"app.example.com","http://127.0.0.1:3000",false,true)
	if err!=nil||!changed{t.Fatalf("managed route update changed=%v err=%v",changed,err)}
	var service string;_ = json.Unmarshal(updated[0]["service"],&service)
	if service!="http://127.0.0.1:3000"{t.Fatalf("service=%q",service)}
}
