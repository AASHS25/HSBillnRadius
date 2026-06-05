package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h := NewHasher()

	encoded, err := h.Hash("correct horse battery staple")
	require.NoError(t, err)
	assert.Contains(t, encoded, "$argon2id$")

	ok, err := h.Verify("correct horse battery staple", encoded)
	require.NoError(t, err)
	assert.True(t, ok, "correct password should verify")

	ok, err = h.Verify("wrong password", encoded)
	require.NoError(t, err)
	assert.False(t, ok, "wrong password must not verify")
}

func TestHash_DifferentSaltsEachTime(t *testing.T) {
	h := NewHasher()
	a, err := h.Hash("same")
	require.NoError(t, err)
	b, err := h.Hash("same")
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "salts must differ so hashes differ")
}

func TestVerify_InvalidHash(t *testing.T) {
	h := NewHasher()
	cases := []string{
		"",
		"not-a-hash",
		"$argon2id$v=19$m=65536,t=2,p=1$only-five-parts",
		"$bcrypt$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA",
	}
	for _, c := range cases {
		_, err := h.Verify("x", c)
		assert.Error(t, err, "input %q", c)
	}
}
