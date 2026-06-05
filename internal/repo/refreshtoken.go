package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type refreshTokenRepo struct{ q *sqlc.Queries }

func (r *refreshTokenRepo) Create(ctx context.Context, t iam.RefreshToken) (iam.RefreshToken, error) {
	m, err := r.q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		UserID:    t.UserID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		UserAgent: t.UserAgent,
		Ip:        pgTextOrNull(t.IP),
	})
	if err != nil {
		return iam.RefreshToken{}, fmt.Errorf("create refresh token: %w", err)
	}
	return toDomainRefreshToken(m), nil
}

func (r *refreshTokenRepo) GetByHash(ctx context.Context, hash string) (iam.RefreshToken, error) {
	m, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if isNotFound(err) {
			return iam.RefreshToken{}, iam.ErrTokenNotFound
		}
		return iam.RefreshToken{}, fmt.Errorf("get refresh token: %w", err)
	}
	return toDomainRefreshToken(m), nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, hash string) error {
	if err := r.q.RevokeRefreshToken(ctx, hash); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *refreshTokenRepo) RevokeAllForUser(ctx context.Context, userID int64) error {
	if err := r.q.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return nil
}

func (r *refreshTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	n, err := r.q.DeleteExpiredRefreshTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return n, nil
}
