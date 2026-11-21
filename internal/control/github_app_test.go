package control

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
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

func TestGitHubAppVerificationMessageDoesNotExposeInternalErrors(t *testing.T) {
	cases := []struct { name, input, want string }{
		{"jwt rejected", "GitHub rejected App installation credentials: provider returned HTTP 401", "GitHub rejected the App JWT (HTTP 401). Check that the App ID matches the App that generated the private key."},
		{"installation missing", "GitHub rejected App installation credentials: provider returned HTTP 404", "GitHub could not find this installation (HTTP 404). Check the installation ID and that the App is installed for this account."},
		{"wrong app", "installation does not belong to this GitHub App", "The installation belongs to a different GitHub App. Check the configured App ID and installation ID."},
		{"bad key", "could not parse GitHub App RSA private key", "The configured GitHub App private key could not be parsed. Upload the private key generated for this App."},
		{"network", "dial tcp: private-network-detail", "GitHub App verification failed during installation lookup. Check the App configuration and retry; internal details were withheld."},
		{"installation token rejected", "could not create GitHub installation token: provider returned HTTP 422", "GitHub rejected the installation token creation request (HTTP 422). Recheck that the App is installed for the selected account and that its installation is active."},
		{"TLS failure", "x509: certificate signed by unknown authority", "The Control Plane could not establish a trusted TLS connection to GitHub. Check the device date, certificate store and network filtering."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := githubAppVerificationMessage(fmt.Errorf("%s", tc.input))
			if got != tc.want { t.Fatalf("message = %q, want %q", got, tc.want) }
			if strings.Contains(got, "private-network-detail") { t.Fatal("internal error leaked") }
		})
	}
}
