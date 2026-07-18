package rules_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/lucasew/cloud-savegame/internal/config"
	"github.com/lucasew/cloud-savegame/internal/rules"
)

func TestParseRules(t *testing.T) {
	tmpDir := t.TempDir()

	ruleContent := "save /path/to/save\nignore_me /should/not/be/seen"
	err := os.WriteFile(filepath.Join(tmpDir, "game.txt"), []byte(ruleContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

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

	apps, err := loader.GetApps()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := apps["game"]; !ok {
		t.Error("Expected app 'game' to be found")
	}

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

func TestGetAppsWalkError(t *testing.T) {
	loader := rules.NewLoader(config.New(), []fs.FS{failFS{}})
	apps, err := loader.GetApps()
	if err == nil {
		t.Fatal("expected walk error from failing FS")
	}
	if len(apps) != 0 {
		t.Errorf("expected no apps, got %d", len(apps))
	}
}

func TestGetAppsLaterSourceOverrides(t *testing.T) {
	first := fstest.MapFS{
		"game.txt": &fstest.MapFile{Data: []byte("save $home/from-first\n")},
	}
	second := fstest.MapFS{
		"game.txt": &fstest.MapFile{Data: []byte("save $home/from-second\n")},
	}
	loader := rules.NewLoader(config.New(), []fs.FS{first, second})
	apps, err := loader.GetApps()
	if err != nil {
		t.Fatal(err)
	}
	r, err := loader.ParseRules("game", apps["game"])
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 1 || r[0].Path != "$home/from-second" {
		t.Fatalf("expected second source to win, got %#v", r)
	}
}

func TestParseRulesSkipsMalformedAndKeepsUnsupportedVars(t *testing.T) {
	fsys := fstest.MapFS{
		"game.txt": &fstest.MapFile{Data: []byte("" +
			"ok $home/save\n" +
			"not-a-valid-line\n" +
			"mods $steamapps/common/Game/Mods\n" +
			"  \n" +
			"emptyname \n"),
		},
	}
	loader := rules.NewLoader(config.New(), []fs.FS{fsys})
	apps, err := loader.GetApps()
	if err != nil {
		t.Fatal(err)
	}
	r, err := loader.ParseRules("game", apps["game"])
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 2 {
		t.Fatalf("expected 2 rules (ok + unsupported-var), got %d: %#v", len(r), r)
	}
	if r[0].Name != "ok" || r[0].Path != "$home/save" {
		t.Errorf("first rule: %#v", r[0])
	}
	if r[1].Name != "mods" || r[1].Path != "$steamapps/common/Game/Mods" {
		t.Errorf("second rule should retain unsupported var path: %#v", r[1])
	}
}

// failFS fails every Open, so fs.WalkDir cannot start.
type failFS struct{}

func (failFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrPermission
}
