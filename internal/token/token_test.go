package token

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	tok, err := Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(tok) != tokenLength {
		t.Errorf("token length = %d, want %d", len(tok), tokenLength)
	}

	for _, c := range tok {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			t.Errorf("token contains invalid character: %c", c)
		}
	}
}

func TestGenerateUniqueness(t *testing.T) {
	const n = 100
	tokens := make(map[string]bool, n)

	for i := 0; i < n; i++ {
		tok, err := Generate()
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if tokens[tok] {
			t.Errorf("duplicate token generated: %s", tok)
		}
		tokens[tok] = true
	}
}
