package wa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// defaultFonnteURL is the Fonnte send endpoint; overridable via config base_url.
const defaultFonnteURL = "https://api.fonnte.com/send"

// Fonnte implements Client for the Fonnte WhatsApp gateway.
type Fonnte struct{ http *http.Client }

// NewFonnte builds a Fonnte client using the given HTTP client (with timeout).
func NewFonnte(httpClient *http.Client) *Fonnte { return &Fonnte{http: httpClient} }

// Send posts target/message form data with the token in the Authorization header.
func (f *Fonnte) Send(ctx context.Context, msg Message) (string, error) {
	endpoint := msg.Config["base_url"]
	if endpoint == "" {
		endpoint = defaultFonnteURL
	}

	form := url.Values{"target": {msg.To}, "message": {msg.Body}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("fonnte build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", msg.Config["token"])

	resp, err := f.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fonnte send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fonnte http %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Status bool   `json:"status"`
		ID     []any  `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", nil // accepted but unparsable body; treat as sent without ref
	}
	if !out.Status {
		return "", fmt.Errorf("fonnte rejected: %s", out.Reason)
	}
	if len(out.ID) > 0 {
		return fmt.Sprintf("%v", out.ID[0]), nil
	}
	return "", nil
}
