package reporter

import (
	"log/slog"
)

// Report logs an error using slog.Error.
// All error reporting in the project should funnel through this function.
func Report(msg string, args ...any) {
	slog.Error(msg, args...)
}
