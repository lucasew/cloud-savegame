// Package rules provides mechanisms to load and parse backup rule definitions.
// Rules define which files and directories should be backed up for specific applications.
// They are stored as text files where each line specifies a rule name and a path pattern.
package rules

import (
	"bufio"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/lucasew/cloud-savegame/internal/config"
)

// Rule represents a single backup instruction for an application.
type Rule struct {
	// Name is the identifier for the specific rule (e.g., "saves", "config").
	// This name is used to organize backups in the output directory.
	Name string
	// Path is the file system path or glob pattern to be backed up.
	// It may contain special variables (e.g., $home, $appdata) which are resolved at runtime.
	Path string
}

// RuleFile represents a source file containing rule definitions.
type RuleFile struct {
	// FS is the file system where the rule file resides.
	FS fs.FS
	// Path is the path to the rule file within the FS.
	Path string
}

// Loader manages the discovery and parsing of rule files from multiple sources.
type Loader struct {
	// Cfg is the application configuration, used to check for rule exclusions.
	Cfg *config.Config
	// Sources is a list of file systems to search for rule files.
	// This allows combining embedded default rules with user-provided custom rules.
	Sources []fs.FS
}

// NewLoader creates a new Loader instance with the given configuration and rule sources.
func NewLoader(cfg *config.Config, sources []fs.FS) *Loader {
	return &Loader{
		Cfg:     cfg,
		Sources: sources,
	}
}

// GetApps scans all configured sources to discover available application rule files.
// It returns a map where the key is the application name (derived from the filename)
// and the value is the RuleFile location.
//
// It walks the file system looking for ".txt" files.
// If multiple sources contain rules for the same app, the last one found takes precedence (based on map assignment).
func (l *Loader) GetApps() (map[string]RuleFile, error) {
	apps := make(map[string]RuleFile)

	for _, fsys := range l.Sources {
		// Walk the file system to find all .txt files which represent application rules.
		// The FS structure is expected to have rule files at the root or handle nested paths if fs.Sub was used.
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
			// Continue to next source to allow partial loading if one source fails.
		}
	}
	return apps, nil
}

// ParseRules reads and parses the rules for a specific application from its rule file.
//
// The file format expects each line to contain two space-separated fields:
//  1. Rule Name: Identifier for the backup item.
//  2. Rule Path: The path pattern to back up.
//
// Lines matching "ignore_<RuleName>" in the configuration (e.g. [app] ignore_saves=true) are skipped.
// Empty lines are ignored.
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

			// Check configuration to see if this specific rule should be ignored.
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
