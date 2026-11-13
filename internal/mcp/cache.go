package mcp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DefaultTTL is how long the cached tool list is trusted.
const DefaultTTL = 24 * time.Hour

// ToolCache is the on-disk tool list.
type ToolCache struct {
	Server    string    `json:"server"`
	FetchedAt time.Time `json:"fetched_at"`
	Tools     []Tool    `json:"tools"`
}

// Find returns the tool with the given name.
func (c *ToolCache) Find(name string) (Tool, bool) {
	if c == nil {
		return Tool{}, false
	}
	for _, t := range c.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Names returns sorted tool names.
func (c *ToolCache) Names() []string {
	var n []string
	for _, t := range c.Tools {
		n = append(n, t.Name)
	}
	sort.Strings(n)
	return n
}

// Fresh reports whether the cache is for this server and younger than ttl.
func (c *ToolCache) Fresh(server string, ttl time.Duration, now time.Time) bool {
	return c != nil && c.Server == server && now.Sub(c.FetchedAt) < ttl
}

// LoadCache reads <dir>/tools.json; a missing file returns nil, nil.
func LoadCache(dir string) (*ToolCache, error) {
	b, err := os.ReadFile(filepath.Join(dir, "tools.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c ToolCache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, nil // corrupt cache: treat as absent
	}
	return &c, nil
}

// SaveCache writes <dir>/tools.json.
func SaveCache(dir string, c *ToolCache) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tools.json"), b, 0o644)
}
