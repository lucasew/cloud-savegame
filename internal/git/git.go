package git

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
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

// Init initializes a new Git repository if one does not already exist.
// It checks for the existence of the '.git' directory before running 'git init'.
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

// Commit creates a new commit with the specified message.
// It safely passes the commit message via stdin using '--file=-' to avoid command line length limits or character escaping issues.
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
