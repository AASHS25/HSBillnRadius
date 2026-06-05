package httpserver

import (
	"io"
	"log/slog"
)

// discardLogger returns a logger that drops all output, for use in tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
