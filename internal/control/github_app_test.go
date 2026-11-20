package control

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestGitHubAppJWTUsesRS256AndShortExpiry(t *testing.T) {
	key,err:=rsa.GenerateKey(rand.Reader,2048);if err!=nil{t.Fatal(err)}
	private:=string(pem.EncodeToMemory(&pem.Block{Type:"RSA PRIVATE KEY",Bytes:x509.MarshalPKCS1PrivateKey(key)}))
	now:=time.Date(2026,11,20,12,0,0,0,time.UTC)
	token,err:=githubAppJWT("123456",private,now);if err!=nil{t.Fatal(err)}
	parts:=strings.Split(token,".");if len(parts)!=3{t.Fatalf("expected JWT with 3 parts, got %d",len(parts))}
	header,err:=base64.RawURLEncoding.DecodeString(parts[0]);if err!=nil{t.Fatal(err)}
	if string(header)!=`{"alg":"RS256","typ":"JWT"}`{t.Fatalf("unexpected JWT header: %s",header)}
	payload,err:=base64.RawURLEncoding.DecodeString(parts[1]);if err!=nil{t.Fatal(err)}
	if !strings.Contains(string(payload),`"iss":123456`)||!strings.Contains(string(payload),`"exp":1795176480`){t.Fatalf("unexpected JWT claims: %s",payload)}
	signature,err:=base64.RawURLEncoding.DecodeString(parts[2]);if err!=nil{t.Fatal(err)}
	sum:=sha256.Sum256([]byte(parts[0]+"."+parts[1]))
	if err:=rsa.VerifyPKCS1v15(&key.PublicKey,crypto.SHA256,sum[:],signature);err!=nil{t.Fatalf("JWT signature did not verify: %v",err)}
}

func TestValidateGitHubAppCredentials(t *testing.T) {
	valid:=githubAppCredentials{AppID:"123",InstallationID:"456",PrivateKey:"-----BEGIN RSA PRIVATE KEY-----\n"+strings.Repeat("A",128)+"\n-----END RSA PRIVATE KEY-----"}
	if err:=validateGitHubAppCredentials(valid);err!=nil{t.Fatalf("valid input rejected: %v",err)}
	for _,invalid:=range []githubAppCredentials{{AppID:"0",InstallationID:"456",PrivateKey:valid.PrivateKey},{AppID:"123",InstallationID:"x",PrivateKey:valid.PrivateKey},{AppID:"123",InstallationID:"456",PrivateKey:"not a key"}} {
		if err:=validateGitHubAppCredentials(invalid);err==nil{t.Fatalf("invalid credentials accepted: %#v",invalid)}
	}
}
