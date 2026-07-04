package reporter

import (
	"log/slog"
)

// Report logs an error using the application's standard structured logging.
// This serves as the centralized error reporting mechanism for the project,
// fulfilling the requirement to avoid scattered, direct logging of errors.
// All error paths should route through this function.
func Report(msg string, args ...any) {
	slog.Error(msg, args...)
}
