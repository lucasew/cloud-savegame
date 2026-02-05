package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/config"
	"github.com/lucasew/cloud-savegame/internal/rules"
	"io/fs"
)

func TestParseRules(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "rules_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove tmp dir: %v", err)
		}
	}()

	ruleContent := "save /path/to/save\nignore_me /should/not/be/seen"
	err = os.WriteFile(filepath.Join(tmpDir, "game.txt"), []byte(ruleContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Mock Config
	configFile := filepath.Join(tmpDir, "config.ini")
	configContent := "[game]\nignore_ignore_me=1" // Ignore the rule named "ignore_me"
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	if err := cfg.Load(configFile); err != nil {
		t.Fatal(err)
	}

	loader := rules.NewLoader(cfg, []fs.FS{os.DirFS(tmpDir)})

	// Test GetApps
	apps, err := loader.GetApps()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := apps["game"]; !ok {
		t.Error("Expected app 'game' to be found")
	}

	// Test ParseRules
	r, err := loader.ParseRules("game", apps["game"])
	if err != nil {
		t.Fatal(err)
	}

	if len(r) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(r))
	}
	if r[0].Name != "save" {
		t.Errorf("Expected rule name 'save', got '%s'", r[0].Name)
	}
}
