package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type Config struct {
	file *ini.File
}

func New() *Config {
	return &Config{}
}

func (c *Config) Load(path string) error {
	f, err := ini.Load(path)
	if err != nil {
		return err
	}
	c.file = f
	return nil
}

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

// GetBool returns true if the key exists, regardless of value.
func (c *Config) GetBool(section, key string) bool {
	if c.file == nil {
		return false
	}
	return c.file.Section(section).HasKey(key)
}

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

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, _ := os.UserHomeDir()
		return filepath.Join(dirname, path[2:])
	}
	return path
}
