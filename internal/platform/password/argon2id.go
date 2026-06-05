// Package password hashes and verifies passwords using argon2id, encoding the
// result as a self-describing PHC string so parameters travel with the hash.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// params holds the argon2id cost parameters. These defaults target server-side
// hashing and exceed the OWASP minimum.
type params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultParams = params{
	memory:      64 * 1024, // 64 MiB
	iterations:  2,
	parallelism: 1,
	saltLength:  16,
	keyLength:   32,
}

// ErrInvalidHash is returned when an encoded hash cannot be parsed.
var ErrInvalidHash = errors.New("invalid password hash format")

// ErrIncompatibleVersion is returned when the argon2 version differs.
var ErrIncompatibleVersion = errors.New("incompatible argon2 version")

// Hasher derives and verifies argon2id password hashes.
type Hasher struct{ p params }

// NewHasher returns a Hasher using the default cost parameters.
func NewHasher() *Hasher { return &Hasher{p: defaultParams} }

// Hash derives an argon2id hash and returns it PHC-encoded.
func (h *Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, h.p.iterations, h.p.memory, h.p.parallelism, h.p.keyLength)

	b64 := base64.RawStdEncoding.EncodeToString
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.memory, h.p.iterations, h.p.parallelism,
		b64(salt), b64(key))
	return encoded, nil
}

// Verify reports whether plain matches the PHC-encoded hash, in constant time.
func (h *Hasher) Verify(plain, encoded string) (bool, error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, err
	}

	other := argon2.IDKey([]byte(plain), salt, p.iterations, p.memory, p.parallelism, p.keyLength)
	if subtle.ConstantTimeEq(int32(len(key)), int32(len(other))) == 0 {
		return false, nil
	}
	return subtle.ConstantTimeCompare(key, other) == 1, nil
}

// decode parses a PHC-encoded argon2id hash into its parts.
func decode(encoded string) (p params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return p, nil, nil, ErrIncompatibleVersion
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return p, nil, nil, ErrInvalidHash
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	p.saltLength = uint32(len(salt))
	p.keyLength = uint32(len(key))
	return p, salt, key, nil
}
