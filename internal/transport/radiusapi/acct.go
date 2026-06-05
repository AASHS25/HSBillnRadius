package radiusapi

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"time"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/rfc2869"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
)

// AcctHandler parses Accounting-Request packets and enqueues them to the writer,
// replying with Accounting-Response immediately.
type AcctHandler struct {
	svc    *radiussvc.Service
	writer *radiussvc.AcctWriter
	log    *slog.Logger
}

// NewAcctHandler builds an AcctHandler.
func NewAcctHandler(svc *radiussvc.Service, writer *radiussvc.AcctWriter, log *slog.Logger) *AcctHandler {
	return &AcctHandler{svc: svc, writer: writer, log: log}
}

// Handle processes one accounting datagram.
func (h *AcctHandler) Handle(ctx context.Context, remote *net.UDPAddr, raw []byte) ([]byte, error) {
	ip := remote.IP.String()

	nas, err := h.svc.ResolveNAS(ctx, ip)
	if err != nil {
		if radiussvc.IsNasUnknown(err) {
			return nil, nil
		}
		return nil, err
	}

	packet, err := radius.Parse(raw, []byte(nas.Secret))
	if err != nil {
		h.log.WarnContext(ctx, "radius acct: bad packet", slog.String("nas_ip", ip), slog.Any("error", err))
		return nil, nil
	}
	if packet.Code != radius.CodeAccountingRequest {
		return nil, nil
	}

	username := rfc2865.UserName_GetString(packet)
	sessionID := rfc2866.AcctSessionID_GetString(packet)

	e := domainradius.AcctEvent{
		TenantID:       nas.TenantID,
		SessionID:      sessionID,
		UniqueID:       domainradius.ComputeUniqueID(sessionID, ip, username),
		Username:       username,
		NASIP:          ip,
		StartTime:      time.Now().Truncate(time.Second),
		SessionTime:    int64(rfc2866.AcctSessionTime_Get(packet)),
		InputOctets:    octets(uint32(rfc2866.AcctInputOctets_Get(packet)), uint32(rfc2869.AcctInputGigawords_Get(packet))),
		OutputOctets:   octets(uint32(rfc2866.AcctOutputOctets_Get(packet)), uint32(rfc2869.AcctOutputGigawords_Get(packet))),
		FramedIP:       framedIP(packet),
		CallingStation: rfc2865.CallingStationID_GetString(packet),
		CalledStation:  rfc2865.CalledStationID_GetString(packet),
	}

	switch rfc2866.AcctStatusType_Get(packet) {
	case rfc2866.AcctStatusType_Value_Start:
		e.Status = domainradius.AcctStart
	case rfc2866.AcctStatusType_Value_InterimUpdate:
		e.Status = domainradius.AcctInterim
	case rfc2866.AcctStatusType_Value_Stop:
		e.Status = domainradius.AcctStop
		if tc := rfc2866.AcctTerminateCause_Get(packet); tc != 0 {
			e.TerminateCause = strconv.Itoa(int(tc))
		}
	default:
		return ack(packet) // Accounting-On/Off and others: just acknowledge
	}

	if !h.writer.Submit(e) {
		h.log.WarnContext(ctx, "accounting queue full, dropping event", slog.String("session", sessionID))
	}
	return ack(packet)
}

// ack encodes an empty Accounting-Response.
func ack(packet *radius.Packet) ([]byte, error) {
	return packet.Response(radius.CodeAccountingResponse).Encode()
}

// octets combines the 32-bit octet counter with its gigawords high word.
func octets(low, gigawords uint32) int64 {
	return int64(low) | int64(gigawords)<<32
}

// framedIP returns the Framed-IP-Address as a string, or "" when absent.
func framedIP(packet *radius.Packet) string {
	if ip := rfc2865.FramedIPAddress_Get(packet); ip != nil {
		return ip.String()
	}
	return ""
}
