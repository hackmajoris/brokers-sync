package watchlist

import (
	"strings"
	"testing"
)

func TestNewCodeRoundTrips(t *testing.T) {
	code, err := NewCode()
	if err != nil {
		t.Fatalf("NewCode: %v", err)
	}
	if got, want := len(code), CodeLen+3; got != want {
		t.Errorf("display length = %d, want %d (%q)", got, want, code)
	}
	norm, ok := Normalize(code)
	if !ok {
		t.Fatalf("Normalize rejected freshly generated code %q", code)
	}
	if len(norm) != CodeLen {
		t.Errorf("normalized length = %d, want %d", len(norm), CodeLen)
	}
}

// A code is the only credential, so a collision would silently hand one user
// another user's watchlist.
func TestNewCodeIsUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		code, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode: %v", err)
		}
		if seen[code] {
			t.Fatalf("duplicate code generated: %q", code)
		}
		seen[code] = true
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  string
		valid bool
	}{
		{"canonical", "K7M29QRF3XVB8TDW", "K7M29QRF3XVB8TDW", true},
		{"grouped", "K7M2-9QRF-3XVB-8TDW", "K7M29QRF3XVB8TDW", true},
		{"lowercase", "k7m29qrf3xvb8tdw", "K7M29QRF3XVB8TDW", true},
		{"spaces", "K7M2 9QRF 3XVB 8TDW", "K7M29QRF3XVB8TDW", true},
		{"confusable I and L fold to 1", "K7M29QRF3XVB8TDI", "K7M29QRF3XVB8TD1", true},
		{"confusable O folds to 0", "K7M29QRF3XVB8TDO", "K7M29QRF3XVB8TD0", true},
		{"too short", "K7M29QRF3XVB8TD", "", false},
		{"too long", "K7M29QRF3XVB8TDWX", "", false},
		{"empty", "", "", false},
		{"separators only", "----", "", false},
		{"out of alphabet", "K7M29QRF3XVB8TD!", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Normalize(tt.in)
			if ok != tt.valid {
				t.Fatalf("Normalize(%q) valid = %v, want %v", tt.in, ok, tt.valid)
			}
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Lookup is by hash, so every accepted spelling of one code must hash
// identically. If this breaks, users are locked out by their own formatting.
func TestHashKeyIsStableAcrossSpellings(t *testing.T) {
	spellings := []string{
		"K7M29QRF3XVB8TDW",
		"K7M2-9QRF-3XVB-8TDW",
		"k7m2 9qrf 3xvb 8tdw",
	}
	var want string
	for i, s := range spellings {
		norm, ok := Normalize(s)
		if !ok {
			t.Fatalf("Normalize(%q) rejected", s)
		}
		got := HashKey(norm)
		if i == 0 {
			want = got
			if !strings.HasPrefix(got, "P#") {
				t.Errorf("HashKey = %q, want P# prefix", got)
			}
			continue
		}
		if got != want {
			t.Errorf("HashKey(%q) = %q, want %q", s, got, want)
		}
	}
}

// The raw code must never be recoverable from what is persisted.
func TestHashKeyDoesNotContainCode(t *testing.T) {
	const code = "K7M29QRF3XVB8TDW"
	if strings.Contains(HashKey(code), code) {
		t.Error("HashKey output contains the plaintext code")
	}
}
