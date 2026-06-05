// Package token issues and validates short-lived JWT access tokens and the
// opaque refresh tokens stored (hashed) in the database.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned when an access token is malformed, expired or has
// an invalid signature.
var ErrInvalidToken = errors.New("invalid or expired token")

// AccessClaims are the custom claims embedded in an access token. Permissions
// are embedded so RBAC checks need no per-request database lookup; the short
// TTL bounds how stale they can be.
type AccessClaims struct {
	jwt.RegisteredClaims
	TenantID    int64    `json:"tid"`
	UserID      int64    `json:"uid"`
	RoleID      int64    `json:"rid"`
	Permissions []string `json:"perms"`
}

// Manager signs and verifies access tokens with a shared HMAC secret.
type Manager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// NewManager builds a token Manager.
func NewManager(secret, issuer string, accessTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

// AccessInput carries the identity baked into a new access token.
type AccessInput struct {
	TenantID    int64
	UserID      int64
	RoleID      int64
	Permissions []string
}

// IssueAccess signs a new access token and returns it with its expiry.
func (m *Manager) IssueAccess(in AccessInput) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(in.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		TenantID:    in.TenantID,
		UserID:      in.UserID,
		RoleID:      in.RoleID,
		Permissions: in.Permissions,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccess validates an access token and returns its claims.
func (m *Manager) ParseAccess(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, errors.Join(ErrInvalidToken, err)
	}
	return claims, nil
}

// GenerateRefresh returns a new opaque refresh token (given to the client) and
// its SHA-256 hash (stored in the database). The plaintext is never persisted.
func GenerateRefresh() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashRefresh(plaintext), nil
}

// HashRefresh returns the SHA-256 hex digest used to look up a refresh token.
func HashRefresh(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
