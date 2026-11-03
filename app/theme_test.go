package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeColors(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "colors.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestThemeFollowsOmarchy(t *testing.T) {
	th := themeFrom(writeColors(t, `
mode = "dark"
accent = "#81a1c1"
background = "#2e3440"
dark_background = "#222730"
foreground = "#d8dee9"
red = "#bf616a"
`))
	want := map[string]string{
		"color-scheme": "dark",
		"--bg":         "#2e3440",
		"--bg-sidebar": "#222730",
		"--fg":         "#d8dee9",
		"--accent":     "#81a1c1",
		"--danger":     "#bf616a",
		"--sel":        "rgba(129, 161, 193, 0.18)",
		"--tag":        "#81a1c1", // no magenta in this palette: falls back to the accent
	}
	for k, v := range want {
		if th[k] != v {
			t.Errorf("%s = %q, want %q", k, th[k], v)
		}
	}
}

func TestThemeLightMode(t *testing.T) {
	th := themeFrom(writeColors(t, "mode = \"light\"\naccent = \"#005\"\nbackground = \"#fff\"\nforeground = \"#000\"\n"))
	if th["color-scheme"] != "light" {
		t.Errorf("color-scheme = %q", th["color-scheme"])
	}
}

// No theme, or one we cannot read, must leave the stylesheet's own palette in
// charge rather than paint half a window.
func TestThemeAbsentOrUnusable(t *testing.T) {
	if themeFrom(filepath.Join(t.TempDir(), "nope.toml")) != nil {
		t.Error("a missing file should give no theme")
	}
	if themeFrom(writeColors(t, "accent = \"#fff\"\n")) != nil {
		t.Error("a palette without a background should give no theme")
	}
	if themeFrom(writeColors(t, "this is not toml")) != nil {
		t.Error("garbage should give no theme")
	}
}
