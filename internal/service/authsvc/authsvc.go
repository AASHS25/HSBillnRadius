// Package authsvc implements the authentication use-cases: tenant signup,
// login, refresh-token rotation and logout. It orchestrates the repository
// ports with the password hasher and token issuer; it owns no I/O itself.
package authsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Hasher hashes and verifies passwords.
type Hasher interface {
	Hash(plain string) (string, error)
	Verify(plain, encoded string) (bool, error)
}

// TokenIssuer signs access tokens.
type TokenIssuer interface {
	IssueAccess(in token.AccessInput) (string, time.Time, error)
}

// Service provides the authentication use-cases.
type Service struct {
	repos      repo.Repositories
	tx         repo.TxManager
	hasher     Hasher
	tokens     TokenIssuer
	refreshTTL time.Duration
	log        *slog.Logger
	now        func() time.Time
}

// New constructs an auth Service.
func New(repos repo.Repositories, tx repo.TxManager, hasher Hasher, tokens TokenIssuer, refreshTTL time.Duration, log *slog.Logger) *Service {
	return &Service{
		repos:      repos,
		tx:         tx,
		hasher:     hasher,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		log:        log,
		now:        time.Now,
	}
}

// RegisterInput is the self-service signup request: it creates a new tenant and
// its first (owner) user.
type RegisterInput struct {
	TenantName string
	TenantSlug string
	Name       string
	Email      string
	Password   string
	UserAgent  string
	IP         string
}

// LoginInput authenticates an existing user within a tenant (resolved by slug).
type LoginInput struct {
	TenantSlug string
	Email      string
	Password   string
	UserAgent  string
	IP         string
}

// RefreshInput rotates a refresh token for a fresh session.
type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IP           string
}

// AuthResult is the outcome of a successful authentication.
type AuthResult struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	User            iam.User
	Tenant          tenant.Tenant
	Permissions     []string
}

// Register provisions a new tenant with an owner user and returns a session.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if in.TenantName == "" || in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", iam.ErrInvalidEmail)
	}
	if err := tenant.ValidateSlug(in.TenantSlug); err != nil {
		return nil, err
	}
	email := iam.NormalizeEmail(in.Email)
	if err := iam.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := iam.ValidatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var result *AuthResult
	err = s.tx.WithTx(ctx, func(r repo.Repositories) error {
		t, err := r.Tenant.Create(ctx, tenant.Tenant{
			Name:   in.TenantName,
			Slug:   in.TenantSlug,
			Status: tenant.StatusTrial,
			Plan:   "free",
		})
		if err != nil {
			return err
		}

		role, err := r.Role.Create(ctx, iam.Role{TenantID: t.ID, Name: iam.DefaultOwnerRole, IsSystem: true})
		if err != nil {
			return err
		}
		if err := r.Role.GrantAllPermissions(ctx, role.ID); err != nil {
			return err
		}

		user, err := r.User.Create(ctx, iam.User{
			TenantID:     t.ID,
			Name:         in.Name,
			Email:        email,
			PasswordHash: hash,
			RoleID:       role.ID,
		})
		if err != nil {
			return err
		}

		perms, err := r.Role.PermissionCodes(ctx, role.ID)
		if err != nil {
			return err
		}

		s.audit(ctx, r, t.ID, &user.ID, "tenant.register", "tenant", strconv.FormatInt(t.ID, 10), in.IP)

		result, err = s.issueSession(ctx, r, user, t, perms, in.UserAgent, in.IP)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Login authenticates a user and returns a new session.
func (s *Service) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	t, err := s.repos.Tenant.GetBySlug(ctx, in.TenantSlug)
	if err != nil {
		if errors.Is(err, tenant.ErrNotFound) {
			return nil, iam.ErrInvalidCredential // avoid tenant enumeration
		}
		return nil, err
	}

	email := iam.NormalizeEmail(in.Email)
	user, err := s.repos.User.GetByEmail(ctx, t.ID, email)
	if err != nil {
		if errors.Is(err, iam.ErrUserNotFound) {
			return nil, iam.ErrInvalidCredential
		}
		return nil, err
	}

	ok, err := s.hasher.Verify(in.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return nil, iam.ErrInvalidCredential
	}
	if !user.IsActive {
		return nil, iam.ErrUserInactive
	}

	perms, err := s.repos.Role.PermissionCodes(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	result, err := s.issueSession(ctx, s.repos, user, t, perms, in.UserAgent, in.IP)
	if err != nil {
		return nil, err
	}
	if err := s.repos.User.UpdateLastLogin(ctx, user.ID); err != nil {
		s.log.WarnContext(ctx, "update last login failed", slog.Any("error", err), slog.Int64("user_id", user.ID))
	}
	s.audit(ctx, s.repos, t.ID, &user.ID, "auth.login", "user", strconv.FormatInt(user.ID, 10), in.IP)
	return result, nil
}

// Refresh validates a refresh token, rotates it and returns a new session.
// A presented but already-revoked token triggers revocation of all the user's
// tokens (reuse detection).
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (*AuthResult, error) {
	hash := token.HashRefresh(in.RefreshToken)

	stored, err := s.repos.RefreshToken.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, iam.ErrTokenNotFound) {
			return nil, iam.ErrInvalidCredential
		}
		return nil, err
	}

	if stored.RevokedAt != nil {
		// Token reuse: revoke the whole family defensively.
		if rErr := s.repos.RefreshToken.RevokeAllForUser(ctx, stored.UserID); rErr != nil {
			s.log.ErrorContext(ctx, "revoke-all on token reuse failed", slog.Any("error", rErr))
		}
		return nil, iam.ErrInvalidCredential
	}
	if !stored.Active(s.now()) {
		return nil, iam.ErrInvalidCredential
	}

	user, err := s.repos.User.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, iam.ErrUserInactive
	}

	t, err := s.repos.Tenant.GetByID(ctx, user.TenantID)
	if err != nil {
		return nil, err
	}
	perms, err := s.repos.Role.PermissionCodes(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	var result *AuthResult
	err = s.tx.WithTx(ctx, func(r repo.Repositories) error {
		if err := r.RefreshToken.Revoke(ctx, hash); err != nil {
			return err
		}
		result, err = s.issueSession(ctx, r, user, t, perms, in.UserAgent, in.IP)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Logout revokes a refresh token. It is idempotent.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if err := s.repos.RefreshToken.Revoke(ctx, token.HashRefresh(refreshToken)); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// Profile returns the current user with their tenant and permission codes.
func (s *Service) Profile(ctx context.Context, userID int64) (iam.User, tenant.Tenant, []string, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		return iam.User{}, tenant.Tenant{}, nil, err
	}
	t, err := s.repos.Tenant.GetByID(ctx, user.TenantID)
	if err != nil {
		return iam.User{}, tenant.Tenant{}, nil, err
	}
	perms, err := s.repos.Role.PermissionCodes(ctx, user.RoleID)
	if err != nil {
		return iam.User{}, tenant.Tenant{}, nil, err
	}
	return user, t, perms, nil
}

// ListUsers returns a page of users scoped to the tenant, with the total count.
func (s *Service) ListUsers(ctx context.Context, tenantID int64, limit, offset int32) ([]iam.User, int64, error) {
	users, err := s.repos.User.List(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repos.User.Count(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// ListAuditLogs returns a page of audit entries scoped to the tenant.
func (s *Service) ListAuditLogs(ctx context.Context, tenantID int64, limit, offset int32) ([]audit.Entry, error) {
	return s.repos.Audit.List(ctx, tenantID, limit, offset)
}

// issueSession mints an access token plus a stored refresh token for the user.
func (s *Service) issueSession(ctx context.Context, r repo.Repositories, u iam.User, t tenant.Tenant, perms []string, userAgent, ip string) (*AuthResult, error) {
	access, expiresAt, err := s.tokens.IssueAccess(token.AccessInput{
		TenantID:    u.TenantID,
		UserID:      u.ID,
		RoleID:      u.RoleID,
		Permissions: perms,
	})
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	plaintext, hash, err := token.GenerateRefresh()
	if err != nil {
		return nil, err
	}
	if _, err := r.RefreshToken.Create(ctx, iam.RefreshToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: s.now().Add(s.refreshTTL),
		UserAgent: userAgent,
		IP:        ip,
	}); err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken:     access,
		AccessExpiresAt: expiresAt,
		RefreshToken:    plaintext,
		User:            u,
		Tenant:          t,
		Permissions:     perms,
	}, nil
}

// audit records a mutating action, logging (but not failing) on error.
func (s *Service) audit(ctx context.Context, r repo.Repositories, tenantID int64, actor *int64, action, entity, entityID, ip string) {
	if _, err := r.Audit.Insert(ctx, audit.Entry{
		TenantID:    tenantID,
		ActorUserID: actor,
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		IP:          ip,
	}); err != nil {
		s.log.WarnContext(ctx, "audit insert failed", slog.Any("error", err), slog.String("action", action))
	}
}
