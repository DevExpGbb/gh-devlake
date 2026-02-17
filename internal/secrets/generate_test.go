package secrets

import (
	"testing"
	"unicode"
)

func TestEncryptionSecret_Length(t *testing.T) {
	s, err := EncryptionSecret(128)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 128 {
		t.Errorf("got length %d, want 128", len(s))
	}
}

func TestEncryptionSecret_OnlyUppercase(t *testing.T) {
	s, err := EncryptionSecret(128)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range s {
		if !unicode.IsUpper(c) {
			t.Errorf("unexpected character %q in encryption secret", c)
		}
	}
}

func TestMySQLPassword_Suffix(t *testing.T) {
	p, err := MySQLPassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 20 {
		t.Errorf("got length %d, want 20", len(p))
	}
	// Should end with "Aa1!" for complexity
	suffix := p[len(p)-4:]
	if suffix != "Aa1!" {
		t.Errorf("expected suffix Aa1!, got %q", suffix)
	}
}

func TestEncryptionSecret_Unique(t *testing.T) {
	a, _ := EncryptionSecret(32)
	b, _ := EncryptionSecret(32)
	if a == b {
		t.Error("two generated secrets should not be identical")
	}
}
