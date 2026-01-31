package rules

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/cloud-savegame/internal/config"
)

type Rule struct {
	Name string
	Path string
}

type Loader struct {
	Cfg      *config.Config
	RuleDirs []string
}

func NewLoader(cfg *config.Config, ruleDirs []string) *Loader {
	return &Loader{
		Cfg:      cfg,
		RuleDirs: ruleDirs,
	}
}

// GetApps returns a list of apps found in the rule directories.
// It also returns a map of app -> rule file path.
func (l *Loader) GetApps() (map[string]string, error) {
	apps := make(map[string]string)
	for _, dir := range l.RuleDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
				appName := strings.TrimSuffix(entry.Name(), ".txt")
				apps[appName] = filepath.Join(dir, entry.Name())
			}
		}
	}
	return apps, nil
}

// ParseRules yields rules for a specific app.
func (l *Loader) ParseRules(appName, ruleFile string) ([]Rule, error) {
	f, err := os.Open(ruleFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close rule file", "file", ruleFile, "error", err)
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

			// Check config for ignore
			// Python: get_bool(config, app, f"ignore_{rule_name}")
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
