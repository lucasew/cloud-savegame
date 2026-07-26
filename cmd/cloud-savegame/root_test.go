package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAppendUnique(t *testing.T) {
	t.Parallel()

	var list []string
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "euro-truck-simulator-2")
	// Same app appears on many $documents rules; must not multiply work.
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "american-truck-simulator")

	want := []string{
		"farming-simulator-19",
		"euro-truck-simulator-2",
		"american-truck-simulator",
	}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("appendUnique = %#v, want %#v", list, want)
	}
}

func TestPathStatProblemMissing(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	_, err := os.Stat(missing)
	if err == nil {
		t.Fatal("expected Stat error for missing path")
	}
	msg := pathStatProblem("Game install dir for flatout-2", missing, err)
	if !strings.Contains(msg, "does not exist") {
		t.Fatalf("missing path message = %q, want does not exist", msg)
	}
	if !strings.Contains(msg, missing) {
		t.Fatalf("message should include path: %q", msg)
	}
	if !strings.Contains(msg, "flatout-2") {
		t.Fatalf("message should include label: %q", msg)
	}
}

func TestPathStatProblemInaccessible(t *testing.T) {
	t.Parallel()
	// Synthetic non-IsNotExist error (permission-style) must not use the
	// "does not exist" wording and must keep the path skipped by callers.
	err := errors.New("permission denied")
	secretHome := filepath.Join(t.TempDir(), "secret-home")
	msg := pathStatProblem("extra home", secretHome, err)
	if strings.Contains(msg, "does not exist") {
		t.Fatalf("inaccessible path must not look missing: %q", msg)
	}
	if !strings.Contains(msg, "inaccessible") {
		t.Fatalf("message = %q, want inaccessible", msg)
	}
	if !strings.Contains(msg, secretHome) {
		t.Fatalf("message should include path: %q", msg)
	}
	if !strings.Contains(msg, "permission denied") {
		t.Fatalf("message should include underlying error: %q", msg)
	}
}

func TestListSubdirNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"user-a", "user-b"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Files must not appear in the result.
	if err := os.WriteFile(filepath.Join(root, "users.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := listSubdirNames(root)
	if err != nil {
		t.Fatalf("listSubdirNames: %v", err)
	}
	want := map[string]bool{"user-a": true, "user-b": true}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want two user dirs", names)
	}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("unexpected name %q in %v", n, names)
		}
	}
}

func TestListSubdirNamesError(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	_, err := listSubdirNames(missing)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	// Call site must surface this via pathStatProblem, not treat as empty list.
	msg := pathStatProblem("Ubisoft savegames dir", missing, err)
	if !strings.Contains(msg, "does not exist") && !strings.Contains(msg, "inaccessible") {
		t.Fatalf("pathStatProblem for ReadDir err = %q", msg)
	}
}

func TestPathStatProblemProgramFilesParent(t *testing.T) {
	t.Parallel()
	// Program Files discovery uses the same helper when grandparent ReadDir fails.
	missing := filepath.Join(t.TempDir(), "no-such-drive-root")
	_, err := os.ReadDir(missing)
	if err == nil {
		t.Fatal("expected ReadDir error for missing path")
	}
	msg := pathStatProblem("Program Files parent dir", missing, err)
	if !strings.Contains(msg, "does not exist") && !strings.Contains(msg, "inaccessible") {
		t.Fatalf("pathStatProblem for Program Files parent = %q", msg)
	}
	if !strings.Contains(msg, missing) {
		t.Fatalf("message should include path: %q", msg)
	}
	if !strings.Contains(msg, "Program Files parent dir") {
		t.Fatalf("message should include label: %q", msg)
	}
}

// TestPathStatProblemHomeDiscoveryProbes checks labels used when Stat fails
// for documents / Common Files / Ubisoft probes (inaccessible, not missing).
func TestPathStatProblemHomeDiscoveryProbes(t *testing.T) {
	t.Parallel()
	err := errors.New("permission denied")
	cases := []struct {
		label string
		path  string
	}{
		{"documents dir", filepath.Join(t.TempDir(), "home", "Documents")},
		{"Program Files Common Files probe", filepath.Join(t.TempDir(), "PF", "Common Files")},
		{"Ubisoft savegames dir", filepath.Join(t.TempDir(), "savegames")},
	}
	for _, tc := range cases {
		msg := pathStatProblem(tc.label, tc.path, err)
		if strings.Contains(msg, "does not exist") {
			t.Fatalf("%s: inaccessible must not look missing: %q", tc.label, msg)
		}
		if !strings.Contains(msg, "inaccessible") {
			t.Fatalf("%s: message = %q, want inaccessible", tc.label, msg)
		}
		if !strings.Contains(msg, tc.path) {
			t.Fatalf("%s: message should include path: %q", tc.label, msg)
		}
		if !strings.Contains(msg, tc.label) {
			t.Fatalf("%s: message should include label: %q", tc.label, msg)
		}
	}
}

// TestDocumentsProbeStatUnderUnreadableHome documents that Stat of a child under
// a mode-000 home fails with a non-IsNotExist error (the case WarningNews covers).
func TestDocumentsProbeStatUnderUnreadableHome(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(home, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })

	docs := filepath.Join(home, "Documents")
	_, err := os.Stat(docs)
	if err == nil {
		t.Skip("platform allows Stat under mode-000 home; cannot exercise")
	}
	if os.IsNotExist(err) {
		t.Skip("platform reports NotExist under mode-000 home; cannot exercise")
	}
	msg := pathStatProblem("documents dir", docs, err)
	if strings.Contains(msg, "does not exist") {
		t.Fatalf("must not look missing: %q", msg)
	}
	if !strings.Contains(msg, "inaccessible") {
		t.Fatalf("message = %q, want inaccessible", msg)
	}
	if !strings.Contains(msg, docs) {
		t.Fatalf("message should include path: %q", msg)
	}
}
