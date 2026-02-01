package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/cloud-savegame/internal/config"
)

func TestConfigLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove tmp dir: %v", err)
		}
	}()

	content := `
[general]
divider=,

[test]
foo=bar
list=a,b, c
path=~/test/file.txt
bool=true
`
	path := filepath.Join(tmpDir, "test.cfg")
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize Config
	cfg := config.New()
	err = cfg.Load(path)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test GetStr
	if val := cfg.GetStr("test", "foo"); val != "bar" {
		t.Errorf("GetStr: Expected 'bar', got '%s'", val)
	}
	if val := cfg.GetStr("test", "missing"); val != "" {
		t.Errorf("GetStr: Expected empty string for missing key, got '%s'", val)
	}

	// Test GetList
	list := cfg.GetList("test", "list")
	if len(list) != 3 {
		t.Errorf("GetList: Expected 3 items, got %d", len(list))
	}
	expectedList := []string{"a", "b", "c"}
	for i, v := range list {
		if v != expectedList[i] {
			t.Errorf("GetList: Index %d expected '%s', got '%s'", i, expectedList[i], v)
		}
	}

	// Test GetBool
	if !cfg.GetBool("test", "bool") {
		t.Error("GetBool: Expected true for existing key")
	}
	if cfg.GetBool("test", "nonexistent") {
		t.Error("GetBool: Expected false for missing key")
	}

	// Test GetPaths
	paths := cfg.GetPaths("test", "path")
	if len(paths) != 1 {
		t.Fatalf("GetPaths: Expected 1 path, got %d", len(paths))
	}

	// Check expansion: ~/test/file.txt should not start with ~
	if strings.HasPrefix(paths[0], "~") {
		t.Errorf("GetPaths: Path was not expanded: %s", paths[0])
	}

	// It should be absolute
	if !filepath.IsAbs(paths[0]) {
		t.Errorf("GetPaths: Path is not absolute: %s", paths[0])
	}
}
