package backup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"errors"
	"github.com/lucasew/cloud-savegame/internal/backup"
	"github.com/lucasew/cloud-savegame/internal/config"
	"io/fs"
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

// withDeletedCWD runs fn after chdir into a directory that is then removed so
// filepath.Abs of relative paths fails (Getwd cannot resolve the cwd).
func withDeletedCWD(t *testing.T, fn func()) {
	t.Helper()
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	fn()
}

func TestIngestPathFailsClosedWhenAbsFails(t *testing.T) {
	// Security checks must not be skipped when filepath.Abs cannot resolve.
	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)

	withDeletedCWD(t, func() {
		eng.IngestPath(t.Context(), "app", "rule", "relative/path", false, "relative/base")
	})

	if len(eng.NewsList) == 0 {
		t.Fatal("expected security warning when path Abs fails")
	}
	if !strings.Contains(eng.NewsList[0], "cannot resolve") {
		t.Fatalf("unexpected warning: %s", eng.NewsList[0])
	}
}

// TestCopyItemSurfacesLstatErrors checks that a source path whose Lstat fails
// for a reason other than NotExist produces WarningNews instead of looking
// like "nothing to copy".
func TestCopyItemSurfacesLstatErrors(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	// Child under an unreadable parent: Lstat fails with permission (not NotExist).
	secret := filepath.Join(blocked, "save.dat")
	if _, err := os.Lstat(secret); err == nil {
		t.Skip("path is lstat-able; cannot exercise inaccessible Lstat")
	} else if errors.Is(err, fs.ErrNotExist) {
		t.Skip("platform reports NotExist for unreadable parent; cannot exercise")
	}

	outDir := t.TempDir()
	dest := filepath.Join(outDir, "app", "rule", "save.dat")
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	eng.CopyItem(secret, dest, outDir, 0)

	if len(eng.NewsList) == 0 {
		t.Fatal("expected WarningNews when CopyItem cannot Lstat source")
	}
	if !strings.Contains(eng.NewsList[0], "Failed to access path") {
		t.Fatalf("unexpected warning: %s", eng.NewsList[0])
	}
	if !strings.Contains(eng.NewsList[0], secret) {
		t.Fatalf("warning should mention path %s: %s", secret, eng.NewsList[0])
	}
}

// TestCopyItemMissingPathIsSilent checks that a missing source does not warn.
func TestCopyItemMissingPathIsSilent(t *testing.T) {
	outDir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no-such-file")
	dest := filepath.Join(outDir, "app", "rule", "file")
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	eng.CopyItem(missing, dest, outDir, 0)

	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "Failed to access path") {
			t.Fatalf("missing path must not warn as inaccessible: %s", msg)
		}
	}
}

// TestCopyItemSurfacesReadDirErrors checks that a directory whose contents
// cannot be listed produces WarningNews instead of a silent partial copy.
func TestCopyItemSurfacesReadDirErrors(t *testing.T) {
	srcRoot := t.TempDir()
	blocked := filepath.Join(srcRoot, "blocked")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	// If we can still read the dir (e.g. running as root), skip.
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("directory is readable; cannot exercise ReadDir failure")
	}

	outDir := t.TempDir()
	dest := filepath.Join(outDir, "app", "rule")
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	eng.CopyItem(blocked, dest, outDir, 0)

	if len(eng.NewsList) == 0 {
		t.Fatal("expected WarningNews when CopyItem cannot ReadDir source")
	}
	if !strings.Contains(eng.NewsList[0], "Failed to list directory") {
		t.Fatalf("unexpected warning: %s", eng.NewsList[0])
	}
	if !strings.Contains(eng.NewsList[0], blocked) {
		t.Fatalf("warning should mention path %s: %s", blocked, eng.NewsList[0])
	}
}

// TestIngestPathSurfacesInaccessibleStat checks that a concrete rule path
// whose Stat fails for a reason other than NotExist produces WarningNews
// instead of looking like "game not installed".
func TestIngestPathSurfacesInaccessibleStat(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	// Child under an unreadable parent: Stat fails with permission (not NotExist).
	secret := filepath.Join(blocked, "save.dat")
	if _, err := os.Stat(secret); err == nil {
		t.Skip("path is stat-able; cannot exercise inaccessible Stat")
	} else if errors.Is(err, fs.ErrNotExist) {
		t.Skip("platform reports NotExist for unreadable parent; cannot exercise")
	}

	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	// basePath empty would reject absolute pathStr; pass base so security allows it.
	eng.IngestPath(t.Context(), "test-app", "saves", secret, true, blocked)

	if len(eng.NewsList) == 0 {
		t.Fatal("expected WarningNews when IngestPath cannot Stat rule path")
	}
	msg := eng.NewsList[0]
	if !strings.Contains(msg, "inaccessible") {
		t.Fatalf("unexpected warning: %s", msg)
	}
	if !strings.Contains(msg, secret) {
		t.Fatalf("warning should mention path %s: %s", secret, msg)
	}
	if !strings.Contains(msg, "test-app") {
		t.Fatalf("warning should mention app: %s", msg)
	}
}

// TestIngestPathMissingPathIsSilent checks that a missing concrete path
// (game not installed) does not produce a warning.
func TestIngestPathMissingPathIsSilent(t *testing.T) {
	outDir := t.TempDir()
	base := t.TempDir()
	missing := filepath.Join(base, "no-such-save")
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	eng.IngestPath(t.Context(), "test-app", "saves", missing, true, base)

	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "inaccessible") {
			t.Fatalf("missing path must not warn as inaccessible: %s", msg)
		}
	}
}

// TestSearchForHomesSurfacesReadDirErrors checks that a directory whose
// contents cannot be listed produces WarningNews instead of looking like
// "no homes under this path".
func TestSearchForHomesSurfacesReadDirErrors(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	// If we can still read the dir (e.g. running as root), skip.
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("directory is readable; cannot exercise ReadDir failure")
	}

	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	homes := eng.SearchForHomes(blocked, 3)

	if len(homes) != 0 {
		t.Fatalf("expected no homes from unreadable dir, got %v", homes)
	}
	if len(eng.NewsList) == 0 {
		t.Fatal("expected WarningNews when SearchForHomes cannot ReadDir")
	}
	if !strings.Contains(eng.NewsList[0], "searching for homes") {
		t.Fatalf("unexpected warning: %s", eng.NewsList[0])
	}
	if !strings.Contains(eng.NewsList[0], blocked) {
		t.Fatalf("warning should mention path %s: %s", blocked, eng.NewsList[0])
	}
}

// TestSearchForHomesSurfacesLstatErrors checks that a search start path whose
// Lstat fails for a reason other than NotExist produces WarningNews instead
// of looking like "no homes here".
func TestSearchForHomesSurfacesLstatErrors(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	// Child under an unreadable parent: Lstat fails with permission (not NotExist).
	secret := filepath.Join(blocked, "nested-home")
	if _, err := os.Lstat(secret); err == nil {
		t.Skip("path is lstat-able; cannot exercise inaccessible Lstat")
	} else if errors.Is(err, fs.ErrNotExist) {
		t.Skip("platform reports NotExist for unreadable parent; cannot exercise")
	}

	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	homes := eng.SearchForHomes(secret, 3)

	if len(homes) != 0 {
		t.Fatalf("expected no homes from inaccessible path, got %v", homes)
	}
	if len(eng.NewsList) == 0 {
		t.Fatal("expected WarningNews when SearchForHomes cannot Lstat start path")
	}
	if !strings.Contains(eng.NewsList[0], "Failed to access path") {
		t.Fatalf("unexpected warning: %s", eng.NewsList[0])
	}
	if !strings.Contains(eng.NewsList[0], secret) {
		t.Fatalf("warning should mention path %s: %s", secret, eng.NewsList[0])
	}
}

// TestSearchForHomesMissingPathIsSilent checks that a missing start path does
// not produce a warning (normal when a search root is absent).
func TestSearchForHomesMissingPathIsSilent(t *testing.T) {
	outDir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no-such-home-root")
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	homes := eng.SearchForHomes(missing, 3)

	if len(homes) != 0 {
		t.Fatalf("expected no homes from missing path, got %v", homes)
	}
	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "Failed to access path") {
			t.Fatalf("missing path must not warn as inaccessible: %s", msg)
		}
	}
}

// TestSearchForHomesSurfacesMarkerStatErrors checks that when .config/AppData
// cannot be Stat'd for a reason other than NotExist, SearchForHomes surfaces
// WarningNews instead of treating the dir as "no home markers".
func TestSearchForHomesSurfacesMarkerStatErrors(t *testing.T) {
	// Unreadable directory: Lstat of the dir itself succeeds, but Stat of
	// children (.config / AppData) fails with permission denied.
	blocked := filepath.Join(t.TempDir(), "blocked-home")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	marker := filepath.Join(blocked, ".config")
	if _, err := os.Stat(marker); err == nil {
		t.Skip("marker is stat-able; cannot exercise inaccessible marker Stat")
	} else if errors.Is(err, fs.ErrNotExist) {
		// Some platforms report NotExist when the parent is unreadable.
		t.Skip("platform reports NotExist for unreadable parent; cannot exercise")
	}

	outDir := t.TempDir()
	eng := backup.NewEngine(config.New(), nil, nil, outDir)
	homes := eng.SearchForHomes(blocked, 1)

	if len(homes) != 0 {
		t.Fatalf("expected no homes from unreadable dir, got %v", homes)
	}
	foundMarkerWarn := false
	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "home marker") && strings.Contains(msg, blocked) {
			foundMarkerWarn = true
			break
		}
	}
	if !foundMarkerWarn {
		t.Fatalf("expected WarningNews about home marker under %s; got %v", blocked, eng.NewsList)
	}
}

// TestSearchForHomesMissingMarkersAreSilent checks that a readable dir without
// .config or AppData does not warn about missing home markers.
func TestSearchForHomesMissingMarkersAreSilent(t *testing.T) {
	root := t.TempDir()
	// Intermediate directory with neither marker.
	mid := filepath.Join(root, "not-a-home")
	if err := os.Mkdir(mid, 0o755); err != nil {
		t.Fatal(err)
	}
	eng := backup.NewEngine(config.New(), nil, nil, t.TempDir())
	homes := eng.SearchForHomes(mid, 1)
	if len(homes) != 0 {
		t.Fatalf("expected no homes without markers, got %v", homes)
	}
	for _, msg := range eng.NewsList {
		if strings.Contains(msg, "home marker") {
			t.Fatalf("missing markers must not warn: %s", msg)
		}
	}
}
