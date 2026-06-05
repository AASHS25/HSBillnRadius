package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/aashs25/hsbillnradius/internal/platform/token"
)

// AccessParser validates an access token into its claims.
type AccessParser interface {
	ParseAccess(raw string) (*token.AccessClaims, error)
}

// Authenticator validates the Bearer access token and injects its claims into
// the request context. Requests without a valid token are rejected with 401.
func Authenticator(parser AccessParser, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := bearerToken(r)
			if err != nil {
				writeError(w, log, token.ErrInvalidToken)
				return
			}
			claims, err := parser.ParseAccess(raw)
			if err != nil {
				writeError(w, log, token.ErrInvalidToken)
				return
			}
			next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
		})
	}
}

// RequirePermission enforces that the authenticated user holds the given
// permission code. It must be chained after Authenticator.
func RequirePermission(code string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, log, token.ErrInvalidToken)
				return
			}
			if !slices.Contains(claims.Permissions, code) {
				writeJSON(w, http.StatusForbidden, errorBody{
					Error: "permission denied: " + code,
					Code:  "forbidden",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, error) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(h[len(prefix):]), nil
}
