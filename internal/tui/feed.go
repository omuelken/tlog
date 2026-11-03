package tui

import (
	"strings"
	"time"

	"github.com/tilman-schieber/tlog/internal/app"
	"github.com/tilman-schieber/tlog/internal/markdown"
	"github.com/tilman-schieber/tlog/internal/store"
)

// The journals page: every day, newest first, each still its own file. The
// outliner draws them under one another and otherwise behaves as it does on a
// single page; what changes is that each block row carries the day it belongs
// to (row.doc), and every edit is addressed through it. The core does the rest
// — it only ever rewrites one file per operation, so a day's edge is a wall.

// feedChunk is how many days are opened at a time. More are opened as the
// cursor nears the bottom, so a long history costs nothing until it is read.
const feedChunk = 14

type feed struct {
	rels []string  // every day on the page, newest first
	days []app.Day // the ones opened so far: always a prefix of rels
}

func (f *feed) index(rel string) int {
	for i, d := range f.days {
		if d.Doc.Rel == rel {
			return i
		}
	}
	return -1
}

// dayPrefix keeps a day's fold paths apart from every other day's.
func dayPrefix(rel string) string { return rel + ":" }

// blankIfEmpty puts a blank bullet on a page with nothing on it yet, so there
// is somewhere to type. It exists only in memory until something is written.
func blankIfEmpty(d *app.Doc) {
	if len(d.Doc.Blocks) == 0 {
		d.Doc.AppendChild(nil, &markdown.Block{})
	}
}

// openFeed (re)opens the journals page with at least n days.
func (m *Model) openFeed(n int) error {
	rels, err := m.svc.JournalDays()
	if err != nil {
		return err
	}
	n = min(len(rels), max(n, feedChunk))
	days, err := m.svc.OpenDays(rels[:n])
	if err != nil {
		return err
	}
	for _, d := range days {
		blankIfEmpty(d.Doc)
	}
	m.feed = &feed{rels: rels, days: days}
	// A stand-in, so that everything asking "which page is this" — the
	// header, history, the recent list — has an answer.
	m.doc = &app.Doc{Rel: app.JournalsRel, Doc: &markdown.Document{}}
	m.page = nil
	return nil
}

// reloadDay re-reads one day after it was written, leaving every other day —
// and anything being typed into one — exactly as it is.
func (m *Model) reloadDay(rel string) error {
	i := m.feed.index(rel)
	if i < 0 {
		return nil
	}
	keepRel, keepOff := m.cursorBlock()
	days, err := m.svc.OpenDays([]string{rel})
	if err != nil {
		return err
	}
	nd := days[0]
	blankIfEmpty(nd.Doc)
	m.collapsed = carryPrefixed(m.collapsed, dayPrefix(rel), m.feed.days[i].Doc.Doc, nd.Doc.Doc)
	m.feed.days[i] = nd
	m.stale = false
	m.buildRows()
	// A day above the cursor that grew or shrank would otherwise move the
	// cursor onto a different block. The day that was written is left to the
	// caller, which knows where in it the cursor belongs.
	if keepRel != "" && keepRel != rel {
		m.focusOffset(keepRel, keepOff)
	}
	return nil
}

// refreshDay re-reads what the notes say about one day — how its references
// resolve — without touching the document being edited.
func (m *Model) refreshDay(rel string) {
	i := m.feed.index(rel)
	if i < 0 {
		return
	}
	v, err := m.svc.View(rel)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.feed.days[i].View = v
}

// refreshFeed picks up days created or deleted elsewhere, and is what reloading
// the whole page means. The cursor stays on the block it was on.
func (m *Model) refreshFeed() error {
	keepRel, keepOff := m.cursorBlock()
	before := m.feed
	if err := m.openFeed(len(m.feed.days)); err != nil {
		return err
	}
	for _, d := range m.feed.days {
		if i := before.index(d.Doc.Rel); i >= 0 {
			m.collapsed = carryPrefixed(m.collapsed, dayPrefix(d.Doc.Rel), before.days[i].Doc.Doc, d.Doc.Doc)
		}
	}
	m.stale = false
	m.buildRows()
	if keepRel != "" {
		m.focusOffset(keepRel, keepOff)
	}
	return nil
}

// nearEnd opens the next days once the cursor is close to the bottom.
func (m *Model) nearEnd() {
	f := m.feed
	if f == nil || len(f.days) >= len(f.rels) || m.cur < len(m.rows)-3 {
		return
	}
	n := min(len(f.rels), len(f.days)+feedChunk)
	more, err := m.svc.OpenDays(f.rels[len(f.days):n])
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	for _, d := range more {
		blankIfEmpty(d.Doc)
	}
	f.days = append(f.days, more...)
	m.buildRows() // appended below, so every row index above stays put
}

// jumpDay moves to the next older (delta < 0) or newer (delta > 0) day, the
// way [ and ] step through days on a single journal.
func (m *Model) jumpDay(delta int) {
	m.leave()
	head := m.cur
	for head > 0 && m.rows[head].kind != rowDay {
		head--
	}
	target := -1
	if delta < 0 {
		for {
			for i := m.cur + 1; i < len(m.rows); i++ {
				if m.rows[i].kind == rowDay {
					target = i
					break
				}
			}
			if target >= 0 || len(m.feed.days) >= len(m.feed.rels) {
				break
			}
			m.cur = len(m.rows) - 1
			m.nearEnd()
		}
	} else {
		for i := head - 1; i >= 0; i-- {
			if m.rows[i].kind == rowDay {
				target = i
				break
			}
		}
	}
	if target < 0 {
		m.cur = max(0, min(m.cur, len(m.rows)-1))
		return
	}
	m.cur = target
	if target+1 < len(m.rows) && m.rows[target+1].kind == rowBlock {
		m.cur = target + 1
	}
	m.syncEditor()
}

// cursorBlock is the day and offset of the block under the cursor, for putting
// the cursor back after the rows are rebuilt.
func (m *Model) cursorBlock() (string, int) {
	if m.cur < 0 || m.cur >= len(m.rows) {
		return "", 0
	}
	r := m.rows[m.cur]
	if r.kind != rowBlock || r.doc == nil {
		return "", 0
	}
	return r.doc.Rel, r.block.Start
}

// carryPrefixed is carryCollapse for one day's share of the fold state.
func carryPrefixed(all map[string]bool, prefix string, before, after *markdown.Document) map[string]bool {
	mine := map[string]bool{}
	out := map[string]bool{}
	for k, v := range all {
		if rest, ok := strings.CutPrefix(k, prefix); ok {
			mine[rest] = v
		} else {
			out[k] = v
		}
	}
	for k, v := range carryCollapse(mine, before, after) {
		out[prefix+k] = v
	}
	return out
}

// dayHeading draws a day's heading: the date, the weekday, and whether it is
// today. Every day but the first has a blank line above it, as its own line.
func (m *Model) dayHeading(r row, selected bool) []string {
	label := styleTitle.Render(r.text)
	if selected {
		label = styleSelected.Render(r.text)
	}
	note := ""
	if t, ok := store.JournalDay(r.rel); ok {
		note = t.Format("Monday")
		if r.rel == m.svc.TodayRel() {
			note += " · today"
		} else if t.After(time.Now()) {
			note += " · ahead"
		}
	}
	line := label + " " + styleMuted.Render(note)
	if len(m.feed.days) > 0 && r.rel != m.feed.days[0].Doc.Rel {
		return []string{"", line}
	}
	return []string{line}
}
