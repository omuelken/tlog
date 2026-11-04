package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnlyPathsThatLeadSomewhereAreLinks(t *testing.T) {
	s := newSvc(t)
	if err := os.MkdirAll(filepath.Join(s.Store.Root, "projects", "tlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Store.Root, "projects")
	got := s.LivePaths("pages/Note.md", "in ../projects/tlog and "+dir+", not ../nowhere or and/or")
	if len(got) != 2 || got[0] != "../projects/tlog" || got[1] != dir {
		t.Fatalf("got %v", got)
	}
	before, _ := s.Store.List()
	s.LivePaths("pages/New.md", "./x")
	if after, _ := s.Store.List(); len(after) != len(before) {
		t.Fatal("looking at paths wrote a file")
	}
}

func TestAPastedPathIsWrittenSoItStaysAPath(t *testing.T) {
	s := newSvc(t)
	plain := filepath.Join(s.Store.Root, "plain")
	spaced := filepath.Join(s.Store.Root, "My Folder (old)")
	for _, d := range []string{plain, spaced} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for in, want := range map[string]string{
		plain:                       plain,
		"  " + plain + "\n":         plain,
		"file://" + plain:           plain,
		spaced:                      "[My Folder (old)](" + escaped(spaced) + ")",
		`"` + spaced + `"`:          "[My Folder (old)](" + escaped(spaced) + ")",
		shellEscape(spaced):         "[My Folder (old)](" + escaped(spaced) + ")",
		"file://" + urlPath(spaced): "[My Folder (old)](" + escaped(spaced) + ")",
	} {
		got, ok := s.PastedPath(in)
		if !ok || got != want {
			t.Errorf("%q: got %q %v, want %q", in, got, ok, want)
		}
	}
	for _, not := range []string{"/no/such/place", "some words", plain + "\n" + plain, "relative/path"} {
		if got, ok := s.PastedPath(not); ok {
			t.Errorf("%q was taken for a path: %q", not, got)
		}
	}

	// Under the home folder a path is written from ~.
	home, _ := os.UserHomeDir()
	if got := pathText(filepath.Join(home, "work", "x y")); got != "[x y](~/work/x%20y)" {
		t.Errorf("home: %q", got)
	}
	if got := pathText(filepath.Join(home, "work")); got != "~/work" {
		t.Errorf("home: %q", got)
	}
}

func escaped(p string) string {
	r := ""
	for _, c := range p {
		switch c {
		case ' ':
			r += "%20"
		case '(':
			r += "%28"
		case ')':
			r += "%29"
		default:
			r += string(c)
		}
	}
	return r
}

func shellEscape(p string) string {
	r := ""
	for _, c := range p {
		if c == ' ' || c == '(' || c == ')' {
			r += `\`
		}
		r += string(c)
	}
	return r
}

func urlPath(p string) string {
	r := ""
	for _, c := range p {
		if c == ' ' {
			r += "%20"
		} else {
			r += string(c)
		}
	}
	return r
}
