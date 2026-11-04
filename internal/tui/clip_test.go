package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// flat is a file's text without the blank lines a setting may put between
// top-level blocks: the shape is what these tests are about.
func flat(t *testing.T, m *Model, rel string) string {
	t.Helper()
	return strings.ReplaceAll(fileOf(t, m, rel), "\n\n", "\n")
}

// moving puts the outliner into MOVE on the block with this text.
func moving(t *testing.T, m *Model, text string) {
	t.Helper()
	send(t, m, k(tea.KeyEsc))
	cursorTo(t, m, text)
}

func TestYyPCopiesABlockWithWhatIsUnderIt(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- a\n  - a1\n- b\n")
	moving(t, m, "a")
	send(t, m, runes("y"), runes("y"))
	if fakeClip != "- a\n  - a1\n" {
		t.Fatalf("the system clipboard got %q", fakeClip)
	}
	cursorTo(t, m, "b")
	send(t, m, runes("p"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n  - a1\n- b\n- a\n  - a1\n" {
		t.Fatalf("got %q", got)
	}
	if m.current() == nil || m.current().Text != "a" {
		t.Fatal("the cursor should be on what was pasted")
	}
}

func TestDdCutsAndPPutsItBack(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- a\n- b\n- c\n")
	moving(t, m, "a")
	send(t, m, runes("d"), runes("d"))
	if got := flat(t, m, m.doc.Rel); got != "- b\n- c\n" {
		t.Fatalf("after dd: %q", got)
	}
	cursorTo(t, m, "c")
	send(t, m, runes("p"))
	if got := flat(t, m, m.doc.Rel); got != "- b\n- c\n- a\n" {
		t.Fatalf("after p: %q", got)
	}
	cursorTo(t, m, "b")
	send(t, m, runes("P"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n- b\n- c\n- a\n" {
		t.Fatalf("after P: %q", got)
	}
}

func TestVSelectsBlocksToCopyOrCut(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- a\n- b\n  - b1\n- c\n- d\n")
	moving(t, m, "b")
	send(t, m, runes("V"), runes("j"), runes("j"))
	if !strings.Contains(stripANSI(m.header()), "SELECT") {
		t.Fatalf("the header should say a selection is being made: %q", stripANSI(m.header()))
	}
	send(t, m, runes("y"))
	if fakeClip != "- b\n  - b1\n- c\n" {
		t.Fatalf("copied %q", fakeClip)
	}
	if m.vsel != nil {
		t.Fatal("copying should end the selection")
	}

	cursorTo(t, m, "b")
	send(t, m, runes("V"), runes("j"), runes("j"), runes("d"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n- d\n" {
		t.Fatalf("after cutting the selection: %q", got)
	}
	cursorTo(t, m, "a")
	send(t, m, runes("p"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n- b\n  - b1\n- c\n- d\n" {
		t.Fatalf("pasted back: %q", got)
	}
}

func TestEscEndsASelectionWithoutDoingAnything(t *testing.T) {
	m := treeModel(t, "- a\n- b\n")
	moving(t, m, "a")
	send(t, m, runes("V"), runes("j"), k(tea.KeyEsc))
	if m.vsel != nil || m.mode != modeMove {
		t.Fatalf("selection %v, mode %v", m.vsel, m.mode)
	}
	if got := flat(t, m, m.doc.Rel); got != "- a\n- b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestShiftingASelection(t *testing.T) {
	m := treeModel(t, "- p\n- a\n- b\n- c\n")
	moving(t, m, "a")
	send(t, m, runes("V"), runes("j"), runes(">"))
	if got := flat(t, m, m.doc.Rel); got != "- p\n  - a\n  - b\n- c\n" {
		t.Fatalf("after >: %q", got)
	}
	cursorTo(t, m, "a")
	send(t, m, runes("V"), runes("j"), runes("<"))
	if got := flat(t, m, m.doc.Rel); got != "- p\n- a\n- b\n- c\n" {
		t.Fatalf("after <: %q", got)
	}
}

func TestAnOutlineCopiedElsewherePastesAsBlocks(t *testing.T) {
	m := treeModel(t, "- a\n")
	fakeClip = "- from nvim\n  - with a child\n"
	moving(t, m, "a")
	send(t, m, runes("p"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n- from nvim\n  - with a child\n" {
		t.Fatalf("got %q", got)
	}

	// Pasted with the terminal's own paste while moving, it goes in the same way.
	fakeClip = ""
	cursorTo(t, m, "a")
	send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("- pasted\n  - too"), Paste: true})
	if got := flat(t, m, m.doc.Rel); got != "- a\n- pasted\n  - too\n- from nvim\n  - with a child\n" {
		t.Fatalf("got %q", got)
	}
}

func TestASelectionStopsAtTheEdgeOfItsDay(t *testing.T) {
	fakeClip = ""
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- one\n")
	d2 := writeDay(t, m, 2, "- two\n")
	openJournals(t, m)
	cursorTo(t, m, "one")
	send(t, m, runes("V"), runes("j"), runes("d"))
	if got := flat(t, m, d1); got != "" {
		t.Fatalf("day 1: %q", got)
	}
	if got := flat(t, m, d2); got != "- two\n" {
		t.Fatalf("the cut reached into another day: %q", got)
	}
	if fakeClip != "- one\n" {
		t.Fatalf("cut %q", fakeClip)
	}
}

func TestColonOpensThePalette(t *testing.T) {
	m := newModel(t)
	send(t, m, k(tea.KeyEsc), runes(":"))
	if m.mode != modePalette {
		t.Fatalf("mode %v", m.mode)
	}
}
