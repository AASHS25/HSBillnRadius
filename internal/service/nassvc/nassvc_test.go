package nassvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/nassvc"
)

func newSvc() *nassvc.Service {
	return nassvc.New(memrepo.New().Repositories(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateAndList(t *testing.T) {
	store := memrepo.New()
	svc := nassvc.New(store.Repositories(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	n, err := svc.Create(ctx, 1, "10.0.0.1", "mikrotik", "radsecret")
	require.NoError(t, err)
	assert.Positive(t, n.ID)
	assert.Equal(t, "10.0.0.1", n.Name)

	list, err := svc.List(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "radsecret", list[0].Secret)

	// Different tenant sees nothing.
	other, err := svc.List(ctx, 2)
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestCreate_RequiresSecret(t *testing.T) {
	_, err := newSvc().Create(context.Background(), 1, "10.0.0.1", "mt", "")
	assert.Error(t, err)
}
