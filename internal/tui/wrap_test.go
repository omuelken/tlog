package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func resize(m *Model, w int) { m.Update(tea.WindowSizeMsg{Width: w, Height: 24}) }

const longBlock = "alpha beta gamma delta epsilon zeta eta theta"

// --- the column a run of arrows holds ---------------------------------------

func TestUpAndDownHoldTheColumnAcrossBlocks(t *testing.T) {
	m := treeModel(t, "- abcdef\n\n- abcdefghij\n\n- ab\n\n- abcdefgh\n")
	steps := []struct {
		key       tea.KeyType
		row, want int
	}{
		{tea.KeyDown, 1, 6}, // not the end of the line, which is 10
		{tea.KeyDown, 2, 2}, // the short block clamps
		{tea.KeyDown, 3, 6}, // and the column is still what is aimed at
		{tea.KeyUp, 2, 2},
		{tea.KeyUp, 1, 6},
	}
	for i, s := range steps {
		send(t, m, k(s.key))
		if m.cur != s.row || m.ed.cur != s.want {
			t.Fatalf("step %d: row %d caret %d, want row %d caret %d", i, m.cur, m.ed.cur, s.row, s.want)
		}
	}
}

func TestAnotherKeyEndsTheRunOfArrows(t *testing.T) {
	m := treeModel(t, "- abcdef\n\n- ab\n\n- abcdef\n")
	send(t, m, k(tea.KeyDown), k(tea.KeyLeft), k(tea.KeyDown))
	if m.cur != 2 || m.ed.cur != 1 {
		t.Fatalf("row %d caret %d, want the column left moved to", m.cur, m.ed.cur)
	}
}

func TestTheColumnIsTheOneOnScreenAtAnyDepth(t *testing.T) {
	m := treeModel(t, "- abcdef\n  - abcdef\n")
	send(t, m, k(tea.KeyDown))
	if m.cur != 1 || m.ed.cur != 4 {
		t.Fatalf("indented one level, the caret should be two to the left: row %d caret %d", m.cur, m.ed.cur)
	}
	send(t, m, k(tea.KeyUp))
	if m.cur != 0 || m.ed.cur != 6 {
		t.Fatalf("row %d caret %d", m.cur, m.ed.cur)
	}

	// Left of where the deeper text starts: the aim survives the round trip.
	m = treeModel(t, "- abcdef\n  - abcdef\n")
	m.ed.cur = 1
	send(t, m, k(tea.KeyDown))
	if m.ed.cur != 0 {
		t.Fatalf("caret %d", m.ed.cur)
	}
	send(t, m, k(tea.KeyUp))
	if m.ed.cur != 1 {
		t.Fatalf("the column was lost on the way: caret %d", m.ed.cur)
	}
}

func TestArrowsOffThePageLeaveTheCaretAlone(t *testing.T) {
	m := treeModel(t, "- abcdef\n")
	m.ed.cur = 3
	send(t, m, k(tea.KeyUp))
	if m.ed.cur != 3 {
		t.Fatalf("up from the first block moved the caret to %d", m.ed.cur)
	}
	send(t, m, k(tea.KeyDown))
	if m.ed.cur != 3 {
		t.Fatalf("down from the last block moved the caret to %d", m.ed.cur)
	}
}

func TestArrivingFromBelowLandsOnTheLastLine(t *testing.T) {
	m := treeModel(t, "- first\n  xy\n\n- abcdef\n")
	send(t, m, k(tea.KeyDown), k(tea.KeyHome), k(tea.KeyRight), k(tea.KeyUp))
	if m.cur != 0 || m.ed.cur != len("first\n")+1 {
		t.Fatalf("row %d caret %d", m.cur, m.ed.cur)
	}
}

// --- folding to the window ---------------------------------------------------

func TestALongBlockFoldsToTheWindowAndFollowsItsSize(t *testing.T) {
	m := treeModel(t, "- "+longBlock+"\n\n- short\n")
	resize(m, 20)
	lines, _ := m.bodyLines()
	if len(lines) != 4 { // three for the long block, one for short
		t.Fatalf("got %d lines: %q", len(lines), lines)
	}
	var text []string
	for _, l := range lines[:3] {
		if w := lipgloss.Width(l); w > 20 {
			t.Fatalf("%q is %d wide", l, w)
		}
		text = append(text, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(stripANSI(l)), "•")))
	}
	if got := strings.Join(text, " "); got != longBlock {
		t.Fatalf("folding changed the text: %q", got)
	}

	resize(m, 80)
	if lines, _ = m.bodyLines(); len(lines) != 2 {
		t.Fatalf("widening the window did not unfold: %q", lines)
	}
}

func TestADeadlineFoldsWithTheTextRatherThanFallingOffTheEdge(t *testing.T) {
	m := treeModel(t, "- "+longBlock+"\n  deadline:: 2030-01-02\n\n- other\n")
	send(t, m, k(tea.KeyDown))
	resize(m, 30)
	lines, _ := m.bodyLines()
	for _, l := range lines {
		if w := lipgloss.Width(l); w > 30 {
			t.Fatalf("%q is %d wide", l, w)
		}
	}
	if !strings.Contains(stripANSI(strings.Join(lines, "\n")), "⏰") {
		t.Fatalf("no deadline: %q", lines)
	}
}

func TestArrowsWalkThroughTheFoldedRowsOfABlock(t *testing.T) {
	m := treeModel(t, "- "+longBlock+"\n")
	resize(m, 20) // folds at 17 and 31; the caret is at the end, column 14
	send(t, m, k(tea.KeyUp))
	if m.cur != 0 || m.ed.cur != 30 {
		t.Fatalf("up should go one screen row: row %d caret %d", m.cur, m.ed.cur)
	}
	if _, line := m.bodyLines(); line != 1 {
		t.Fatalf("the view follows the caret to row 1, got %d", line)
	}
	send(t, m, k(tea.KeyUp))
	if m.ed.cur != 14 {
		t.Fatalf("caret %d", m.ed.cur)
	}
	send(t, m, k(tea.KeyDown), k(tea.KeyDown))
	if m.ed.cur != 45 {
		t.Fatalf("the column should come back on the last row: caret %d", m.ed.cur)
	}
}

func TestARowsFoldIsCrossedByTheBlockBelow(t *testing.T) {
	m := treeModel(t, "- "+longBlock+"\n\n- abcdef\n")
	resize(m, 20)
	send(t, m, k(tea.KeyDown), k(tea.KeyHome), k(tea.KeyRight), k(tea.KeyRight), k(tea.KeyUp))
	if m.cur != 0 || m.ed.cur != 33 {
		t.Fatalf("up should land on the block's last screen row: row %d caret %d", m.cur, m.ed.cur)
	}
}

func TestAClickFindsTheFoldedRow(t *testing.T) {
	m := treeModel(t, "- "+longBlock+"\n")
	resize(m, 20)
	m.View()                // a click is resolved against what was last drawn
	m.Update(clickAt(5, 2)) // second screen row, three into the text
	if m.ed.cur != 20 {
		t.Fatalf("caret %d", m.ed.cur)
	}
}

func TestSoftBreaksNeverLoseOrStall(t *testing.T) {
	cases := []struct {
		text string
		w    int
		want []int
	}{
		{"short", 10, nil},
		{"anything", 0, nil},
		{"abcd efgh", 5, []int{5}},
		{"abc def ghi", 7, []int{8}},   // a word ending at the edge stays on its row
		{"abcd efgh", 4, []int{5}},     // and the next row does not start with the space
		{"abcd ", 4, []int{4}},         // a space at the very end still makes a row for the caret
		{"abcdefghij", 4, []int{4, 8}}, // no space to fold at
		{"日本語日本語", 4, []int{2, 4}},     // two columns each
		{"日本", 1, []int{1}},            // wider than the row, and still moving
	}
	for _, c := range cases {
		if got := softBreaks([]rune(c.text), c.w); !reflect.DeepEqual(got, c.want) {
			t.Errorf("softBreaks(%q, %d) = %v, want %v", c.text, c.w, got, c.want)
		}
	}
}

func TestCutStyledCarriesTheStyleOntoTheNextRow(t *testing.T) {
	got := cutStyled("\x1b[7mabcd\x1b[0m", []int{2})
	want := []string{"\x1b[7mab\x1b[0m", "\x1b[7mcd\x1b[0m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestAFoldedLinkIsClickableOnBothRows(t *testing.T) {
	_, hits := wrapLine("• ", "  ", "abcd efgh", "abcd efgh", []linkHit{{from: 3, to: 7}}, 5)
	if len(hits) != 2 || len(hits[0]) != 1 || len(hits[1]) != 1 {
		t.Fatalf("%v", hits)
	}
	if h := hits[0][0]; h.from != 5 || h.to != 7 {
		t.Fatalf("first row: %v", h)
	}
	if h := hits[1][0]; h.from != 2 || h.to != 4 {
		t.Fatalf("second row: %v", h)
	}
}

// The screen is laid out the way the file is: with format.blank_lines on, an
// empty line separates two top-level blocks when either has children, and
// never comes between a block and its children. A click on the gap selects
// nothing.
func TestTopLevelBlocksHaveAGapAboveThem(t *testing.T) {
	m := treeModel(t, "- zero\n\n- one\n  - child\n\n- two\n- three\n")
	resize(m, 80)
	lines, _ := m.bodyLines()
	var plain []string
	for _, l := range lines {
		plain = append(plain, strings.TrimSpace(stripANSI(l)))
	}
	want := []string{"• zero", "", "▾ one", "• child", "", "• two", "• three"}
	if strings.Join(plain, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", plain)
	}
	if m.lineRow[1] != -1 || m.lineRow[4] != -1 || m.lineRow[5] != 3 {
		t.Fatalf("the gap belongs to a row: %v", m.lineRow)
	}

	m.svc.Cfg.Format.BlankLines = false
	if lines, _ = m.bodyLines(); len(lines) != 5 {
		t.Fatalf("with the setting off: %q", lines)
	}
}
