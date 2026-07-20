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
	msg := pathStatProblem("extra home", "/secret/home", err)
	if strings.Contains(msg, "does not exist") {
		t.Fatalf("inaccessible path must not look missing: %q", msg)
	}
	if !strings.Contains(msg, "inaccessible") {
		t.Fatalf("message = %q, want inaccessible", msg)
	}
	if !strings.Contains(msg, "/secret/home") {
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
