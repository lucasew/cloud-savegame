package git

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Wrapper handles git operations.
type Wrapper struct {
	gitBin string
	dir    string
}

// New creates a new git wrapper. If dir is empty, uses current working directory.
func New(dir string) *Wrapper {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil
	}
	return &Wrapper{
		gitBin: bin,
		dir:    dir,
	}
}

// Available returns true if git is available.
func (g *Wrapper) Available() bool {
	return g != nil && g.gitBin != ""
}

// Exec executes a git command.
func (g *Wrapper) Exec(ctx context.Context, args ...string) error {
	if !g.Available() {
		return nil
	}
	slog.Info("git", "args", args)
	cmd := exec.CommandContext(ctx, g.gitBin, args...)
	cmd.Dir = g.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsRepoDirty returns true if there are uncommitted changes.
func (g *Wrapper) IsRepoDirty(ctx context.Context) (bool, error) {
	if !g.Available() {
		return false, nil
	}
	// git status -s
	cmd := exec.CommandContext(ctx, g.gitBin, "status", "-s")
	cmd.Dir = g.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return out.Len() > 0, nil
}

// Init initializes a git repo if not exists.
func (g *Wrapper) Init(ctx context.Context, initialBranch string) error {
	if !g.Available() {
		return nil
	}
	gitDir := ".git"
	if g.dir != "" {
		gitDir = g.dir + "/.git"
	}
	if _, err := os.Stat(gitDir); err == nil {
		return nil
	}
	return g.Exec(ctx, "init", "--initial-branch", initialBranch)
}

func (g *Wrapper) Commit(ctx context.Context, message string) error {
	if !g.Available() {
		return nil
	}
	// Secure commit using --file=- to read message from stdin
	cmd := exec.CommandContext(ctx, g.gitBin, "commit", "--file=-")
	cmd.Dir = g.dir
	cmd.Stdin = strings.NewReader(message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	slog.Info("git", "args", []string{"commit", "-m", "..."}) // Log without message to avoid clutter/secrets? or log message.
	return cmd.Run()
}
