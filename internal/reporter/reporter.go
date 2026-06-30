package reporter

import (
	"log/slog"
)

// Report logs an error using the application's central reporting mechanism.
func Report(msg string, args ...any) {
	slog.Error(msg, args...)
}
