package backup_test

import (
	"strings"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/backup"
	"github.com/lucasew/cloud-savegame/internal/config"
)

func TestIngestPathSecurityNoBase(t *testing.T) {
	eng := backup.NewEngine(config.New(), nil, nil, "/tmp/output")

	// "../secret" relative to CWD should be blocked if we enforce CWD as base.
	// Currently it is NOT blocked.
	eng.IngestPath("app", "rule", "../secret", false, "")

	found := false
	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "resolves outside of its base") {
			found = true
			break
		}
	}

	if !found {
		t.Error("VULNERABILITY CONFIRMED: Expected security warning for relative path traversal without base, but got none.")
	}
}
