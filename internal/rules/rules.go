package rules

import (
	"bufio"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"strings"

	"github.com/lucasew/cloud-savegame/internal/config"
)

// supportedPathVars are placeholders the CLI expands while scanning.
// Keep in sync with the variable loops in cmd/cloud-savegame.
var supportedPathVars = map[string]struct{}{
	"home":          {},
	"appdata":       {},
	"documents":     {},
	"installdir":    {},
	"program_files": {},
	"ubisoft":       {},
}

var pathVarPattern = regexp.MustCompile(`\$([a-z_]+)`)

type Rule struct {
	Name string
	Path string
}

type RuleFile struct {
	FS   fs.FS
	Path string
}

type Loader struct {
	Cfg     *config.Config
	Sources []fs.FS
}

func NewLoader(cfg *config.Config, sources []fs.FS) *Loader {
	return &Loader{
		Cfg:     cfg,
		Sources: sources,
	}
}

// GetApps returns apps discovered as *.txt rule files across all sources.
// Later sources override earlier ones for the same app name (custom rules win).
// If any source fails to walk, the error is returned along with apps found so far.
func (l *Loader) GetApps() (map[string]RuleFile, error) {
	apps := make(map[string]RuleFile)
	var walkErr error

	for _, fsys := range l.Sources {
		err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".txt") {
				appName := strings.TrimSuffix(d.Name(), ".txt")
				apps[appName] = RuleFile{FS: fsys, Path: path}
			}
			return nil
		})
		if err != nil {
			slog.Error("failed to walk rules fs", "error", err)
			walkErr = err
		}
	}
	return apps, walkErr
}

// ParseRules yields rules for a specific app.
// Non-empty lines that are not "name path" are skipped with a warning.
// Paths that use an unsupported $variable are still returned, but a warning is logged
// because the CLI will never expand them.
func (l *Loader) ParseRules(appName string, rf RuleFile) ([]Rule, error) {
	f, err := rf.FS.Open(rf.Path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close rule file", "file", rf.Path, "error", err)
		}
	}()

	var rules []Rule
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			slog.Warn("skipping malformed rule line", "app", appName, "file", rf.Path, "line", lineNo, "text", line)
			continue
		}
		ruleName := strings.TrimSpace(parts[0])
		rulePath := strings.TrimSpace(parts[1])
		if ruleName == "" || rulePath == "" {
			slog.Warn("skipping rule with empty name or path", "app", appName, "file", rf.Path, "line", lineNo, "text", line)
			continue
		}

		if l.Cfg.GetBool(appName, "ignore_"+ruleName) {
			continue
		}

		for _, m := range pathVarPattern.FindAllStringSubmatch(rulePath, -1) {
			if len(m) < 2 {
				continue
			}
			v := m[1]
			if _, ok := supportedPathVars[v]; !ok {
				slog.Warn("unsupported path variable in rule; it will not be expanded",
					"app", appName, "var", v, "rule", ruleName, "path", rulePath)
			}
		}

		rules = append(rules, Rule{
			Name: ruleName,
			Path: rulePath,
		})
	}
	if err := scanner.Err(); err != nil {
		return rules, fmt.Errorf("scan rules %s: %w", rf.Path, err)
	}
	return rules, nil
}
