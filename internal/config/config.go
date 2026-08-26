package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// Config wraps the INI file parsing logic.
type Config struct {
	file *ini.File
}

// New creates a new empty Config instance.
func New() *Config {
	return &Config{}
}

// Load reads and parses an INI file from the specified path.
func (c *Config) Load(path string) error {
	f, err := ini.Load(path)
	if err != nil {
		return err
	}
	c.file = f
	return nil
}

// GetStr retrieves a string value from the specified section and key.
// Returns an empty string if the key or section does not exist.
func (c *Config) GetStr(section, key string) string {
	if c.file == nil {
		return ""
	}
	sec := c.file.Section(section)
	if !sec.HasKey(key) {
		return ""
	}
	return sec.Key(key).String()
}

// GetBool returns true if the key exists in the section.
// Note: This does NOT check the value (e.g. "true" or "false"), only existence.
// This is useful for flags or toggles where presence implies enabled.
func (c *Config) GetBool(section, key string) bool {
	if c.file == nil {
		return false
	}
	return c.file.Section(section).HasKey(key)
}

// GetList retrieves a list of strings from a key.
// The value is split by a delimiter defined in [general] divider (defaults to comma).
// Whitespace around items is trimmed.
func (c *Config) GetList(section, key string) []string {
	divider := c.GetStr("general", "divider")
	if divider == "" {
		divider = ","
	}
	raw := c.GetStr(section, key)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, divider)
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetPaths retrieves a list of paths from a key, performing expansion and absolute path resolution.
// It expands home directory usage (~) and converts relative paths to absolute ones.
func (c *Config) GetPaths(section, key string) []string {
	list := c.GetList(section, key)
	var result []string
	for _, p := range list {
		expanded := expandPath(p)
		abs, err := filepath.Abs(expanded)
		if err == nil {
			result = append(result, abs)
		}
	}
	return result
}

// expandPath expands the leading tilde (~) to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, _ := os.UserHomeDir()
		return filepath.Join(dirname, path[2:])
	}
	return path
}
