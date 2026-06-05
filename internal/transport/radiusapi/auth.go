// Package radiusapi is the RADIUS delivery layer. It parses Access-Request
// packets with layeh.com/radius, verifies PAP/CHAP credentials against the
// looked-up cleartext password, and encodes Access-Accept/Reject replies with
// the user's reply attributes (including Mikrotik VSAs).
package radiusapi

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 is mandated by the CHAP protocol (RFC 1994)
	"crypto/subtle"
	"log/slog"
	"net"
	"strconv"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
	"layeh.com/radius/vendors/mikrotik"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
)

// AuthHandler binds the radius-service to the UDP server's packet handler.
type AuthHandler struct {
	svc *radiussvc.Service
	log *slog.Logger
}

// NewAuthHandler builds an AuthHandler.
func NewAuthHandler(svc *radiussvc.Service, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// Handle processes one raw datagram and returns the reply bytes (or nil to drop
// silently when the NAS or packet is invalid).
func (h *AuthHandler) Handle(ctx context.Context, remote *net.UDPAddr, raw []byte) ([]byte, error) {
	ip := remote.IP.String()

	nas, err := h.svc.ResolveNAS(ctx, ip)
	if err != nil {
		if radiussvc.IsNasUnknown(err) {
			h.log.WarnContext(ctx, "radius: unknown NAS, dropping", slog.String("nas_ip", ip))
			return nil, nil // silent drop
		}
		return nil, err
	}

	packet, err := radius.Parse(raw, []byte(nas.Secret))
	if err != nil {
		h.log.WarnContext(ctx, "radius: bad packet", slog.String("nas_ip", ip), slog.Any("error", err))
		return nil, nil
	}
	if packet.Code != radius.CodeAccessRequest {
		return nil, nil // accounting handled by the acct listener (M4)
	}

	username := rfc2865.UserName_GetString(packet)
	accepted := h.authenticate(ctx, nas.TenantID, username, packet)

	replyCode := radius.CodeAccessReject
	replyWord := "Access-Reject"
	if accepted {
		replyCode = radius.CodeAccessAccept
		replyWord = "Access-Accept"
	}

	resp := packet.Response(replyCode)
	if accepted {
		if info, err := h.svc.Lookup(ctx, nas.TenantID, username); err == nil {
			encodeReply(resp, info.Reply, h.log)
		}
	}

	// Post-auth audit (synchronous; runs inside the bounded worker).
	h.svc.RecordPostAuth(ctx, domainradius.PostAuth{
		TenantID: nas.TenantID,
		Username: username,
		Reply:    replyWord,
		NASIP:    ip,
	})

	encoded, err := resp.Encode()
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// authenticate verifies the supplied credentials (PAP or CHAP) against the
// stored cleartext password.
func (h *AuthHandler) authenticate(ctx context.Context, tenantID int64, username string, packet *radius.Packet) bool {
	info, err := h.svc.Lookup(ctx, tenantID, username)
	if err != nil {
		h.log.WarnContext(ctx, "radius: lookup failed", slog.Any("error", err))
		return false
	}
	if !info.Found {
		return false
	}

	if pw, err := rfc2865.UserPassword_Lookup(packet); err == nil {
		return subtle.ConstantTimeCompare(pw, []byte(info.ClearPassword)) == 1
	}
	if chap := rfc2865.CHAPPassword_Get(packet); len(chap) == 17 {
		return verifyCHAP(chap, challenge(packet), info.ClearPassword)
	}
	return false // no supported credential present
}

// challenge returns the CHAP challenge: the explicit attribute or, per RFC 2865,
// the Request Authenticator when no CHAP-Challenge attribute is present.
func challenge(packet *radius.Packet) []byte {
	if c := rfc2865.CHAPChallenge_Get(packet); len(c) > 0 {
		return c
	}
	return packet.Authenticator[:]
}

// verifyCHAP checks chap = [ident || MD5(ident || password || challenge)].
func verifyCHAP(chap, challenge []byte, password string) bool {
	ident := chap[0]
	h := md5.New() //nolint:gosec // CHAP requires MD5
	h.Write([]byte{ident})
	h.Write([]byte(password))
	h.Write(challenge)
	return subtle.ConstantTimeCompare(h.Sum(nil), chap[1:]) == 1
}

// encodeReply writes known reply attributes onto the response packet.
func encodeReply(packet *radius.Packet, attrs []domainradius.Attr, log *slog.Logger) {
	for _, a := range attrs {
		var err error
		switch a.Attribute {
		case domainradius.AttrMikrotikRateLimit:
			err = mikrotik.MikrotikRateLimit_SetString(packet, a.Value)
		case domainradius.AttrMikrotikAddrList:
			err = mikrotik.MikrotikAddressList_AddString(packet, a.Value)
		case "Mikrotik-Group":
			err = mikrotik.MikrotikGroup_SetString(packet, a.Value)
		case domainradius.AttrFramedPool:
			err = rfc2869.FramedPool_SetString(packet, a.Value)
		case "Session-Timeout":
			if n, convErr := strconv.Atoi(a.Value); convErr == nil {
				err = rfc2865.SessionTimeout_Set(packet, rfc2865.SessionTimeout(n))
			}
		default:
			log.Debug("radius: skipping unknown reply attribute", slog.String("attribute", a.Attribute))
		}
		if err != nil {
			log.Warn("radius: failed to set reply attribute", slog.String("attribute", a.Attribute), slog.Any("error", err))
		}
	}
}
