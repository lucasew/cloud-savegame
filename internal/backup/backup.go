package backup

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucasew/cloud-savegame/internal/config"
	"github.com/lucasew/cloud-savegame/internal/git"
	"github.com/lucasew/cloud-savegame/internal/rules"
)

// Engine manages the backup and restore process.
// It orchestrates reading configuration, git operations, and rule processing.
type Engine struct {
	Cfg          *config.Config
	Git          *git.Wrapper
	RulesLoader  *rules.Loader
	OutputDir    string
	IgnoredPaths []string
	Backlink     bool
	Verbose      bool
	MaxDepth     int
	Hostname     string
	NewsList     []string
}

// NewEngine creates a new Engine instance.
// It initializes the engine with the provided configuration, git wrapper, rules loader, and output directory.
// It also sets the default MaxDepth to 10 and captures the current hostname.
func NewEngine(cfg *config.Config, g *git.Wrapper, rl *rules.Loader, outputDir string) *Engine {
	hostname, _ := os.Hostname()
	return &Engine{
		Cfg:         cfg,
		Git:         g,
		RulesLoader: rl,
		OutputDir:   outputDir,
		Hostname:    hostname,
		MaxDepth:    10,
	}
}

// WarningNews appends a warning message to the news list and logs it as a warning.
// The news list is used to report issues to the user at the end of execution.
func (e *Engine) WarningNews(msg string) {
	e.NewsList = append(e.NewsList, msg)
	slog.Warn(msg)
}

// IsPathIgnored checks if the given path matches any of the configured ignored paths.
// It resolves the path to an absolute path before checking prefixes.
func (e *Engine) IsPathIgnored(path string) bool {
	path, _ = filepath.Abs(path)
	for _, ignored := range e.IgnoredPaths {
		if strings.HasPrefix(path, ignored) {
			return true
		}
	}
	return false
}

// BackupItem moves an item to a timestamped backup directory within the output directory.
// This is used to preserve existing files that would otherwise be overwritten or lost during operations.
func (e *Engine) BackupItem(item, outputDir string) {
	backupDir := filepath.Join(outputDir, "__backup__")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		slog.Error("failed to create backup dir", "path", backupDir, "error", err)
		return
	}
	name := filepath.Base(item)
	backupTarget := filepath.Join(backupDir, fmt.Sprintf("%s.%d", name, time.Now().Unix()))

	if err := os.Rename(item, backupTarget); err != nil {
		slog.Error("failed to move item to backup", "item", item, "target", backupTarget, "error", err)
		return
	}
	slog.Info("Moved item to backup", "item", item, "target", backupTarget)
	e.WarningNews(fmt.Sprintf("Moved potentially conflicting item '%s' to the backup directory at '%s'.", item, backupTarget))
}

// CopyItem recursively copies a file or directory from inputItem to destination.
// It includes several safety checks:
// 1. Loop detection: Prevents copying if the source is inside the output directory.
// 2. Symlink handling: Skips symlinks to avoid recursion loops or broken links.
// 3. Timestamp check: Skips copying if the destination is newer than the source (unless forced).
func (e *Engine) CopyItem(inputItem, destination, outputDir string, depth int) {
	// Verbose logging
	if e.Verbose {
		slog.Debug("Evaluating copy", "input", inputItem, "dest", destination)
	}

	info, err := os.Lstat(inputItem)
	if err != nil {
		return // doesn't exist or error
	}

	// Loop detection
	absInput, _ := filepath.Abs(inputItem)
	absOutput, _ := filepath.Abs(outputDir)
	if strings.HasPrefix(absInput, absOutput) {
		slog.Warn("copy_item: Not copying: Origin is inside output", "path", inputItem)
		return
	}

	// Symlink check
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Warn("copy_item: Not copying because it's a symlink", "path", inputItem)
		return
	}

	if info.IsDir() {
		if err := os.MkdirAll(destination, 0755); err != nil {
			slog.Error("failed to mkdir", "path", destination, "error", err)
			return
		}
		entries, err := os.ReadDir(inputItem)
		if err != nil {
			return
		}
		for _, entry := range entries {
			e.CopyItem(
				filepath.Join(inputItem, entry.Name()),
				filepath.Join(destination, entry.Name()),
				outputDir,
				depth+1,
			)
		}
	} else {
		// File
		// Check timestamp
		destInfo, err := os.Stat(destination)
		if err == nil {
			if info.ModTime().Before(destInfo.ModTime()) {
				if e.Verbose {
					slog.Debug("copy_item: Not copying: Didn't change", "path", inputItem)
				}
				return
			}
		}

		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			slog.Error("failed to mkdir parent", "path", filepath.Dir(destination), "error", err)
			return
		}

		slog.Info("copy_item: Copying", "src", inputItem, "dst", destination)
		if err := copyFile(inputItem, destination); err != nil {
			slog.Error("copy failed", "error", err)
		}
	}
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.Error("failed to close src", "error", err)
		}
	}()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := d.Close(); err != nil {
			slog.Error("failed to close dst", "error", err)
		}
	}()
	_, err = io.Copy(d, s)
	return err
}

// IngestPath processes a single path rule, handling globs, security checks, copying, and backlinking.
//
// Security:
// It strictly validates that `pathStr` is contained within `basePath` if provided, preventing path traversal attacks.
// Absolute paths are disallowed unless they are glob patterns.
//
// Glob Handling:
// If the path contains a wildcard (*), it expands the glob and recursively calls IngestPath for each match.
// It correctly handles matches in both the source and existing output directory to support syncing.
//
// Git Integration:
// If Git is configured, it commits changes after each successful ingestion.
//
// Backlinking:
// If enabled (`e.Backlink`), it replaces the original file with a symlink to the backup location.
// Before creating the symlink, it backs up the original file to avoid data loss.
func (e *Engine) IngestPath(app, ruleName, pathStr string, topLevel bool, basePath string) {
	// Security: base_path check
	if basePath != "" {
		resolvedPath, err := filepath.Abs(pathStr)
		resolvedBase, err2 := filepath.Abs(basePath)
		if err == nil && err2 == nil {
			if !strings.HasPrefix(resolvedPath, resolvedBase) {
				e.WarningNews(fmt.Sprintf("Security: Path '%s' for app '%s' resolves outside of its base '%s'. Skipping.", pathStr, app, basePath))
				return
			}
		}
	} else {
		// No base path: disallow absolute paths unless they are globs?
		// Python: elif "*" not in path and Path(path).is_absolute():
		if !strings.Contains(pathStr, "*") && filepath.IsAbs(pathStr) {
			e.WarningNews(fmt.Sprintf("Security: Absolute path '%s' for app '%s' is not allowed in rules. Skipping.", pathStr, app))
			return
		}
	}

	if e.IsPathIgnored(pathStr) {
		return
	}

	outputDir := filepath.Join(e.OutputDir, app, ruleName)

	if strings.Contains(pathStr, "*") {
		// Glob handling
		dir := filepath.Dir(pathStr)
		pattern := filepath.Base(pathStr)

		if strings.Contains(dir, "*") {
			slog.Error("globs in any path segment but the last are unsupported", "app", app, "rule", ruleName, "path", pathStr)
			return
		}

		// Find matches in source AND output (to handle deleted files? or just logic from python)
		// Python: names = set([x.name for x in [*parent.glob(filename), *output_dir.glob(filename)]])
		matches, _ := filepath.Glob(pathStr)
		outputMatches, _ := filepath.Glob(filepath.Join(outputDir, pattern))

		names := make(map[string]struct{})
		for _, m := range matches {
			names[filepath.Base(m)] = struct{}{}
		}
		for _, m := range outputMatches {
			names[filepath.Base(m)] = struct{}{}
		}

		for name := range names {
			item := filepath.Join(dir, name)
			newRuleName := filepath.Join(ruleName, name)
			base := basePath
			if base == "" {
				base = dir
			}
			e.IngestPath(app, newRuleName, item, true, base)
		}

	} else {
		// Concrete path
		if _, err := os.Stat(pathStr); err == nil {
			slog.Info("ingest", "path", pathStr, "output", outputDir)
			e.CopyItem(pathStr, outputDir, e.OutputDir, 0)

			// Git commit per file/ingest
			if e.Git != nil {
				isDirty, _ := e.Git.IsRepoDirty(context.TODO())
				if isDirty {
					commitMsg := fmt.Sprintf("hostname=%s app=%s rule=%s path=%s", e.Hostname, app, ruleName, pathStr)
					if err := e.Git.Exec(context.TODO(), "add", "-A"); err != nil {
						slog.Error("git add failed", "error", err)
					}
					if err := e.Git.Commit(context.TODO(), commitMsg); err != nil {
						slog.Error("git commit failed", "error", err)
					}
				}
			}
		}

		// Backlink logic
		if e.Backlink && topLevel {
			slog.Debug("TOPLEVEL backlink", "app", app, "rule", ruleName, "path", pathStr)
			parent := filepath.Dir(pathStr)
			if err := os.MkdirAll(parent, 0755); err != nil {
				slog.Error("failed to mkdir parent for backlink", "error", err)
			}

			info, err := os.Lstat(pathStr)
			isSymlink := err == nil && (info.Mode()&os.ModeSymlink != 0)
			exists := err == nil

			if isSymlink {
				if err := os.Remove(pathStr); err != nil {
					slog.Error("failed to remove symlink", "path", pathStr, "error", err)
				}
			} else if exists {
				e.BackupItem(pathStr, e.OutputDir)
			}

			slog.Info("ln", "src", pathStr, "target", outputDir)
			if err := os.Symlink(outputDir, pathStr); err != nil {
				slog.Error("failed to create symlink", "src", outputDir, "dst", pathStr, "error", err)
			}
		}

		// Check broken symlink
		info, err := os.Lstat(pathStr)
		if err == nil && (info.Mode()&os.ModeSymlink != 0) {
			if _, err := os.Stat(pathStr); err != nil {
				e.WarningNews(fmt.Sprintf("This may be a rule or a program bug: '%s' points to a non existent location.", pathStr))
			}
		}
	}
}

// SearchForHomes recursively searches for potential game root directories starting from startDir.
// It uses a heuristic to identify game roots by looking for common subdirectories like ".config" or "AppData".
// It avoids traversing into ignored folders (e.g., ".git", "nixpkgs") to improve performance.
func (e *Engine) SearchForHomes(startDir string, maxDepth int) []string {
	var homes []string
	if maxDepth <= 0 || e.IsPathIgnored(startDir) {
		return nil
	}

	info, err := os.Lstat(startDir)
	if err != nil || !info.IsDir() || (info.Mode()&os.ModeSymlink != 0) {
		return nil
	}

	baseName := filepath.Base(startDir)
	ignoreFolders := []string{"dosdevices", "nixpkgs", ".git", ".cache"}
	for _, i := range ignoreFolders {
		if baseName == i {
			return nil
		}
	}

	findFolders := []string{".config", "AppData"}
	found := false
	for _, f := range findFolders {
		if _, err := os.Stat(filepath.Join(startDir, f)); err == nil {
			homes = append(homes, startDir)
			found = true
			break
		}
	}

	if found {
		return homes
	}

	// Recurse
	entries, err := os.ReadDir(startDir)
	if err == nil {
		for _, entry := range entries {
			homes = append(homes, e.SearchForHomes(filepath.Join(startDir, entry.Name()), maxDepth-1)...)
		}
	}
	return homes
}
