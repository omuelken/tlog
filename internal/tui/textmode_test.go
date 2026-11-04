package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestVSelectsTextAndYCopiesIt(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- hello world\n")
	moving(t, m, "hello world")
	send(t, m, runes("v"))
	if m.mode != modeText || !strings.Contains(stripANSI(m.header()), "SELECT TEXT") {
		t.Fatalf("mode %v, header %q", m.mode, stripANSI(m.header()))
	}
	send(t, m, runes("e"), runes("y"))
	if fakeClip != "hello" {
		t.Fatalf("copied %q", fakeClip)
	}
	send(t, m, k(tea.KeyEsc))
	if m.mode != modeMove {
		t.Fatalf("esc should go back to moving, mode %v", m.mode)
	}
	if got := flat(t, m, m.doc.Rel); got != "- hello world\n" {
		t.Fatalf("copying changed the file: %q", got)
	}
}

func TestDCutsTextAndTheBlockIsWrittenOnLeaving(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- hello world\n")
	moving(t, m, "hello world")
	send(t, m, runes("v"), runes("w"), runes("d"))
	if fakeClip != "hello w" {
		t.Fatalf("cut %q", fakeClip)
	}
	if m.tsel != nil || m.mode != modeText {
		t.Fatal("cutting ends the selection and stays in the text")
	}
	send(t, m, k(tea.KeyEsc))
	if got := flat(t, m, m.doc.Rel); got != "- orld\n" {
		t.Fatalf("got %q", got)
	}
}

func TestCChangesTheSelection(t *testing.T) {
	m := treeModel(t, "- hello world\n")
	moving(t, m, "hello world")
	send(t, m, runes("v"), runes("e"), runes("c"))
	if m.mode != modeOutline {
		t.Fatalf("c should start writing, mode %v", m.mode)
	}
	typeText(t, m, "goodbye")
	send(t, m, k(tea.KeyEsc))
	if got := flat(t, m, m.doc.Rel); got != "- goodbye world\n" {
		t.Fatalf("got %q", got)
	}
}

func TestXAndPLikeVim(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- abc\n")
	moving(t, m, "abc")
	send(t, m, runes("v"), k(tea.KeyEsc)) // in the text, nothing selected
	send(t, m, runes("x"))                // takes the a
	send(t, m, runes("p"))                // puts it after the b
	send(t, m, k(tea.KeyEsc))
	if got := flat(t, m, m.doc.Rel); got != "- bac\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWordMotions(t *testing.T) {
	e := newEditor("one two.three  four")
	e.cur = 0
	e.wordForward()
	if e.cur != 4 {
		t.Fatalf("w from one: %d", e.cur)
	}
	e.wordForward()
	if e.cur != 7 { // "two" then "." is a word of its own
		t.Fatalf("w from two: %d", e.cur)
	}
	e.wordEnd() // already at the end of the one-character word ".", so on to the next
	if e.cur != 12 {
		t.Fatalf("e to the end of three: %d", e.cur)
	}
	e.wordBack()
	if e.cur != 8 {
		t.Fatalf("b to the start of three: %d", e.cur)
	}
}

func TestBlocksPastedInTextGoBelowTheBlock(t *testing.T) {
	m := treeModel(t, "- a\n- b\n")
	fakeClip = ""
	moving(t, m, "a")
	send(t, m, runes("y"), runes("y")) // whole block a
	cursorTo(t, m, "b")
	send(t, m, runes("v"), k(tea.KeyEsc), runes("p"))
	if got := flat(t, m, m.doc.Rel); got != "- a\n- b\n- a\n" {
		t.Fatalf("got %q", got)
	}
}
