package tui

import (
	"fmt"
	"strings"

	"github.com/tilman-schieber/tlog/internal/app"
	"github.com/tilman-schieber/tlog/internal/markdown"
)

// Selecting, copying, cutting and pasting whole blocks while moving, the way
// vim does it with lines: V starts a selection and j k grow it, y copies, d
// cuts, p and P put it back below or above. A block always comes with
// everything under it. The core does the work (Fragment, CutBlocks,
// PasteBlocks, ShiftBlocks), so what is cut here is cut the same way by a
// script; this file is the keys and the selection.

// visual is a selection of blocks being made. It is held by the block it
// started on rather than by row, so a redraw that moves rows around cannot
// make it start somewhere else.
type visual struct {
	rel    string
	offset int
}

// startVisual begins a selection at the block under the cursor.
func (m *Model) startVisual() {
	b := m.current()
	if b == nil {
		m.status = "nothing here to select — this lives on another page"
		return
	}
	d := m.docOf(b)
	m.vsel = &visual{rel: d.Rel, offset: b.Start}
	m.status = "selecting — j k to grow it · y copy · d cut · > < indent · esc stop"
}

// anchorRow is the row the selection started on, or -1 when it is gone.
func (m *Model) anchorRow() int {
	if m.vsel == nil {
		return -1
	}
	for i, r := range m.rows {
		if r.kind == rowBlock && r.block.Start == m.vsel.offset && r.doc != nil && r.doc.Rel == m.vsel.rel {
			return i
		}
	}
	return -1
}

// inSelection reports whether a row is drawn as selected: a block of the file
// the selection started in, between where it started and the cursor. A
// selection never reaches into another file — on the journals page it stops
// at the edge of its day, as every edit does.
func (m *Model) inSelection(i int) bool {
	a := m.anchorRow()
	if a < 0 || i < min(a, m.cur) || i > max(a, m.cur) {
		return false
	}
	r := m.rows[i]
	return r.kind == rowBlock && r.doc != nil && r.doc.Rel == m.vsel.rel
}

// picked is the file and the offsets of the blocks to act on: the selection
// while there is one, otherwise the block under the cursor.
func (m *Model) picked() (*app.Doc, []int) {
	if m.vsel == nil || m.anchorRow() < 0 {
		b := m.current()
		if b == nil {
			return nil, nil
		}
		return m.docOf(b), []int{b.Start}
	}
	var d *app.Doc
	var offs []int
	for i := range m.rows {
		if m.inSelection(i) {
			d = m.rows[i].doc
			offs = append(offs, m.rows[i].block.Start)
		}
	}
	return d, offs
}

// onDisk makes sure the file the picked blocks are in exists, since the core
// addresses blocks by where they are in the file: a day shown with a blank
// bullet has no file until something is written to it. A file that exists is
// left exactly as it is — rewriting it here would move the very offsets the
// blocks are about to be addressed by.
func (m *Model) onDisk(d *app.Doc) bool {
	if d == nil {
		return false
	}
	if d.Hash != markdown.Hash(nil) {
		return true
	}
	for _, r := range m.rows {
		if r.doc == d && r.kind == rowBlock {
			return m.ensureOnDisk(r.block)
		}
	}
	return true
}

// yank copies the picked blocks, each with everything under it.
func (m *Model) yank() {
	d, offs := m.picked()
	m.vsel = nil
	if d == nil {
		m.status = "nothing here to copy"
		return
	}
	if !m.onDisk(d) {
		return
	}
	text, err := m.svc.Fragment(d.Rel, d.Hash, offs)
	if err != nil {
		m.fail(err)
		return
	}
	m.status = "copied " + blocksIn(text)
	m.copyText(text, true)
}

// cutBlocks removes the picked blocks and keeps them to paste. It is what dd
// does, so a deleted block is never further away than p.
func (m *Model) cutBlocks() {
	d, offs := m.picked()
	m.vsel = nil
	if d == nil {
		m.status = "nothing here to cut"
		return
	}
	if !m.onDisk(d) {
		return
	}
	res, text, err := m.svc.CutBlocks(d.Rel, d.Hash, offs)
	if err != nil {
		m.fail(err)
		return
	}
	if !m.apply(res, nil) {
		return
	}
	m.status = "cut " + blocksIn(text) + " — p puts it back"
	m.copyText(text, true)
}

// put pastes below the block under the cursor, or above it: whole blocks as
// blocks, and text copied elsewhere as a block of its own.
func (m *Model) put(before bool) {
	reg, ok := m.pasteText()
	if !ok {
		m.status = "nothing to paste — y copies, d cuts"
		return
	}
	m.putText(reg.text, before)
}

func (m *Model) putText(text string, before bool) {
	m.vsel = nil
	b := m.current()
	if b == nil {
		m.status = "paste onto a block of this page"
		return
	}
	if !m.onDisk(m.docOf(b)) {
		return
	}
	if m.apply(m.svc.PasteBlocks(m.addrOf(b), text, before)) {
		m.status = "pasted " + blocksIn(text)
	}
}

// shiftBlocks indents or outdents the picked blocks together.
func (m *Model) shiftBlocks(outdent bool) {
	d, offs := m.picked()
	if d == nil || !m.onDisk(d) {
		return
	}
	// As in vim, shifting ends the selection.
	m.vsel = nil
	m.apply(m.svc.ShiftBlocks(d.Rel, d.Hash, offs, outdent))
}

// visualKey handles a key while a selection is being made, reporting whether
// it did. Moving grows the selection and is left to the ordinary keys.
func (m *Model) visualKey(k string) bool {
	switch k {
	case "y":
		m.yank()
	case "d", "x":
		m.cutBlocks()
	case ">":
		m.shiftBlocks(false)
	case "<":
		m.shiftBlocks(true)
	case "esc", "V":
		m.vsel = nil
		m.status = ""
	case "up", "k", "down", "j", "pgup", "pgdown", "ctrl+home", "ctrl+end", "G":
		return false
	default:
		m.vsel = nil // anything else ends the selection and does what it does
		return false
	}
	return true
}

// blocksIn says how much was copied, for the status line.
func blocksIn(text string) string {
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "- ") || l == "-" {
			n++
		}
	}
	if n <= 1 {
		return "1 block"
	}
	return fmt.Sprintf("%d blocks", n)
}
