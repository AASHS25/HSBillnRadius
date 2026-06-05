// Package repo implements the repository ports (internal/ports/repo) on top of
// sqlc-generated queries and a pgx pool. The sqlc dependency is confined to
// this package; the service layer sees only domain types and port interfaces.
package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aashs25/hsbillnradius/internal/platform/postgres"
	port "github.com/aashs25/hsbillnradius/internal/ports/repo"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

// Store is the root repository provider. It builds repositories over the pool
// for normal use and over a transaction for WithTx.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Repositories returns repositories that run each call on the pool.
func (s *Store) Repositories() port.Repositories {
	return buildRepos(sqlc.New(s.pool))
}

// WithTx runs fn inside a transaction, providing repositories bound to it.
func (s *Store) WithTx(ctx context.Context, fn func(r port.Repositories) error) error {
	return postgres.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(buildRepos(sqlc.New(tx)))
	})
}

// buildRepos wires every repository over a single sqlc querier (pool or tx).
func buildRepos(q *sqlc.Queries) port.Repositories {
	return port.Repositories{
		Tenant:       &tenantRepo{q: q},
		User:         &userRepo{q: q},
		Role:         &roleRepo{q: q},
		Permission:   &permissionRepo{q: q},
		RefreshToken: &refreshTokenRepo{q: q},
		Audit:        &auditRepo{q: q},
	}
}

// isNotFound reports whether err is pgx's no-rows sentinel.
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Ensure Store satisfies the TxManager port.
var _ port.TxManager = (*Store)(nil)
