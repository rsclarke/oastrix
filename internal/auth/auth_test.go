package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	displayKey, prefix, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if len(prefix) != prefixLength {
		t.Errorf("prefix length = %d, want %d", len(prefix), prefixLength)
	}

	for _, c := range prefix {
		if !isAlphanumeric(c) {
			t.Errorf("prefix contains non-alphanumeric character: %c", c)
		}
	}

	// Format: oastrix_<prefix>_<secret>
	expectedStart := "oastrix_" + prefix + "_"
	if !strings.HasPrefix(displayKey, expectedStart) {
		t.Errorf("displayKey %q does not start with %q", displayKey, expectedStart)
	}

	// Extract secret part - base62 encoding of 32 bytes is ~43 chars
	secret := strings.TrimPrefix(displayKey, expectedStart)
	if len(secret) < 40 || len(secret) > 44 {
		t.Errorf("secret length = %d, want 40-44 (base62 of 32 bytes)", len(secret))
	}
	// Verify secret contains only alphanumeric characters (no _ or -)
	for _, c := range secret {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Errorf("secret contains invalid character: %c", c)
		}
	}

	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32 (SHA256)", len(hash))
	}
}

func TestHashSecretDeterministic(t *testing.T) {
	secret := "test-secret-value"

	hash1 := HashSecret(secret)
	hash2 := HashSecret(secret)

	if string(hash1) != string(hash2) {
		t.Error("HashSecret is not deterministic")
	}

	differentSecret := "different-secret"
	hash3 := HashSecret(differentSecret)
	if string(hash1) == string(hash3) {
		t.Error("HashSecret should produce different results with different secret")
	}
}

func TestVerifyAPIKey(t *testing.T) {
	displayKey, _, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if !VerifyAPIKey(displayKey, hash) {
		t.Error("VerifyAPIKey should return true for valid key")
	}

	if VerifyAPIKey("oastrix_invalid12345_key", hash) {
		t.Error("VerifyAPIKey should return false for invalid key")
	}

	wrongHash := make([]byte, 32)
	if VerifyAPIKey(displayKey, wrongHash) {
		t.Error("VerifyAPIKey should return false with wrong hash")
	}
}

func TestParseAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantPre string
		wantSec string
	}{
		{
			name:    "valid key",
			input:   "oastrix_abcdef123456_somesecretvalue123",
			wantErr: false,
			wantPre: "abcdef123456",
			wantSec: "somesecretvalue123",
		},
		{
			name:    "missing service prefix",
			input:   "abcdef123456_somesecretvalue123",
			wantErr: true,
		},
		{
			name:    "wrong service prefix",
			input:   "stripe_abcdef123456_somesecretvalue123",
			wantErr: true,
		},
		{
			name:    "no separator",
			input:   "oastrix_noseparatorhere",
			wantErr: true,
		},
		{
			name:    "prefix too short",
			input:   "oastrix_short_secret",
			wantErr: true,
		},
		{
			name:    "prefix too long",
			input:   "oastrix_abcdef1234567_secret",
			wantErr: true,
		},
		{
			name:    "uppercase in prefix",
			input:   "oastrix_ABCDEF123456_secret",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, secret, err := ParseAPIKey(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseAPIKey should return error")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseAPIKey failed: %v", err)
				return
			}
			if prefix != tt.wantPre {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPre)
			}
			if secret != tt.wantSec {
				t.Errorf("secret = %q, want %q", secret, tt.wantSec)
			}
		})
	}
}
