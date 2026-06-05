// Package wa defines the WhatsApp client port and provider implementations.
// Each provider reads its credentials from the per-tenant gateway config.
package wa

import "context"

// Message is an outbound WhatsApp message plus the gateway config to send it.
type Message struct {
	To     string
	Body   string
	Config map[string]string // token, sender, base_url, ...
}

// Client sends a WhatsApp message and returns the provider's reference id.
type Client interface {
	Send(ctx context.Context, msg Message) (providerRef string, err error)
}
