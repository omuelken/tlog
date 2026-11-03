package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTypeTree(t *testing.T) {
	got, ok := parseTypeTree("(\n    \"com.apple.rtfd\",\n    \"com.apple.package\"\n)\n")
	if !ok || len(got) != 2 || got[0] != "com.apple.rtfd" || got[1] != "com.apple.package" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := parseTypeTree("(null)"); ok {
		t.Fatal("(null) is no answer")
	}
	if _, ok := parseTypeTree(""); ok {
		t.Fatal("nothing is no answer")
	}
}

// onDisk makes files and folders under the notes directory and returns the
// note a link to them would be written in.
func onDisk(t *testing.T, s *Service, files map[string]os.FileMode, dirs ...string) string {
	t.Helper()
	for _, d := range append(dirs, ".") {
		if err := os.MkdirAll(filepath.Join(s.Store.Root, "assets", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, mode := range files {
		if err := os.WriteFile(filepath.Join(s.Store.Root, "assets", name), []byte("x"), mode); err != nil {
			t.Fatal(err)
		}
	}
	return "pages/Note.md"
}

func TestLinkOpeningAsksTheSystem(t *testing.T) {
	s := newSvc(t)
	note := onDisk(t, s, map[string]os.FileMode{"report.docx": 0o644, "go.command": 0o644, "build.sh": 0o644},
		"Plan.pages", "src", "Calc.app")
	types := map[string][]string{
		"report.docx": {"org.openxmlformats.wordprocessingml.document", "public.data"},
		"Plan.pages":  {"com.apple.iwork.pages.sffpages", "com.apple.package", "public.directory"},
		"src":         {"public.folder", "public.directory"},
		"Calc.app":    {"com.apple.application-bundle", "com.apple.application", "public.executable", "com.apple.package"},
		"go.command":  {"com.apple.terminal.shell-script", "public.shell-script"},
		"build.sh":    {"public.shell-script", "public.script"},
	}
	old := contentTypes
	contentTypes = func(p string) ([]string, bool) { t, ok := types[filepath.Base(p)]; return t, ok }
	t.Cleanup(func() { contentTypes = old })

	for name, want := range map[string]Opening{
		"report.docx": OpenDocument,
		"Plan.pages":  OpenDocument,
		"src":         OpenFolder,
		"Calc.app":    OpenProgram,
		"go.command":  OpenProgram,
		"build.sh":    OpenDocument, // opened in an editor, not run
	} {
		_, got, err := s.LinkOpening(note, "../assets/"+name)
		if err != nil || got != want {
			t.Errorf("%s: got %v, %v; want %v", name, got, err, want)
		}
	}
	if _, got, _ := s.LinkOpening(note, "https://go.dev"); got != OpenAddress {
		t.Errorf("an address: got %v", got)
	}
}

func TestLinkOpeningGuessesWhereTheSystemCannotSay(t *testing.T) {
	old := contentTypes
	contentTypes = func(string) ([]string, bool) { return nil, false }
	t.Cleanup(func() { contentTypes = old })

	s := newSvc(t)
	note := onDisk(t, s, map[string]os.FileMode{"a.docx": 0o644, "go.command": 0o644, "run": 0o755, "tool.jar": 0o644},
		"folder", "Plan.pages", "Calc.app")
	for name, want := range map[string]Opening{
		"a.docx":     OpenDocument,
		"folder":     OpenFolder,
		"Plan.pages": OpenDocument,
		"Calc.app":   OpenProgram,
		"go.command": OpenProgram,
		"run":        OpenProgram,
		"tool.jar":   OpenProgram,
	} {
		if _, got, _ := s.LinkOpening(note, "../assets/"+name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestOpenLinkShowsAProgramInsteadOfRunningIt(t *testing.T) {
	old := contentTypes
	contentTypes = func(string) ([]string, bool) { return nil, false }
	t.Cleanup(func() { contentTypes = old })

	s := newSvc(t)
	note := onDisk(t, s, map[string]os.FileMode{"go.command": 0o644, "a.pdf": 0o644})
	var launched, revealed []string
	s.Launch = func(p string) error { launched = append(launched, p); return nil }
	s.Reveal = func(p string) error { revealed = append(revealed, p); return nil }

	for _, target := range []string{"../assets/go.command", "../assets/a.pdf"} {
		if err := s.OpenLink(note, target); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(s.Store.Root, "assets")
	if len(launched) != 1 || launched[0] != filepath.Join(dir, "a.pdf") {
		t.Fatalf("launched %v", launched)
	}
	if len(revealed) != 1 || revealed[0] != filepath.Join(dir, "go.command") {
		t.Fatalf("revealed %v", revealed)
	}
}
