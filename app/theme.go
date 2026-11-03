package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// omarchyColors is where Omarchy keeps the active theme's palette. Every theme
// ships one, whatever it does for the terminal or the bar, so it is the one
// file worth reading.
func omarchyColors() string {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "omarchy", "current", "theme", "colors.toml")
}

// Theme is the desktop's palette as CSS custom properties, or nil when there is
// no Omarchy theme to follow — macOS, or Linux without it — and the stylesheet's
// own light and dark palettes stand. The keys are the names style.css already
// uses, so the frontend only has to set them.
func (a *API) Theme() map[string]string { return themeFrom(omarchyColors()) }

func themeFrom(path string) map[string]string {
	if path == "" {
		return nil
	}
	var c map[string]any
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil
	}
	get := func(key string) string {
		s, _ := c[key].(string)
		return s
	}
	bg, fg, accent := get("background"), get("foreground"), get("accent")
	if bg == "" || fg == "" || accent == "" {
		return nil // not a palette we understand; better the default than a half theme
	}
	// A role missing from a theme falls back to a neighbour rather than to
	// nothing, so a sparse palette still produces a complete set.
	or := func(keys ...string) string {
		for _, k := range keys {
			if v := get(k); v != "" {
				return v
			}
		}
		return fg
	}
	scheme := "dark"
	if get("mode") == "light" {
		scheme = "light"
	}
	return map[string]string{
		"color-scheme": scheme,
		"--bg":         bg,
		"--bg-sidebar": or("dark_background", "background"),
		"--fg":         fg,
		"--fg-muted":   or("light_foreground", "dark_foreground"),
		"--line":       or("selection", "lighter_background", "muted"),
		"--accent":     accent,
		"--tag":        or("magenta", "accent"),
		"--prop":       or("cyan", "accent"),
		"--done":       or("green", "accent"),
		"--todo":       or("yellow", "orange", "accent"),
		"--sel":        translucent(accent, 0.18),
		"--danger":     or("red", "accent"),
	}
}

// translucent turns "#rrggbb" into an rgba() at the given alpha, which is how
// the stylesheet's selection tint is written. Anything else is passed through.
func translucent(hex string, alpha float64) string {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return hex
	}
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return hex
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", n>>16&0xff, n>>8&0xff, n&0xff, alpha)
}
