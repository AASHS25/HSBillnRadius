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

// defaultWablasURL is the Wablas domain; the send path is appended.
const defaultWablasURL = "https://wablas.com"

// Wablas implements Client for the Wablas WhatsApp gateway.
type Wablas struct{ http *http.Client }

// NewWablas builds a Wablas client.
func NewWablas(httpClient *http.Client) *Wablas { return &Wablas{http: httpClient} }

// Send posts phone/message with the token in the Authorization header.
func (w *Wablas) Send(ctx context.Context, msg Message) (string, error) {
	base := msg.Config["base_url"]
	if base == "" {
		base = defaultWablasURL
	}
	endpoint := strings.TrimRight(base, "/") + "/api/send-message"

	form := url.Values{"phone": {msg.To}, "message": {msg.Body}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("wablas build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", msg.Config["token"])

	resp, err := w.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("wablas send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("wablas http %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Status bool `json:"status"`
		Data   struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", nil
	}
	if !out.Status {
		return "", fmt.Errorf("wablas rejected: %s", string(body))
	}
	if len(out.Data.Messages) > 0 {
		return out.Data.Messages[0].ID, nil
	}
	return "", nil
}
