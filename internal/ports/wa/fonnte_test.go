package wa_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/ports/wa"
)

func TestFonnte_Send(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"status":true,"id":["msg-123"]}`))
	}))
	defer srv.Close()

	client := wa.NewFonnte(srv.Client())
	ref, err := client.Send(context.Background(), wa.Message{
		To:     "628123",
		Body:   "halo",
		Config: map[string]string{"token": "secret-token", "base_url": srv.URL},
	})
	require.NoError(t, err)
	assert.Equal(t, "msg-123", ref)
	assert.Equal(t, "secret-token", gotAuth)
	assert.Contains(t, gotBody, "target=628123")
	assert.Contains(t, gotBody, "message=halo")
}

func TestFonnte_Send_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":false,"reason":"invalid target"}`))
	}))
	defer srv.Close()

	_, err := wa.NewFonnte(srv.Client()).Send(context.Background(), wa.Message{
		To: "x", Body: "y", Config: map[string]string{"base_url": srv.URL},
	})
	assert.Error(t, err)
}
