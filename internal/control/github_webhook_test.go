package control

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"strings"
)

func TestValidGitHubWebhookSignature(t *testing.T) {
	secret:=[]byte("test-webhook-secret")
	body:=[]byte(`{"ref":"refs/heads/main","after":"0123456789012345678901234567890123456789"}`)
	mac:=hmac.New(sha256.New,secret);_,_=mac.Write(body)
	valid:="sha256="+hex.EncodeToString(mac.Sum(nil))
	if !validGitHubWebhookSignature(valid,body,secret){t.Fatal("expected valid signature to pass")}
	if validGitHubWebhookSignature(valid,[]byte("different"),secret){t.Fatal("modified payload must fail")}
	if validGitHubWebhookSignature(valid,body,[]byte("wrong-secret")){t.Fatal("wrong secret must fail")}
	for _,invalid:=range []string{"","sha1=abcd","sha256=xyz","sha256=00"}{if validGitHubWebhookSignature(invalid,body,secret){t.Fatalf("invalid signature %q passed",invalid)}}
}

func TestWebhookBranchAndCommitValidation(t *testing.T) {
	if !validWebhookCommit("0123456789012345678901234567890123456789"){t.Fatal("valid SHA-1 rejected")}
	if !validWebhookCommit("0123456789012345678901234567890123456789012345678901234567890123"){t.Fatal("valid SHA-256 rejected")}
	for _,v:=range []string{"", "abc", "012345678901234567890123456789012345678G", strings.Repeat("0",39)}{if validWebhookCommit(v){t.Fatalf("invalid commit %q passed",v)}}
}
