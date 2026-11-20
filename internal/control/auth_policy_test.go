package control

import (
	"strings"
	"testing"
)

func TestValidateAccountPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "minimum length", password: strings.Repeat("a", 12)},
		{name: "maximum length", password: strings.Repeat("a", 72)},
		{name: "too short", password: strings.Repeat("a", 11), wantErr: true},
		{name: "too long", password: strings.Repeat("a", 73), wantErr: true},
		{name: "leading whitespace", password: " " + strings.Repeat("a", 12), wantErr: true},
		{name: "trailing whitespace", password: strings.Repeat("a", 12) + " ", wantErr: true},
		{name: "multibyte within byte limit", password: strings.Repeat("я", 12)},
		{name: "multibyte over byte limit", password: strings.Repeat("я", 37), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateAccountPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
