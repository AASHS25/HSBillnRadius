package httpapi

import (
	"context"

	"github.com/aashs25/hsbillnradius/internal/platform/token"
)

type ctxKey int

const claimsKey ctxKey = iota

// withClaims stores the authenticated access-token claims in the context.
func withClaims(ctx context.Context, c *token.AccessClaims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext returns the authenticated claims, if any.
func ClaimsFromContext(ctx context.Context) (*token.AccessClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*token.AccessClaims)
	return c, ok
}

// TenantID returns the authenticated tenant id, or 0 when unauthenticated.
func TenantID(ctx context.Context) int64 {
	if c, ok := ClaimsFromContext(ctx); ok {
		return c.TenantID
	}
	return 0
}

// UserID returns the authenticated user id, or 0 when unauthenticated.
func UserID(ctx context.Context) int64 {
	if c, ok := ClaimsFromContext(ctx); ok {
		return c.UserID
	}
	return 0
}
