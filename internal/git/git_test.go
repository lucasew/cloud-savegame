package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/git"
)

func requireGit(t *testing.T, dir string) *git.Wrapper {
	t.Helper()
	g := git.New(dir)
	if g == nil || !g.Available() {
		t.Skip("git binary not available")
	}
	return g
}

func configureIdentity(t *testing.T, g *git.Wrapper) {
	t.Helper()
	ctx := t.Context()
	if err := g.Exec(ctx, "config", "user.email", "janitor@example.com"); err != nil {
		t.Fatalf("config user.email: %v", err)
	}
	if err := g.Exec(ctx, "config", "user.name", "Janitor"); err != nil {
		t.Fatalf("config user.name: %v", err)
	}
}

func TestNilWrapperUnavailable(t *testing.T) {
	var g *git.Wrapper
	if g.Available() {
		t.Fatal("nil wrapper should not be available")
	}
	if err := g.Init(t.Context(), "main"); err != nil {
		t.Fatalf("Init on nil wrapper: %v", err)
	}
	dirty, err := g.IsRepoDirty(t.Context())
	if err != nil || dirty {
		t.Fatalf("IsRepoDirty on nil wrapper: dirty=%v err=%v", dirty, err)
	}
	if err := g.Commit(t.Context(), "msg"); err != nil {
		t.Fatalf("Commit on nil wrapper: %v", err)
	}
}

func TestInitIdempotentAndDirtyDetection(t *testing.T) {
	dir := t.TempDir()
	g := requireGit(t, dir)
	ctx := t.Context()

	if err := g.Init(ctx, "main"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git missing after Init: %v", err)
	}
	// Second Init must be a no-op (existing .git).
	if err := g.Init(ctx, "main"); err != nil {
		t.Fatalf("second Init: %v", err)
	}

	configureIdentity(t, g)

	dirty, err := g.IsRepoDirty(ctx)
	if err != nil {
		t.Fatalf("IsRepoDirty empty repo: %v", err)
	}
	if dirty {
		t.Fatal("fresh repo should not be dirty")
	}

	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err = g.IsRepoDirty(ctx)
	if err != nil {
		t.Fatalf("IsRepoDirty after write: %v", err)
	}
	if !dirty {
		t.Fatal("repo with untracked file should be dirty")
	}

	if err := g.Exec(ctx, "add", "-A"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := g.Commit(ctx, "add note"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	dirty, err = g.IsRepoDirty(ctx)
	if err != nil {
		t.Fatalf("IsRepoDirty after commit: %v", err)
	}
	if dirty {
		t.Fatal("clean tree after commit should not be dirty")
	}
}

func TestIsRepoDirtyNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := requireGit(t, dir)
	_, err := g.IsRepoDirty(t.Context())
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}

func TestInitPropagatesStatPermissionError(t *testing.T) {
	// Init must not treat permission failures as "no .git, so init".
	parent := t.TempDir()
	noAccess := filepath.Join(parent, "noaccess")
	if err := os.Mkdir(noAccess, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(noAccess, 0o755) })

	g := requireGit(t, noAccess)
	err := g.Init(t.Context(), "main")
	if err == nil {
		t.Fatal("expected permission error from Stat of .git under unreadable dir")
	}
}
