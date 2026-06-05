package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"unknown": slog.LevelInfo,
		"":        slog.LevelInfo,
	}
	for in, want := range cases {
		assert.Equal(t, want, parseLevel(in), "level %q", in)
	}
}

func TestNew_DoesNotPanic(t *testing.T) {
	assert.NotNil(t, New(Options{Level: "debug", Format: "json", AppName: "test", Env: "development"}))
	assert.NotNil(t, New(Options{Level: "info", Format: "text", AppName: "test", Env: "development"}))
}
