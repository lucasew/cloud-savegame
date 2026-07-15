package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Wrapper provides an abstraction for Git operations within a specific directory.
// It ensures Git is available and handles command execution.
type Wrapper struct {
	gitBin string
	dir    string
}

// New creates a new Git Wrapper instance.
// It searches for the 'git' binary in the system PATH.
// If dir is empty, operations will run in the current working directory.
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

// Available checks if the Git binary was successfully found during initialization.
func (g *Wrapper) Available() bool {
	return g != nil && g.gitBin != ""
}

// Exec executes a Git command with the provided arguments.
// It streams stdout and stderr to the parent process's outputs.
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

// IsRepoDirty checks if there are any uncommitted changes in the repository.
// It uses 'git status -s' and returns true if the output is non-empty.
func (g *Wrapper) IsRepoDirty(ctx context.Context) (bool, error) {
	if !g.Available() {
		return false, nil
	}
	cmd := exec.CommandContext(ctx, g.gitBin, "status", "-s")
	cmd.Dir = g.dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg != "" {
			return false, fmt.Errorf("git status: %w: %s", err, msg)
		}
		return false, fmt.Errorf("git status: %w", err)
	}
	return out.Len() > 0, nil
}

// Init initializes a new Git repository if one does not already exist.
// It checks for the existence of the '.git' directory before running 'git init'.
// Permission or other Stat failures are returned instead of falling through to init.
func (g *Wrapper) Init(ctx context.Context, initialBranch string) error {
	if !g.Available() {
		return nil
	}
	gitDir := ".git"
	if g.dir != "" {
		gitDir = filepath.Join(g.dir, ".git")
	}
	_, err := os.Stat(gitDir)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return g.Exec(ctx, "init", "--initial-branch", initialBranch)
}

// Commit creates a new commit with the specified message.
// It passes the commit message via stdin using '--file=-' to avoid shell
// escaping issues and command-line length limits.
func (g *Wrapper) Commit(ctx context.Context, message string) error {
	if !g.Available() {
		return nil
	}
	cmd := exec.CommandContext(ctx, g.gitBin, "commit", "--file=-")
	cmd.Dir = g.dir
	cmd.Stdin = strings.NewReader(message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Log flags only; omit the message body to avoid leaking sensitive content.
	slog.Info("git", "args", []string{"commit", "--file=-"})
	return cmd.Run()
}
