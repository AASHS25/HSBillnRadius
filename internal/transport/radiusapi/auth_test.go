package radiusapi_test

import (
	"context"
	"crypto/md5" //nolint:gosec // CHAP test vectors require MD5
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/vendors/mikrotik"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
	"github.com/aashs25/hsbillnradius/internal/transport/radiusapi"
)

const (
	nasIP    = "127.0.0.1"
	secret   = "testing123"
	username = "alice"
	password = "wonderland"
)

func newHandler(t *testing.T) *radiusapi.AuthHandler {
	t.Helper()
	store := memrepo.New()
	store.SeedNas(domainradius.Nas{TenantID: 1, Name: nasIP, Secret: secret})

	r := store.Repositories()
	ctx := context.Background()
	require.NoError(t, r.RadiusMap.SetUserPassword(ctx, 1, username, password))
	require.NoError(t, r.RadiusMap.SetUserGroup(ctx, 1, username, "plan_1", 1))
	require.NoError(t, r.RadiusMap.SetGroupReply(ctx, 1, "plan_1", []domainradius.Attr{
		{Attribute: domainradius.AttrMikrotikRateLimit, Op: ":=", Value: "10M/2M"},
	}))

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := radiussvc.New(r, cache.Noop{}, time.Minute, time.Minute, log)
	return radiusapi.NewAuthHandler(svc, log)
}

func remote(ip string) *net.UDPAddr { return &net.UDPAddr{IP: net.ParseIP(ip), Port: 41000} }

func papRequest(t *testing.T, user, pass string) []byte {
	t.Helper()
	p := radius.New(radius.CodeAccessRequest, []byte(secret))
	require.NoError(t, rfc2865.UserName_SetString(p, user))
	require.NoError(t, rfc2865.UserPassword_SetString(p, pass))
	raw, err := p.Encode()
	require.NoError(t, err)
	return raw
}

func parseReply(t *testing.T, raw []byte) *radius.Packet {
	t.Helper()
	p, err := radius.Parse(raw, []byte(secret))
	require.NoError(t, err)
	return p
}

func TestHandle_PAP_Accept(t *testing.T) {
	h := newHandler(t)
	reply, err := h.Handle(context.Background(), remote(nasIP), papRequest(t, username, password))
	require.NoError(t, err)
	require.NotNil(t, reply)

	resp := parseReply(t, reply)
	assert.Equal(t, radius.CodeAccessAccept, resp.Code)
	assert.Equal(t, "10M/2M", mikrotik.MikrotikRateLimit_GetString(resp), "rate-limit VSA should be present")
}

func TestHandle_PAP_WrongPassword_Reject(t *testing.T) {
	h := newHandler(t)
	reply, err := h.Handle(context.Background(), remote(nasIP), papRequest(t, username, "bad-pass"))
	require.NoError(t, err)
	assert.Equal(t, radius.CodeAccessReject, parseReply(t, reply).Code)
}

func TestHandle_UnknownUser_Reject(t *testing.T) {
	h := newHandler(t)
	reply, err := h.Handle(context.Background(), remote(nasIP), papRequest(t, "nobody", "x"))
	require.NoError(t, err)
	assert.Equal(t, radius.CodeAccessReject, parseReply(t, reply).Code)
}

func TestHandle_UnknownNAS_Drop(t *testing.T) {
	h := newHandler(t)
	reply, err := h.Handle(context.Background(), remote("203.0.113.9"), papRequest(t, username, password))
	require.NoError(t, err)
	assert.Nil(t, reply, "packets from an unregistered NAS are dropped silently")
}

func TestHandle_CHAP_Accept(t *testing.T) {
	h := newHandler(t)

	p := radius.New(radius.CodeAccessRequest, []byte(secret))
	require.NoError(t, rfc2865.UserName_SetString(p, username))
	chal := []byte("0123456789abcdef")
	ident := byte(7)
	sum := md5.Sum(append(append([]byte{ident}, []byte(password)...), chal...)) //nolint:gosec
	p.Add(3, radius.Attribute(append([]byte{ident}, sum[:]...)))                // CHAP-Password
	p.Add(60, radius.Attribute(chal))                                           // CHAP-Challenge
	raw, err := p.Encode()
	require.NoError(t, err)

	reply, err := h.Handle(context.Background(), remote(nasIP), raw)
	require.NoError(t, err)
	assert.Equal(t, radius.CodeAccessAccept, parseReply(t, reply).Code)
}

func BenchmarkHandle_PAP(b *testing.B) {
	store := memrepo.New()
	store.SeedNas(domainradius.Nas{TenantID: 1, Name: nasIP, Secret: secret})
	r := store.Repositories()
	ctx := context.Background()
	_ = r.RadiusMap.SetUserPassword(ctx, 1, username, password)
	_ = r.RadiusMap.SetUserGroup(ctx, 1, username, "plan_1", 1)
	_ = r.RadiusMap.SetGroupReply(ctx, 1, "plan_1", []domainradius.Attr{{Attribute: domainradius.AttrMikrotikRateLimit, Op: ":=", Value: "10M/2M"}})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := radiussvc.New(r, cache.Noop{}, time.Minute, time.Minute, log)
	h := radiusapi.NewAuthHandler(svc, log)

	p := radius.New(radius.CodeAccessRequest, []byte(secret))
	_ = rfc2865.UserName_SetString(p, username)
	_ = rfc2865.UserPassword_SetString(p, password)
	raw, _ := p.Encode()
	addr := remote(nasIP)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.Handle(ctx, addr, raw); err != nil {
			b.Fatal(err)
		}
	}
}
