package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLinkOpensOnlyWhatALinkShouldBe(t *testing.T) {
	s := newSvc(t)
	file := filepath.Join(t.TempDir(), "plan.png")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(s.Store.Root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(s.Store.Root, "assets", "a.pdf")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ target, want string }{
		{"https://go.dev", "https://go.dev"},
		{"mailto:a@b.c", "mailto:a@b.c"},
		{"file://" + file, file},
		{file, file},
		{"../assets/a.pdf", local}, // relative to the note, like Logseq writes them
	} {
		got, err := s.ResolveLink("pages/Plan.md", c.target)
		if err != nil || got != c.want {
			t.Errorf("%q: got %q, %v; want %q", c.target, got, err, c.want)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///nowhere/at/all", "missing.png", ""} {
		if _, err := s.ResolveLink("pages/Plan.md", bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestOpenLinkHandsTheTargetOver(t *testing.T) {
	s := newSvc(t)
	var opened []string
	s.Launch = func(target string) error { opened = append(opened, target); return nil }
	if err := s.OpenLink("journals/x.md", "https://go.dev"); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != "https://go.dev" {
		t.Fatalf("got %v", opened)
	}
}

func TestDroppedFilesReadsWhatTerminalsType(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plan.png")
	spaced := filepath.Join(dir, "my notes.pdf")
	for _, p := range []string{plain, spaced} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escaped := strings.ReplaceAll(spaced, " ", `\ `)

	for _, drop := range []string{
		plain + " " + escaped,              // kitty, ghostty: shell-escaped
		"'" + plain + "' '" + spaced + "'", // quoted
		"file://" + plain + "\nfile://" + strings.ReplaceAll(spaced, " ", "%20"), // a uri-list
	} {
		got := DroppedFiles(drop)
		if len(got) != 2 || got[0] != plain || got[1] != spaced {
			t.Errorf("%q: got %q", drop, got)
		}
	}
	for _, text := range []string{
		"just some text",
		plain + " and some words", // a path in prose is not a drop
		dir,                       // a directory is not a file to attach
		filepath.Join(dir, "gone.png"),
		"",
	} {
		if got := DroppedFiles(text); got != nil {
			t.Errorf("%q was taken for a drop: %q", text, got)
		}
	}
}
