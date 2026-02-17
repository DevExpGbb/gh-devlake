package azure

import "testing"

func TestSuffix_Length(t *testing.T) {
	s := Suffix("devlake-rg")
	if len(s) != 5 {
		t.Errorf("got length %d, want 5", len(s))
	}
}

func TestSuffix_Lowercase(t *testing.T) {
	s := Suffix("MyResourceGroup")
	for _, c := range s {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			t.Errorf("unexpected character %q — should be lowercase hex", c)
		}
	}
}

func TestSuffix_Deterministic(t *testing.T) {
	a := Suffix("devlake-rg")
	b := Suffix("devlake-rg")
	if a != b {
		t.Errorf("same input should give same suffix: %q != %q", a, b)
	}
}

func TestSuffix_Different(t *testing.T) {
	a := Suffix("devlake-rg-1")
	b := Suffix("devlake-rg-2")
	if a == b {
		t.Errorf("different inputs should give different suffixes: %q == %q", a, b)
	}
}
