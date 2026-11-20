package auth

import (
	"encoding/json"
	"testing"
)

func TestParseTelegramIDClaim(t *testing.T) {
	tests := []struct {
		name    string
		claim   string
		want    int64
		wantErr bool
	}{
		{name: "number", claim: `123456789`, want: 123456789},
		{name: "decimal string", claim: `"123456789"`, want: 123456789},
		{name: "missing", claim: ``, wantErr: true},
		{name: "null", claim: `null`, wantErr: true},
		{name: "zero", claim: `0`, wantErr: true},
		{name: "negative", claim: `-1`, wantErr: true},
		{name: "fraction", claim: `12.5`, wantErr: true},
		{name: "not a number", claim: `"student"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.claim != "" {
				raw = json.RawMessage(tt.claim)
			}

			got, err := parseTelegramIDClaim(raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTelegramIDClaim() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parseTelegramIDClaim() = %d, want %d", got, tt.want)
			}
		})
	}
}
