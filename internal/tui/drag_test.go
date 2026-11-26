package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A drag is a press, motion with the button held, and a release, each on a
// frame drawn first, as the terminal would report them.
func drag(m *Model, y int, xs ...int) {
	m.View()
	m.Update(clickAt(xs[0], y))
	for _, x := range xs[1:] {
		m.View()
		m.Update(mouseAt(x, y, tea.MouseActionMotion, tea.MouseButtonLeft))
	}
	m.View()
	m.Update(mouseAt(xs[len(xs)-1], y, tea.MouseActionRelease, tea.MouseButtonLeft))
}

func TestDraggingSelectsTextAndCopiesIt(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- hello world\n- second\n")
	moving(t, m, "hello world")
	drag(m, 1, 2, 5, 6) // from "h" to the "o", both included
	if fakeClip != "hello" {
		t.Fatalf("copied %q", fakeClip)
	}
	if m.mode != modeText || m.tsel == nil {
		t.Fatalf("the selection should stay up: mode %v", m.mode)
	}
	send(t, m, k(tea.KeyEsc), k(tea.KeyEsc))
	if got := flat(t, m, m.doc.Rel); got != "- hello world\n- second\n" {
		t.Fatalf("copying changed the file: %q", got)
	}
}

func TestDraggingWhileWritingSelectsFromTheCaret(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- hello world\n")
	drag(m, 1, 8, 13) // "world"
	if fakeClip != "world" {
		t.Fatalf("copied %q", fakeClip)
	}
	send(t, m, runes("d"), k(tea.KeyEsc))
	if got := flat(t, m, m.doc.Rel); got != "- hello \n" {
		t.Fatalf("d should cut what was dragged over: %q", got)
	}
}

func TestDraggingOnAnotherBlockSelectsThere(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- first\n- second block\n")
	moving(t, m, "first")
	drag(m, 2, 2, 7)
	if fakeClip != "second" {
		t.Fatalf("copied %q", fakeClip)
	}
	if m.current() == nil || m.current().Text != "second block" {
		t.Fatal("the cursor should be on the block dragged over")
	}
}

func TestDraggingPastTheBlockTakesItToTheEnd(t *testing.T) {
	fakeClip = ""
	m := treeModel(t, "- hello world\n- next\n")
	moving(t, m, "hello world")
	m.View()
	m.Update(clickAt(8, 1))
	m.View()
	m.Update(mouseAt(3, 2, tea.MouseActionMotion, tea.MouseButtonLeft)) // onto "next"
	m.View()
	m.Update(mouseAt(3, 2, tea.MouseActionRelease, tea.MouseButtonLeft))
	if fakeClip != "world" {
		t.Fatalf("copied %q", fakeClip)
	}
}

func TestAClickWithoutMovingIsStillAClick(t *testing.T) {
	fakeClip = "untouched"
	m := treeModel(t, "- hello world\n")
	drag(m, 1, 4)
	if fakeClip != "untouched" || m.mode != modeOutline || m.ed == nil || m.ed.cur != 2 {
		t.Fatalf("a click should put the caret there: mode %v, clipboard %q", m.mode, fakeClip)
	}
}

func TestDraggingFromALinkDoesNotSelect(t *testing.T) {
	fakeClip = "untouched"
	m := treeModel(t, "- see [[Other]] now\n")
	moving(t, m, "see [[Other]] now")
	opened(m)
	drag(m, 1, 7, 10)
	if fakeClip != "untouched" || m.tsel != nil {
		t.Fatalf("a press on a link opens it rather than selecting: clipboard %q", fakeClip)
	}
}
