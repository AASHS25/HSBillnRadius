package authsvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/platform/password"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/authsvc"
)

func newService() (*authsvc.Service, *memrepo.Store) {
	store := memrepo.New()
	hasher := password.NewHasher()
	tokens := token.NewManager("test-secret-of-sufficient-length-xx", "billing-radius", 15*time.Minute)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := authsvc.New(store.Repositories(), store, hasher, tokens, 24*time.Hour, log)
	return svc, store
}

func register(t *testing.T, svc *authsvc.Service) *authsvc.AuthResult {
	t.Helper()
	res, err := svc.Register(context.Background(), authsvc.RegisterInput{
		TenantName: "Acme ISP",
		TenantSlug: "acme",
		Name:       "Owner",
		Email:      "owner@acme.test",
		Password:   "password123",
	})
	require.NoError(t, err)
	return res
}

func TestRegister_Success(t *testing.T) {
	svc, store := newService()
	res := register(t, svc)

	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.Equal(t, "owner@acme.test", res.User.Email)
	assert.Equal(t, "acme", res.Tenant.Slug)
	assert.Equal(t, tenant.StatusTrial, res.Tenant.Status)
	assert.Contains(t, res.Permissions, "user.read")

	// An audit entry was recorded for the registration.
	entries, err := svc.ListAuditLogs(context.Background(), res.Tenant.ID, 50, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
	_ = store
}

func TestRegister_DuplicateSlug(t *testing.T) {
	svc, _ := newService()
	register(t, svc)

	_, err := svc.Register(context.Background(), authsvc.RegisterInput{
		TenantName: "Other", TenantSlug: "acme", Name: "X", Email: "x@y.test", Password: "password123",
	})
	assert.ErrorIs(t, err, tenant.ErrSlugTaken)
}

func TestRegister_Validation(t *testing.T) {
	svc, _ := newService()
	base := authsvc.RegisterInput{TenantName: "A", TenantSlug: "acme", Name: "N", Email: "a@b.test", Password: "password123"}

	t.Run("weak password", func(t *testing.T) {
		in := base
		in.Password = "short"
		_, err := svc.Register(context.Background(), in)
		assert.ErrorIs(t, err, iam.ErrWeakPassword)
	})
	t.Run("invalid email", func(t *testing.T) {
		in := base
		in.Email = "not-an-email"
		_, err := svc.Register(context.Background(), in)
		assert.ErrorIs(t, err, iam.ErrInvalidEmail)
	})
	t.Run("invalid slug", func(t *testing.T) {
		in := base
		in.TenantSlug = "A!"
		_, err := svc.Register(context.Background(), in)
		assert.ErrorIs(t, err, tenant.ErrInvalidSlug)
	})
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newService()
	register(t, svc)

	res, err := svc.Login(context.Background(), authsvc.LoginInput{
		TenantSlug: "acme", Email: "OWNER@acme.test", Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.Equal(t, "owner@acme.test", res.User.Email)
}

func TestLogin_Failures(t *testing.T) {
	svc, store := newService()
	res := register(t, svc)

	t.Run("wrong password", func(t *testing.T) {
		_, err := svc.Login(context.Background(), authsvc.LoginInput{TenantSlug: "acme", Email: "owner@acme.test", Password: "nope12345"})
		assert.ErrorIs(t, err, iam.ErrInvalidCredential)
	})
	t.Run("unknown tenant", func(t *testing.T) {
		_, err := svc.Login(context.Background(), authsvc.LoginInput{TenantSlug: "ghost", Email: "owner@acme.test", Password: "password123"})
		assert.ErrorIs(t, err, iam.ErrInvalidCredential)
	})
	t.Run("unknown user", func(t *testing.T) {
		_, err := svc.Login(context.Background(), authsvc.LoginInput{TenantSlug: "acme", Email: "nobody@acme.test", Password: "password123"})
		assert.ErrorIs(t, err, iam.ErrInvalidCredential)
	})
	t.Run("inactive user", func(t *testing.T) {
		store.SetUserActive(res.User.ID, false)

		_, err := svc.Login(context.Background(), authsvc.LoginInput{TenantSlug: "acme", Email: "owner@acme.test", Password: "password123"})
		assert.ErrorIs(t, err, iam.ErrUserInactive)
	})
}

func TestRefresh_RotatesAndDetectsReuse(t *testing.T) {
	svc, _ := newService()
	res1 := register(t, svc)

	res2, err := svc.Refresh(context.Background(), authsvc.RefreshInput{RefreshToken: res1.RefreshToken})
	require.NoError(t, err)
	assert.NotEqual(t, res1.RefreshToken, res2.RefreshToken, "refresh token must rotate")
	assert.NotEmpty(t, res2.AccessToken)

	// Reusing the old (now revoked) token is rejected and revokes the family.
	_, err = svc.Refresh(context.Background(), authsvc.RefreshInput{RefreshToken: res1.RefreshToken})
	assert.ErrorIs(t, err, iam.ErrInvalidCredential)

	// The new token was revoked by the reuse-detection sweep.
	_, err = svc.Refresh(context.Background(), authsvc.RefreshInput{RefreshToken: res2.RefreshToken})
	assert.ErrorIs(t, err, iam.ErrInvalidCredential)
}

func TestRefresh_Expired(t *testing.T) {
	svc, store := newService()
	res := register(t, svc)

	store.ExpireToken(token.HashRefresh(res.RefreshToken))

	_, err := svc.Refresh(context.Background(), authsvc.RefreshInput{RefreshToken: res.RefreshToken})
	assert.ErrorIs(t, err, iam.ErrInvalidCredential)
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, _ := newService()
	res := register(t, svc)

	require.NoError(t, svc.Logout(context.Background(), res.RefreshToken))

	_, err := svc.Refresh(context.Background(), authsvc.RefreshInput{RefreshToken: res.RefreshToken})
	assert.ErrorIs(t, err, iam.ErrInvalidCredential)
}

func TestListUsers_TenantScoped(t *testing.T) {
	svc, _ := newService()
	res := register(t, svc)

	users, total, err := svc.ListUsers(context.Background(), res.Tenant.ID, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)

	// A different tenant id sees nothing.
	_, total2, err := svc.ListUsers(context.Background(), 9999, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2)
}
