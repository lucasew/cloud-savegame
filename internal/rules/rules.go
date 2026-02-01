package rules

import (
	"bufio"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/lucasew/cloud-savegame/internal/config"
)

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

// GetApps returns a list of apps found in the rule FSs.
func (l *Loader) GetApps() (map[string]RuleFile, error) {
	apps := make(map[string]RuleFile)

	for _, fsys := range l.Sources {
		// Start walk from root of each FS
		// If embedded "rules", the files are in "rules/xxx.txt"?
		// If using `//go:embed rules`, the FS has "rules" directory at root.
		// If using `os.DirFS(".../output/__rules__")`, files are at root.
		// We need to handle both cases or normalize.
		// If we use `fs.Sub(fsys, "rules")` for embedded?

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
			// Continue to next source?
		}
	}
	return apps, nil
}

// ParseRules yields rules for a specific app.
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
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			ruleName := strings.TrimSpace(parts[0])
			rulePath := strings.TrimSpace(parts[1])

			if l.Cfg.GetBool(appName, "ignore_"+ruleName) {
				continue
			}

			rules = append(rules, Rule{
				Name: ruleName,
				Path: rulePath,
			})
		}
	}
	return rules, scanner.Err()
}
