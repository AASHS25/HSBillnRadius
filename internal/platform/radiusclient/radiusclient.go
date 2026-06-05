// Package radiusclient sends CoA/Disconnect requests to a NAS (UDP 3799),
// used by auto-isolir/restore to kick a session so it reconnects into its new
// group.
package radiusclient

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
)

// Client sends Disconnect-Request packets to NAS devices.
type Client struct {
	coaPort int
	timeout time.Duration
}

// New builds a CoA client targeting the given NAS CoA port.
func New(coaPort int, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Client{coaPort: coaPort, timeout: timeout}
}

// Disconnect sends a Disconnect-Request identifying the session by username and
// Acct-Session-Id, and returns an error unless the NAS replies Disconnect-ACK.
func (c *Client) Disconnect(ctx context.Context, nas domainradius.Nas, username, sessionID string) error {
	packet := radius.New(radius.CodeDisconnectRequest, []byte(nas.Secret))
	if username != "" {
		if err := rfc2865.UserName_SetString(packet, username); err != nil {
			return fmt.Errorf("set user-name: %w", err)
		}
	}
	if sessionID != "" {
		if err := rfc2866.AcctSessionID_SetString(packet, sessionID); err != nil {
			return fmt.Errorf("set acct-session-id: %w", err)
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	addr := net.JoinHostPort(nas.Name, strconv.Itoa(c.coaPort))
	resp, err := radius.Exchange(reqCtx, packet, addr)
	if err != nil {
		return fmt.Errorf("disconnect exchange to %s: %w", addr, err)
	}
	if resp.Code != radius.CodeDisconnectACK {
		return fmt.Errorf("disconnect not acked by %s: %s", addr, resp.Code)
	}
	return nil
}
