package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSiteEnvironmentIsEncryptedAndPersistent(t *testing.T) {
	root:=t.TempDir()
	manager:=&ProcessManager{DataRoot:root}
	values:=map[string]string{"APP_ENV":"production","API_TOKEN":"very-secret-value"}
	if err:=manager.saveSiteEnvironment("demo",values);err!=nil{t.Fatal(err)}
	path:=filepath.Join(root,"sites","demo",".yevqo","environment.enc")
	raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
	if string(raw)==""||string(raw)=="very-secret-value"{t.Fatal("environment file is empty or plaintext")}
	info,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
	if info.Mode().Perm()!=0600{t.Fatalf("expected 0600 permissions, got %o",info.Mode().Perm())}
	loaded,err:=manager.loadSiteEnvironment("demo");if err!=nil{t.Fatal(err)}
	if loaded["APP_ENV"]!="production"||loaded["API_TOKEN"]!="very-secret-value"{t.Fatalf("environment did not round trip: %#v",loaded)}
}

func TestSiteEnvironmentRejectsTamperedCiphertext(t *testing.T) {
	root:=t.TempDir();manager:=&ProcessManager{DataRoot:root}
	if err:=manager.saveSiteEnvironment("demo",map[string]string{"APP_ENV":"production"});err!=nil{t.Fatal(err)}
	path:=filepath.Join(root,"sites","demo",".yevqo","environment.enc")
	data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
	data[len(data)-1]^=0xff
	if err:=os.WriteFile(path,data,0600);err!=nil{t.Fatal(err)}
	if _,err:=manager.loadSiteEnvironment("demo");err==nil{t.Fatal("tampered environment ciphertext must fail authentication")}
}
