// Package config resolves paths and user settings for ibkr-cli.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AppName is the directory / keyring service name.
const AppName = "ibkr-cli"

// DefaultServerURL is IBKR's public remote MCP connector.
const DefaultServerURL = "https://api.ibkr.com/v1/api/mcp-public"

// Config is the optional user config file (config.json).
type Config struct {
	ServerURL string `json:"server_url,omitempty"`
}

// Dir returns the config directory (IBKR_CONFIG_DIR overrides).
func Dir() (string, error) {
	if d := os.Getenv("IBKR_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// CacheDir returns the cache directory (IBKR_CACHE_DIR overrides).
func CacheDir() (string, error) {
	if d := os.Getenv("IBKR_CACHE_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// Load reads config.json; a missing file yields the zero Config.
func Load() (Config, error) {
	var c Config
	dir, err := Dir()
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// ServerURL resolves the MCP server URL: flag > IBKR_MCP_URL > config file > default.
func ServerURL(flag string) string {
	if flag != "" {
		return strings.TrimRight(flag, "/")
	}
	if v := os.Getenv("IBKR_MCP_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if c, err := Load(); err == nil && c.ServerURL != "" {
		return strings.TrimRight(c.ServerURL, "/")
	}
	return DefaultServerURL
}
