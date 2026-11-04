package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Selecting text inside a block, vim's way. v while moving goes into the
// block under the cursor with a selection begun at its first character; the
// motions grow it, and y copies, d cuts and c cuts and starts writing in its
// place. esc ends the selection, and esc again goes back to moving, writing
// what was cut. Text copied here is characters, not blocks: p puts it back
// inside a block, at the cursor.

// textSel is where a selection of text began, as a rune offset into the
// block, or nil while the cursor only moves.
type textSel struct{ anchor int }

// enterText goes into the block under the cursor, selecting from its start.
func (m *Model) enterText() {
	if m.current() == nil {
		m.status = "nothing here to select — this lives on another page"
		return
	}
	m.vsel = nil
	m.mode = modeText
	m.attach(0)
	m.tsel = &textSel{anchor: 0}
}

func (m *Model) textKey(msg tea.KeyMsg, k string) (tea.Model, tea.Cmd) {
	e := m.ed
	if e == nil {
		m.mode = modeMove
		return m, nil
	}
	switch k {
	case "h", "left":
		e.left()
	case "l", "right":
		e.right()
	case "j", "down":
		e.down()
	case "k", "up":
		e.up()
	case "w":
		e.wordForward()
	case "b":
		e.wordBack()
	case "e":
		e.wordEnd()
	case "0", "home":
		e.home()
	case "$", "end":
		e.end()
	case "v":
		if m.tsel == nil {
			m.tsel = &textSel{anchor: e.cur}
		} else {
			m.tsel = nil
		}
	case "y":
		m.textYank()
	case "d", "x":
		m.textCut(false, k == "x")
	case "c":
		m.textCut(true, false)
	case "p":
		m.textPut(false)
	case "P":
		m.textPut(true)
	case "i":
		m.tsel = nil
		m.mode = modeOutline
		return m, nil
	case "a":
		if e.cur < len(e.runes) && e.runes[e.cur] != '\n' {
			e.right()
		}
		m.tsel = nil
		m.mode = modeOutline
		return m, nil
	case "u":
		e.undo()
	case "esc":
		if m.tsel != nil {
			m.tsel = nil
			return m, nil
		}
		m.mode = modeMove
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.status = "selecting text — v select · y copy · d cut · c change · p paste · i write · esc back"
		}
	}
	if m.mode == modeText && m.ed != nil {
		m.ed.onChar()
	}
	return m, nil
}

// textSpan is the selected text and its range, or empty when nothing is
// selected.
func (m *Model) textSpan() (string, int, int) {
	if m.tsel == nil || m.ed == nil {
		return "", 0, 0
	}
	return m.ed.span(m.tsel.anchor, m.ed.cur)
}

func (m *Model) textYank() {
	text, _, _ := m.textSpan()
	if text == "" {
		m.status = "v first, then y"
		return
	}
	m.tsel = nil
	m.status = "copied " + charsIn(text)
	m.copyText(text, false)
}

// textCut takes the selection out of the block. Without one, x takes the
// character under the cursor, as in vim. change goes on to writing in its
// place. The block is written when it is left, like any typing.
func (m *Model) textCut(change, single bool) {
	text, lo, hi := m.textSpan()
	if text == "" && single && m.ed.cur < len(m.ed.runes) {
		lo, hi = m.ed.cur, m.ed.cur+1
		text = string(m.ed.runes[lo:hi])
	}
	if text == "" {
		m.status = "v first, then d or c"
		return
	}
	m.ed.cut(lo, hi)
	m.tsel = nil
	m.status = "cut " + charsIn(text)
	m.copyText(text, false)
	if change {
		m.mode = modeOutline
	}
}

// textPut pastes after the cursor, or before it. Blocks copied while moving
// are blocks, so they go in below this block rather than into its text.
func (m *Model) textPut(before bool) {
	reg, ok := m.pasteText()
	if !ok {
		m.status = "nothing to paste — v and y copy"
		return
	}
	if reg.lines {
		m.mode = modeMove
		m.leave()
		m.putText(reg.text, before)
		return
	}
	e := m.ed
	e.remember("insert")
	if !before && e.cur < len(e.runes) && e.runes[e.cur] != '\n' {
		e.cur++
	}
	e.put(strings.TrimRight(reg.text, "\n"))
	if e.cur > 0 {
		e.cur-- // on the last character pasted
	}
	m.tsel = nil
	m.status = "pasted " + charsIn(reg.text)
}

func charsIn(s string) string {
	n := len([]rune(s))
	if n == 1 {
		return "1 character"
	}
	return itoa(n) + " characters"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
