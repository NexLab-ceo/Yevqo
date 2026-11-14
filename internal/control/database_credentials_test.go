package control

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialKeyIsPrivateAndStable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	first,err:=loadCredentialKey("");if err!=nil{t.Fatal(err)}
	second,err:=loadCredentialKey("");if err!=nil{t.Fatal(err)}
	if !bytes.Equal(first,second){t.Fatal("credential key changed between loads")}
	path:=filepath.Join(os.Getenv("XDG_CONFIG_HOME"),"veltrix","credentials.key")
	info,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
	if info.Mode().Perm()!=0o600{t.Fatalf("expected 0600 key permissions, got %o",info.Mode().Perm())}
}

func TestCredentialEncryptionRoundTrip(t *testing.T) {
	key:=bytes.Repeat([]byte{0x42},32)
	nonce,ciphertext,err:=encryptCredential(key,[]byte("private-database-password"))
	if err!=nil{t.Fatal(err)}
	plain,err:=decryptCredential(key,nonce,ciphertext)
	if err!=nil{t.Fatal(err)}
	if string(plain)!="private-database-password"{t.Fatalf("unexpected plaintext: %q",plain)}
	if _,err:=decryptCredential(bytes.Repeat([]byte{0x24},32),nonce,ciphertext);err==nil{t.Fatal("expected authentication failure with wrong key")}
}
