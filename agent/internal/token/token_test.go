package token

import (
	"strings"
	"testing"
)

func TestGenerateProducesUniquePrefixedTokens(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !strings.HasPrefix(tok, TokenPrefix) {
			t.Fatalf("token %q missing prefix %q", tok, TokenPrefix)
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = true
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	tok, _ := Generate()
	salt, err := NewSalt()
	if err != nil {
		t.Fatalf("NewSalt: %v", err)
	}
	h := Hash(tok, salt)
	if !Verify(tok, salt, h) {
		t.Fatal("Verify should accept the correct token")
	}
	if Verify(tok+"x", salt, h) {
		t.Fatal("Verify must reject a wrong token")
	}
	if Verify(tok, salt+"00", h) {
		t.Fatal("Verify must reject a wrong salt")
	}
}

func TestHashIsSalted(t *testing.T) {
	tok, _ := Generate()
	h1 := Hash(tok, "aaaa")
	h2 := Hash(tok, "bbbb")
	if h1 == h2 {
		t.Fatal("different salts must yield different hashes")
	}
}

func TestNormalizeCode(t *testing.T) {
	cases := map[string]string{
		"k7qm-29fb-xtrp": "K7QM29FBXTRP",
		"K7QM 29FB XTRP": "K7QM29FBXTRP",
		" k7qm29fbxtrp ": "K7QM29FBXTRP",
	}
	for in, want := range cases {
		if got := NormalizeCode(in); got != want {
			t.Errorf("NormalizeCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCodesEqualNormalizes(t *testing.T) {
	if !CodesEqual("k7qm-29fb", "K7QM29FB") {
		t.Fatal("codes should compare equal after normalization")
	}
	if CodesEqual("k7qm-29fb", "k7qm-29fc") {
		t.Fatal("different codes must not be equal")
	}
}

func TestGenerateCodeShape(t *testing.T) {
	code, err := GenerateCode(3, 4)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	groups := strings.Split(code, "-")
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d (%q)", len(groups), code)
	}
	for _, g := range groups {
		if len(g) != 4 {
			t.Fatalf("expected group length 4, got %q", g)
		}
		for _, r := range g {
			if !strings.ContainsRune(codeAlphabet, r) {
				t.Fatalf("code contains out-of-alphabet rune %q", r)
			}
		}
	}
}
