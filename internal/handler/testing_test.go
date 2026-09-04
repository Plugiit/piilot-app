package handler

import (
	"io"
	"log/slog"
)

// discardLogger evite de polluer la sortie des tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
