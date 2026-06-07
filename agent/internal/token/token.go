// Package token handles generation, hashing, and verification of app tokens and
// one-time pairing codes.
//
// Security model:
//   - Token values are 256 bits of CSPRNG entropy, base64url-encoded, prefixed
//     "ddn_" for easy identification in logs/grep (the prefix is not secret).
//   - Tokens are NEVER stored in plaintext. We store a per-token random salt and
//     a SHA-256 hash of (salt || token). Verification is constant-time.
//   - Pairing codes are short, human-typeable, single-use, and short-lived. They
//     are rate-limited at the API layer.
//
// Why SHA-256 and not bcrypt/argon2 for tokens? App tokens carry full 256-bit
// entropy, so they are not subject to dictionary/brute-force attacks the way a
// human password is; a salted fast hash is sufficient and keeps verification
// cheap on every request. Pairing CODES are low-entropy, which is exactly why
// they are single-use, time-boxed, and rate-limited rather than hashed-and-kept.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// TokenPrefix is a non-secret marker so tokens are recognizable.
	TokenPrefix = "ddn_"
	tokenBytes  = 32 // 256 bits
	saltBytes   = 16
)

// Generate returns a new opaque token value (to be shown to the client once).
func Generate() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// NewSalt returns a fresh random salt, hex-encoded.
func NewSalt() (string, error) {
	b := make([]byte, saltBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Hash computes the at-rest hash for a token given its salt.
func Hash(tokenValue, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(tokenValue))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify reports whether tokenValue matches the stored (salt, expectedHash)
// using a constant-time comparison.
func Verify(tokenValue, salt, expectedHash string) bool {
	got := Hash(tokenValue, salt)
	return subtle.ConstantTimeCompare([]byte(got), []byte(expectedHash)) == 1
}

// HashForLookup returns a salt-free hash suitable as a lookup index. We can't
// use the salted hash to look a token up (we'd need the salt first), so we keep
// an additional unsalted "lookup hash" purely as a map key. This is acceptable
// because the token itself is high-entropy; the lookup hash leaks nothing
// useful without the original 256-bit secret.
func HashForLookup(tokenValue string) string {
	sum := sha256.Sum256([]byte(tokenValue))
	return hex.EncodeToString(sum[:])
}

// codeAlphabet excludes easily-confused characters (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// GenerateCode returns a grouped, human-typeable one-time pairing code such as
// "K7QM-29FB-XTRP". groups*size characters of entropy.
func GenerateCode(groups, size int) (string, error) {
	if groups < 1 {
		groups = 3
	}
	if size < 1 {
		size = 4
	}
	total := groups * size
	buf := make([]byte, total)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	var sb strings.Builder
	for i := 0; i < total; i++ {
		if i > 0 && i%size == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(codeAlphabet[int(buf[i])%len(codeAlphabet)])
	}
	return sb.String(), nil
}

// NormalizeCode upper-cases and strips spaces/dashes so user input like
// "k7qm 29fb-xtrp" compares equal to the canonical form.
func NormalizeCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// CodesEqual compares two pairing codes in normalized, constant-time fashion.
func CodesEqual(a, b string) bool {
	na, nb := NormalizeCode(a), NormalizeCode(b)
	return subtle.ConstantTimeCompare([]byte(na), []byte(nb)) == 1
}
