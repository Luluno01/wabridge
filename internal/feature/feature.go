package feature

import (
	"fmt"
	"strings"
)

// Config controls which action tool categories are enabled.
type Config struct {
	Send        bool `json:"send"`
	Download    bool `json:"download"`
	HistorySync bool `json:"history_sync"`
}

// presets maps access level (0-3) to a Config.
var presets = map[int]Config{
	0: {Send: false, Download: false, HistorySync: false},
	1: {Send: false, Download: true, HistorySync: false},
	2: {Send: false, Download: true, HistorySync: true},
	3: {Send: true, Download: true, HistorySync: true},
}

// NewConfig builds a Config from an access level and an override string.
// The override string is a comma-separated list of "+feature" or "-feature"
// toggles applied on top of the level preset.
// Valid feature names: send, download, history-sync.
func NewConfig(level int, overrides string) (Config, error) {
	cfg, ok := presets[level]
	if !ok {
		return Config{}, fmt.Errorf("invalid access level %d (must be 0-3)", level)
	}

	if overrides == "" {
		return cfg, nil
	}

	return applyOverrides(cfg, overrides)
}

// applyOverrides parses a comma-separated override string and applies
// each toggle to the config. Each entry must be "+feature" or "-feature".
func applyOverrides(cfg Config, overrides string) (Config, error) {
	for _, entry := range strings.Split(overrides, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if len(entry) < 2 || (entry[0] != '+' && entry[0] != '-') {
			return Config{}, fmt.Errorf("override %q must start with + or -", entry)
		}

		enable := entry[0] == '+'
		name := entry[1:]

		switch name {
		case "send":
			cfg.Send = enable
		case "download":
			cfg.Download = enable
		case "history-sync":
			cfg.HistorySync = enable
		default:
			return Config{}, fmt.Errorf("unknown feature %q (valid: send, download, history-sync)", name)
		}
	}
	return cfg, nil
}

// Intersect returns a Config where each feature is enabled only if it is
// enabled in both a and b. Used to combine bridge and local configs.
func Intersect(a, b Config) Config {
	return Config{
		Send:        a.Send && b.Send,
		Download:    a.Download && b.Download,
		HistorySync: a.HistorySync && b.HistorySync,
	}
}
