package backup_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/backup"
	"github.com/lucasew/cloud-savegame/internal/config"
)

func TestIsPathIgnored(t *testing.T) {
	eng := &backup.Engine{
		IgnoredPaths: []string{"/ignore/me", "/also/ignore"},
	}

	if !eng.IsPathIgnored("/ignore/me/subfile") {
		t.Error("Expected /ignore/me/subfile to be ignored")
	}
	if eng.IsPathIgnored("/keep/me") {
		t.Error("Expected /keep/me to be kept")
	}
}

func TestIngestPathSecurity(t *testing.T) {
	eng := backup.NewEngine(config.New(), nil, nil, "/tmp/output")

	basePath := "/safe/base"
	// Path resolves to /safe/base/../../unsafe -> /unsafe
	// But we need filepath.Abs to work. On linux /safe/base might not exist.
	// filepath.Abs cleans paths.
	// If input is "/safe/base/../unsafe", Abs("/safe/base/../unsafe") -> "/safe/unsafe".
	// If input is "/safe/base/../../unsafe", Abs -> "/unsafe".

	unsafePath := filepath.Join(basePath, "..", "..", "unsafe")

	eng.IngestPath("app", "rule", unsafePath, false, basePath)

	if len(eng.NewsList) == 0 {
		t.Error("Expected security warning for unsafe path traversal")
	} else {
		msg := eng.NewsList[0]
		if !strings.Contains(msg, "resolves outside of its base") {
			t.Errorf("Unexpected warning message: %s", msg)
		}
	}
}
