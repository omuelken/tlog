package tui

import (
	"github.com/tilman-schieber/tlog/internal/markdown"
)

// Dragging the mouse across a block selects its text, the way it does in any
// program that has the mouse. The selection is the one v makes: letting go
// copies it, and it stays up so that d cuts it and c writes in its place. A
// selection stays inside one block; dragging past the block's edge takes it
// to the block's start or end, because a range of characters that ran across
// blocks would be neither text nor blocks.

// dragStart is where the left button went down: the screen line, the column,
// and the block drawn there.
type dragStart struct {
	line, x int
	block   *markdown.Block
	// begun is set once the pointer has moved with the button held, so a
	// press and release where it lay stays an ordinary click.
	begun bool
}

// dragFrom is the start of a drag for a press, or nil when the press is not
// on a block's text: links open, bullets fold and boxes tick instead.
func (m *Model) dragFrom(line, x int) *dragStart {
	if line < 0 || line >= len(m.lineRow) || m.lineRow[line] < 0 || m.hitAt(line, x) != nil {
		return nil
	}
	r := m.rows[m.lineRow[line]]
	if r.kind != rowBlock {
		return nil
	}
	textCol := r.depth*2 + 2
	if line == m.lineFirst[line] && r.block.Task() != markdown.NotATask {
		textCol += 2
	}
	if line == m.lineFirst[line] && x < textCol {
		return nil
	}
	return &dragStart{line: line, x: x, block: r.block}
}

// dragTo grows the selection to the cell under the pointer. y is the screen
// line, for a pointer that has left the outline above or below it.
func (m *Model) dragTo(line, y, x int) {
	d := m.drag
	if !d.begun {
		if line == d.line && x == d.x {
			return
		}
		if !m.beginDrag() {
			m.drag = nil
			return
		}
	}
	e := m.ed
	ri := m.cur
	switch {
	case line >= 0 && line < len(m.lineRow) && m.lineRow[line] == ri:
		e.setCaret(line-m.lineFirst[line], x-m.rows[ri].depth*2-2)
	case m.before(line, y, ri):
		e.cur = 0
	default:
		e.cur = len(e.runes)
	}
}

// beginDrag goes into the block the press was on and starts the selection
// where it landed. The click has already put the caret there if the block was
// being written; otherwise it is found from where the block was drawn.
func (m *Model) beginDrag() bool {
	d := m.drag
	ri := -1
	for i, r := range m.rows {
		if r.kind == rowBlock && r.block == d.block {
			ri = i
			break
		}
	}
	if ri < 0 {
		return false // the press opened something, or the page changed
	}
	if m.ed == nil || m.edBlock != d.block {
		if ri != m.cur {
			m.moveCursor(ri - m.cur)
		}
		if m.current() != d.block {
			return false
		}
		m.attach(0)
		if d.line < len(m.lineFirst) {
			m.ed.setCaret(d.line-m.lineFirst[d.line], d.x-m.rows[ri].depth*2-2)
		}
	}
	m.vsel = nil
	m.mode = modeText
	m.tsel = &textSel{anchor: m.ed.cur}
	d.begun = true
	return true
}

// before says whether a screen line is above the block at row ri. A line off
// the outline is above it when the pointer is over the header.
func (m *Model) before(line, y, ri int) bool {
	if line < 0 || line >= len(m.lineRow) {
		return y < 1
	}
	if r := m.lineRow[line]; r >= 0 {
		return r < ri
	}
	for l := line; l < len(m.lineRow); l++ {
		if m.lineRow[l] == ri {
			return true
		}
	}
	return false
}

// endDrag is the button let go: what was selected is copied.
func (m *Model) endDrag() {
	d := m.drag
	m.drag = nil
	if d == nil || !d.begun || m.mode != modeText {
		return
	}
	text, _, _ := m.textSpan()
	if text == "" {
		m.tsel = nil
		return
	}
	m.status = "copied " + charsIn(text) + " — d cut · c change · esc back"
	m.copyText(text, false)
}
