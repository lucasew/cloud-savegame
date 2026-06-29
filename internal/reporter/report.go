package reporter

import (
	"log/slog"
)

// Report provides a centralized error-reporting function.
// All code paths that handle unexpected errors MUST funnel through this function.
func Report(msg string, args ...any) {
	slog.Error(msg, args...)
}
