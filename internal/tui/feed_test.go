package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tapp "github.com/tilman-schieber/tlog/internal/app"
)

// The journals page is driven exactly like a page: keys in, files out. What
// these tests pin down is that every edit lands in the day it was made on, and
// that nothing ever crosses from one day into another.

func dayRel(m *Model, daysAgo int) string {
	return m.svc.JournalRel(time.Now().AddDate(0, 0, -daysAgo))
}

// writeDay puts a day on disk the way any editor would.
func writeDay(t *testing.T, m *Model, daysAgo int, body string) string {
	t.Helper()
	rel := dayRel(m, daysAgo)
	if err := os.WriteFile(m.svc.Store.Abs(rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

// openJournals goes to the journals page the way a person would: esc, J. It
// leaves the outliner moving, with the cursor on today's first block.
func openJournals(t *testing.T, m *Model) {
	t.Helper()
	send(t, m, k(tea.KeyEsc), runes("J"))
	if m.feed == nil {
		t.Fatal("J did not open the journals page")
	}
	m.View()
}

// cursorTo puts the cursor on the block with this text.
func cursorTo(t *testing.T, m *Model, text string) {
	t.Helper()
	for i, r := range m.rows {
		if r.kind == rowBlock && r.block.Text == text {
			m.cur = i
			return
		}
	}
	t.Fatalf("no block %q on screen", text)
}

func TestJournalsAreNewestFirstUnderTheirOwnHeadings(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- yesterday\n")
	d3 := writeDay(t, m, 3, "- earlier\n")
	openJournals(t, m)

	var days []string
	for _, r := range m.rows {
		if r.kind == rowDay {
			days = append(days, r.rel)
		}
	}
	want := []string{m.svc.TodayRel(), d1, d3}
	if strings.Join(days, " ") != strings.Join(want, " ") {
		t.Fatalf("days %v, want %v", days, want)
	}
	if m.rowKind() != rowBlock || m.docOf(m.current()).Rel != m.svc.TodayRel() {
		t.Fatal("the cursor should start on today's first block")
	}
	if _, err := os.Stat(m.svc.Store.Abs(m.svc.TodayRel())); err == nil {
		t.Fatal("opening the journals page wrote today's file — reading must never write")
	}
}

func TestAnEditOnTheJournalsPageGoesToThatDay(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- yesterday\n")
	d2 := writeDay(t, m, 2, "- before\n")
	openJournals(t, m)

	cursorTo(t, m, "before")
	send(t, m, runes("i"))
	typeText(t, m, " that")
	send(t, m, k(tea.KeyEsc))

	if got := fileOf(t, m, d2); got != "- before that\n" {
		t.Fatalf("day 2: %q", got)
	}
	if got := fileOf(t, m, d1); got != "- yesterday\n" {
		t.Fatalf("day 1 was touched: %q", got)
	}
}

func TestEnterStaysInTheSameDay(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- last of the day\n")
	d2 := writeDay(t, m, 2, "- other day\n")
	openJournals(t, m)

	cursorTo(t, m, "last of the day")
	send(t, m, runes("i"), k(tea.KeyEnter))
	typeText(t, m, "added")
	send(t, m, k(tea.KeyEsc))

	if got := fileOf(t, m, d1); got != "- last of the day\n\n- added\n" {
		t.Fatalf("day 1: %q", got)
	}
	if got := fileOf(t, m, d2); got != "- other day\n" {
		t.Fatalf("day 2: %q", got)
	}
}

func TestADaysEdgeIsAWall(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- only\n")
	d2 := writeDay(t, m, 2, "- top\n")
	openJournals(t, m)

	cursorTo(t, m, "top")
	send(t, m, k(tea.KeyTab))                          // nothing above it in its own file
	send(t, m, tea.KeyMsg{Type: tea.KeyUp, Alt: true}) // already first in its file
	cursorTo(t, m, "only")
	send(t, m, tea.KeyMsg{Type: tea.KeyDown, Alt: true}) // already last in its file

	if got := fileOf(t, m, d1); got != "- only\n" {
		t.Fatalf("day 1: %q", got)
	}
	if got := fileOf(t, m, d2); got != "- top\n" {
		t.Fatalf("day 2: %q", got)
	}
}

func TestBackspaceDoesNotMergeAcrossDays(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- newer\n")
	d2 := writeDay(t, m, 2, "- older\n")
	openJournals(t, m)

	cursorTo(t, m, "older")
	send(t, m, runes("i"))
	m.ed.cur = 0 // the caret at the very start of the block
	send(t, m, k(tea.KeyBackspace), k(tea.KeyEsc))
	if got := fileOf(t, m, d1); got != "- newer\n" {
		t.Fatalf("day 1: %q", got)
	}
	if got := fileOf(t, m, d2); got != "- older\n" {
		t.Fatalf("day 2: %q", got)
	}
}

func TestWritingTodayFromTheJournalsPageCreatesIt(t *testing.T) {
	m := newModel(t)
	writeDay(t, m, 1, "- yesterday\n")
	openJournals(t, m)

	send(t, m, k(tea.KeyEnter))
	typeText(t, m, "first today")
	send(t, m, k(tea.KeyEsc))
	if got := fileOf(t, m, m.svc.TodayRel()); got != "- first today\n" {
		t.Fatalf("today: %q", got)
	}
}

func TestArrowingOutOfABlockWhileTypingSkipsTheDayHeading(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- one\n")
	writeDay(t, m, 2, "- two\n")
	openJournals(t, m)

	cursorTo(t, m, "one")
	send(t, m, runes("i"), k(tea.KeyDown))
	if m.mode != modeOutline || m.edBlock == nil || m.edBlock.Text != "two" {
		t.Fatalf("down from the last block of a day should carry on into the next day's first block")
	}
	typeText(t, m, "!")
	send(t, m, k(tea.KeyEsc))
	if got := fileOf(t, m, dayRel(m, 2)); got != "- two!\n" {
		t.Fatalf("day 2: %q", got)
	}
	if got := fileOf(t, m, d1); got != "- one\n" {
		t.Fatalf("day 1: %q", got)
	}
}

func TestFoldingIsPerDay(t *testing.T) {
	m := newModel(t)
	writeDay(t, m, 1, "- a\n  - a child\n")
	writeDay(t, m, 2, "- b\n  - b child\n")
	openJournals(t, m)

	cursorTo(t, m, "a")
	send(t, m, k(tea.KeyLeft))
	for _, r := range m.rows {
		if r.kind == rowBlock && r.block.Text == "a child" {
			t.Fatal("a was not folded")
		}
	}
	found := false
	for _, r := range m.rows {
		if r.kind == rowBlock && r.block.Text == "b child" {
			found = true
		}
	}
	if !found {
		t.Fatal("folding a block on one day folded the same position on another")
	}
}

func TestOlderDaysOpenAsYouReachTheBottom(t *testing.T) {
	m := newModel(t)
	for i := 1; i <= feedChunk+6; i++ {
		writeDay(t, m, i, "- note\n")
	}
	openJournals(t, m)
	if len(m.feed.days) != feedChunk {
		t.Fatalf("opened %d days up front", len(m.feed.days))
	}
	send(t, m, k(tea.KeyCtrlEnd), k(tea.KeyCtrlEnd))
	if len(m.feed.days) != feedChunk+7 { // the six extra, the fourteen, and today
		t.Fatalf("opened %d days after reaching the bottom", len(m.feed.days))
	}
}

func TestAnOutsideEditToAnotherDayDoesNotDisturbTyping(t *testing.T) {
	m := newModel(t)
	writeDay(t, m, 1, "- typing here\n")
	d2 := writeDay(t, m, 2, "- old text\n")
	openJournals(t, m)

	cursorTo(t, m, "typing here")
	send(t, m, runes("i"))
	typeText(t, m, " and more")

	writeDay(t, m, 2, "- changed in nvim\n")
	m.Update(changed(t, m, d2))

	if m.mode != modeOutline || m.ed.String() != "typing here and more" {
		t.Fatalf("typing was disturbed: mode=%d text=%q", m.mode, m.ed.String())
	}
	send(t, m, k(tea.KeyEsc))
	if got := fileOf(t, m, dayRel(m, 1)); got != "- typing here and more\n" {
		t.Fatalf("day 1: %q", got)
	}
	cursorTo(t, m, "changed in nvim") // the other day was reloaded
}

func TestJournalsIsARecentPageAndHasAWayBack(t *testing.T) {
	m := newModel(t)
	writeDay(t, m, 1, "- yesterday\n")
	openJournals(t, m)

	if m.recent[0] != tapp.JournalsRel {
		t.Fatalf("recent: %v", m.recent)
	}
	if head := headerText(m); !strings.HasPrefix(head, "Journals") {
		t.Fatalf("header: %q", head)
	}
	send(t, m, runes("b"))
	if m.feed != nil || m.doc.Rel != m.svc.TodayRel() {
		t.Fatalf("b should return to today's page, on %s", m.doc.Rel)
	}
}

func TestClickingADayHeadingOpensThatDay(t *testing.T) {
	m := newModel(t)
	d1 := writeDay(t, m, 1, "- yesterday\n")
	openJournals(t, m)
	y := -1
	lines, _ := m.bodyLines()
	for line, ri := range m.lineRow {
		if ri >= 0 && m.rows[ri].kind == rowDay && m.rows[ri].rel == d1 && strings.TrimSpace(stripANSI(lines[line])) != "" {
			y = line - m.bodyTop + 1
		}
	}
	if y < 0 {
		t.Fatal("yesterday's heading was not drawn")
	}
	m.Update(clickAt(2, y))
	if m.feed != nil || m.doc.Rel != d1 {
		t.Fatalf("on %s", m.doc.Rel)
	}
}

func TestBracketsStepBetweenDays(t *testing.T) {
	m := newModel(t)
	writeDay(t, m, 1, "- one\n")
	writeDay(t, m, 2, "- two\n")
	openJournals(t, m)

	send(t, m, runes("["))
	if m.current() == nil || m.current().Text != "one" {
		t.Fatal("[ should go to the next older day")
	}
	send(t, m, runes("["))
	if m.current() == nil || m.current().Text != "two" {
		t.Fatal("[ again should go one further")
	}
	send(t, m, runes("]"))
	if m.current() == nil || m.current().Text != "one" {
		t.Fatal("] should come back a day")
	}
}

func TestTheFrameNeverOutgrowsTheWindow(t *testing.T) {
	m := newModel(t)
	for i := 1; i <= 10; i++ {
		writeDay(t, m, i, "- note\n")
	}
	openJournals(t, m)
	m.height = 16
	frame := m.View()
	if n := strings.Count(frame, "\n") + 1; n > m.height {
		t.Fatalf("drew %d lines into a %d-line window — the header is pushed off the top", n, m.height)
	}
	if first := strings.SplitN(stripANSI(frame), "\n", 2)[0]; !strings.HasPrefix(strings.TrimPrefix(first, "󰠮 "), "Journals") {
		t.Fatalf("first line is not the header: %q", first)
	}
}
