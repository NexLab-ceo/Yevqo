package control

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestPreviewKeyIsRandomHex(t *testing.T) {
	seen := map[string]bool{}
	for i:=0;i<32;i++ {
		key,err:=newPreviewKey()
		if err!=nil { t.Fatalf("newPreviewKey: %v",err) }
		if len(key)!=32 { t.Fatalf("key length=%d, want 32",len(key)) }
		if _,err:=hex.DecodeString(key);err!=nil { t.Fatalf("key is not hex: %v",err) }
		if seen[key] { t.Fatalf("duplicate preview key %q",key) }
		seen[key]=true
	}
}

func TestPreviewPortRange(t *testing.T) {
	for i:=0;i<128;i++ {
		port,err:=previewPort()
		if err!=nil { t.Fatalf("previewPort: %v",err) }
		if port<30000 || port>=50000 { t.Fatalf("preview port %d outside [30000,50000)",port) }
	}
}

func TestTruncatePreviewError(t *testing.T) {
	if got:=truncatePreviewError("  failure  ");got!="failure" { t.Fatalf("got %q",got) }
	if got:=truncatePreviewError(strings.Repeat("x",1200));len(got)!=900 { t.Fatalf("truncated error length=%d, want 900",len(got)) }
}
