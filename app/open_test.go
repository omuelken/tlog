package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenURLFollowsALinkFromItsNote(t *testing.T) {
	a := newAPI(t)
	assets := filepath.Join(a.svc.Store.Root, "assets")
	if err := os.MkdirAll(filepath.Join(assets, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{"q3.pdf": 0o644, "go.command": 0o755} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("x"), mode); err != nil {
			t.Fatal(err)
		}
	}
	var launched, revealed []string
	a.svc.Launch = func(p string) error { launched = append(launched, p); return nil }
	a.svc.Reveal = func(p string) error { revealed = append(revealed, p); return nil }

	// Relative to the journal the link is written in, not to the notes root.
	note := filepath.Join("journals", "2026-11-03.md")
	for _, target := range []string{"../assets/q3.pdf", "../assets/src", "../assets/go.command"} {
		if err := a.OpenURL(note, target); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
	}
	want := []string{filepath.Join(assets, "q3.pdf"), filepath.Join(assets, "src")}
	if len(launched) != 2 || launched[0] != want[0] || launched[1] != want[1] {
		t.Fatalf("a document and a folder go to the desktop: got %v, want %v", launched, want)
	}
	if len(revealed) != 1 || revealed[0] != filepath.Join(assets, "go.command") {
		t.Fatalf("a program is shown, never run: %v", revealed)
	}
}
