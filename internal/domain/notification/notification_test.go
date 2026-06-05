package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	body := "Halo {nama}, tagihan {tagihan} jatuh tempo {jatuh_tempo}."
	out := Render(body, map[string]string{
		"nama":        "Budi",
		"tagihan":     "150000",
		"jatuh_tempo": "2026-06-08",
	})
	assert.Equal(t, "Halo Budi, tagihan 150000 jatuh tempo 2026-06-08.", out)
}

func TestRender_NoVarsAndUnknown(t *testing.T) {
	assert.Equal(t, "plain", Render("plain", nil))
	assert.Equal(t, "keep {unknown}", Render("keep {unknown}", map[string]string{"x": "y"}))
}
