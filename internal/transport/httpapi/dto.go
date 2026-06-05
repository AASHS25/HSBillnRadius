package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/service/authsvc"
)

// maxBodyBytes caps request bodies to mitigate abuse.
const maxBodyBytes = 1 << 20 // 1 MiB

// --- requests ---------------------------------------------------------------

type registerRequest struct {
	TenantName string `json:"tenant_name" validate:"required,min=2,max=120"`
	TenantSlug string `json:"tenant_slug" validate:"required,min=3,max=40"`
	Name       string `json:"name" validate:"required,min=2,max=120"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required,min=8,max=200"`
}

type loginRequest struct {
	TenantSlug string `json:"tenant_slug" validate:"required"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// --- responses --------------------------------------------------------------

type userResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	RoleID   int64  `json:"role_id"`
	IsActive bool   `json:"is_active"`
}

type tenantResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
	Plan   string `json:"plan"`
}

type authResponse struct {
	AccessToken  string         `json:"access_token"`
	TokenType    string         `json:"token_type"`
	ExpiresAt    time.Time      `json:"expires_at"`
	RefreshToken string         `json:"refresh_token"`
	User         userResponse   `json:"user"`
	Tenant       tenantResponse `json:"tenant"`
	Permissions  []string       `json:"permissions"`
}

type meResponse struct {
	User        userResponse   `json:"user"`
	Tenant      tenantResponse `json:"tenant"`
	Permissions []string       `json:"permissions"`
}

func toUserResponse(u iam.User) userResponse {
	return userResponse{ID: u.ID, Name: u.Name, Email: u.Email, RoleID: u.RoleID, IsActive: u.IsActive}
}

func toTenantResponse(t tenant.Tenant) tenantResponse {
	return tenantResponse{ID: t.ID, Name: t.Name, Slug: t.Slug, Status: string(t.Status), Plan: t.Plan}
}

func toAuthResponse(r *authsvc.AuthResult) authResponse {
	return authResponse{
		AccessToken:  r.AccessToken,
		TokenType:    "Bearer",
		ExpiresAt:    r.AccessExpiresAt,
		RefreshToken: r.RefreshToken,
		User:         toUserResponse(r.User),
		Tenant:       toTenantResponse(r.Tenant),
		Permissions:  r.Permissions,
	}
}

// decodeJSON reads, strictly decodes and validates a JSON request body.
func decodeJSON(valid *validator.Validate, w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errBadRequest("empty request body")
		}
		return errBadRequest("invalid JSON: " + err.Error())
	}
	return valid.Struct(dst)
}
