package backup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/backup"
	"github.com/lucasew/cloud-savegame/internal/config"
)

func TestIsPathIgnored(t *testing.T) {
	ignoreMe := filepath.Join(t.TempDir(), "ignore", "me")
	alsoIgnore := filepath.Join(t.TempDir(), "also", "ignore")

	eng := &backup.Engine{
		IgnoredPaths: []string{ignoreMe, alsoIgnore},
	}

	subfile := filepath.Join(ignoreMe, "subfile")
	if !eng.IsPathIgnored(subfile) {
		t.Errorf("Expected %s to be ignored", subfile)
	}
	if !eng.IsPathIgnored(ignoreMe) {
		t.Errorf("Expected exact ignored path %s to be ignored", ignoreMe)
	}
	keepMe := filepath.Join(t.TempDir(), "keep", "me")
	if eng.IsPathIgnored(keepMe) {
		t.Errorf("Expected %s to be kept", keepMe)
	}

	// Sibling prefix must not match: ignore "/foo" must not ignore "/foobar"
	root := t.TempDir()
	prefix := filepath.Join(root, "foo")
	sibling := filepath.Join(root, "foobar", "nested")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	engSibling := &backup.Engine{IgnoredPaths: []string{prefix}}
	if engSibling.IsPathIgnored(sibling) {
		t.Errorf("Expected sibling path %s not to match ignore prefix %s", sibling, prefix)
	}
}

func TestIngestPathSecurity(t *testing.T) {
	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)

	basePath := filepath.Join(t.TempDir(), "safe", "base")
	unsafePath := filepath.Join(basePath, "..", "..", "unsafe")

	eng.IngestPath(t.Context(), "app", "rule", unsafePath, false, basePath)

	if len(eng.NewsList) == 0 {
		t.Error("Expected security warning for unsafe path traversal")
	} else {
		msg := eng.NewsList[0]
		if !strings.Contains(msg, "resolves outside of its base") {
			t.Errorf("Unexpected warning message: %s", msg)
		}
	}
}
