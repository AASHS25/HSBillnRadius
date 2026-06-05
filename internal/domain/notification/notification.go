// Package notification holds notification entities and the pure template
// renderer. Delivery (provider HTTP calls) lives in ports/wa; orchestration in
// notifysvc.
package notification

import (
	"errors"
	"strings"
)

// Channel is the delivery medium.
type Channel string

const (
	ChannelWA    Channel = "wa"
	ChannelEmail Channel = "email"
)

// Status is a notification's delivery state.
type Status string

const (
	StatusQueued Status = "queued"
	StatusSent   Status = "sent"
	StatusFailed Status = "failed"
)

// Provider is a WhatsApp gateway vendor.
type Provider string

const (
	ProviderFonnte     Provider = "fonnte"
	ProviderWablas     Provider = "wablas"
	ProviderStarsender Provider = "starsender"
	ProviderOnesender  Provider = "onesender"
	ProviderUnofficial Provider = "unofficial"
)

// Domain errors.
var (
	ErrNoGateway  = errors.New("no active notification gateway for tenant")
	ErrNoTemplate = errors.New("notification template not found")
	ErrNoProvider = errors.New("no client registered for provider")
)

// Job describes a notification to enqueue.
type Job struct {
	TenantID    int64
	Channel     Channel
	To          string
	TemplateKey string
	Vars        map[string]string
	DedupKey    string
}

// Gateway is a tenant's configured WA provider.
type Gateway struct {
	ID         int64
	TenantID   int64
	Provider   Provider
	Config     map[string]string // token, sender, base_url, ...
	IsActive   bool
	DailyLimit int
}

// Template is a message body with {placeholders}.
type Template struct {
	ID       int64
	TenantID int64
	Key      string
	Channel  Channel
	Body     string
	IsActive bool
}

// Log is a queued/sent/failed notification (also the job-queue row).
type Log struct {
	ID          int64
	TenantID    int64
	Channel     Channel
	To          string
	TemplateKey string
	Vars        map[string]string
	Status      Status
	ProviderRef string
	Error       string
	Attempts    int32
	DedupKey    string
}

// Render substitutes {key} placeholders in body with the given variables.
// Unknown placeholders are left untouched.
func Render(body string, vars map[string]string) string {
	if len(vars) == 0 {
		return body
	}
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(body)
}
