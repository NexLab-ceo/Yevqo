package control

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func loadCredentialKey(dataRoot string) ([]byte, error) {
	path := filepath.Join(dataRoot, "credentials.key")
	if b, err := os.ReadFile(path); err == nil {
		if len(b) != 32 { return nil, fmt.Errorf("invalid credential key length") }
		if err := os.Chmod(path, 0o600); err != nil { return nil, err }
		return b, nil
	} else if !os.IsNotExist(err) { return nil, err }
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil { return nil, err }
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) { return loadCredentialKey(dataRoot) }
		return nil, err
	}
	if _, err = f.Write(key); err != nil { f.Close(); _ = os.Remove(path); return nil, err }
	if err = f.Sync(); err != nil { f.Close(); return nil, err }
	if err = f.Close(); err != nil { return nil, err }
	return key, nil
}

func encryptCredential(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key); if err != nil { return nil,nil,err }
	gcm, err := cipher.NewGCM(block); if err != nil { return nil,nil,err }
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return nil,nil,err }
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce,ciphertext,nil
}

func decryptCredential(key, nonce, ciphertext []byte) ([]byte,error) {
	block,err:=aes.NewCipher(key);if err!=nil{return nil,err}
	gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err}
	if len(nonce)!=gcm.NonceSize(){return nil,fmt.Errorf("invalid credential nonce")}
	return gcm.Open(nil,nonce,ciphertext,nil)
}
