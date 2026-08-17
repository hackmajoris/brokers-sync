package wishlist

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

// crockford is the Crockford base32 alphabet: no I, L, O or U, so codes are
// unambiguous when read aloud or retyped and cannot spell words.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// codeBytes is 10 bytes of entropy, which encodes to exactly CodeLen base32
// characters with no padding.
const codeBytes = 10

// CodeLen is the length of a normalized code, without grouping separators.
const CodeLen = 16

var encoding = base32.NewEncoding(crockford).WithPadding(base32.NoPadding)

// NewCode returns a fresh portfolio code in display form, e.g. K7M2-9QRF-3XVB-8TDW.
func NewCode() (string, error) {
	b := make([]byte, codeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return group(encoding.EncodeToString(b)), nil
}

// Normalize converts a code in any input form to its canonical form: uppercase,
// no separators, with visually confusable characters folded onto the Crockford
// equivalents. It reports whether the result is a well-formed code.
func Normalize(s string) (string, bool) {
	var b strings.Builder
	b.Grow(CodeLen)
	for _, r := range strings.ToUpper(s) {
		switch r {
		case '-', ' ':
			continue
		case 'I', 'L':
			r = '1'
		case 'O':
			r = '0'
		}
		if !strings.ContainsRune(crockford, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) != CodeLen {
		return "", false
	}
	return out, true
}

// HashKey returns the DynamoDB partition key for a code. Only this value is
// ever persisted, so a dump of the table yields no usable codes.
func HashKey(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return "P#" + hex.EncodeToString(sum[:])
}

// group inserts a separator every 4 characters for display.
func group(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}
