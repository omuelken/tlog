package app

import (
	"fmt"
	"strings"

	"github.com/tilman-schieber/tlog/internal/markdown"
	"github.com/tilman-schieber/tlog/internal/store"
)

// Copying, cutting and pasting blocks. What travels is outline markdown — the
// blocks exactly as the file stores them, "- a\n  - b" — so a copy pastes into
// nvim, a mail or another note as an outline, and an outline copied anywhere
// else pastes back as blocks. The clipboard itself belongs to the adapter;
// the core turns blocks into that text and that text back into blocks.
//
// A selection is a set of blocks in one file. Each block brings everything
// under it, and a block whose ancestor is also selected is already inside
// that ancestor, so it is not taken twice.

// Fragment renders the blocks starting at the given offsets in a file, each
// with everything under it, as outline markdown. The shallowest of them is
// written at the top level and the rest keep their depth relative to it, so
// the shape on screen is the shape that pastes.
//
// Reading only: nothing is written, and no anchor is created.
func (s *Service) Fragment(rel, hash string, offsets []int) (string, error) {
	_, roots, err := s.selection(rel, hash, offsets)
	if err != nil {
		return "", err
	}
	return renderFragment(roots), nil
}

// CutBlocks removes the blocks starting at the given offsets, each with
// everything under it, in one write, and returns them as Fragment would. The
// result is where the cursor belongs afterwards: whatever took the place of
// the first block removed, or the block before it.
func (s *Service) CutBlocks(rel, hash string, offsets []int) (*Result, string, error) {
	d, roots, err := s.selection(rel, hash, offsets)
	if err != nil {
		return nil, "", err
	}
	text := renderFragment(roots)

	flat := d.Doc.Flatten()
	first := indexIn(flat, roots[0])
	for _, b := range roots {
		if err := d.Doc.Remove(b); err != nil {
			return nil, "", err
		}
	}
	if err := s.Save(d); err != nil {
		return nil, "", err
	}
	fresh, err := s.Load(rel)
	if err != nil {
		return nil, "", err
	}
	res := &Result{Rel: rel, Hash: fresh.Hash}
	if left := fresh.Doc.Flatten(); len(left) > 0 {
		res.Offset = left[max(0, min(first, len(left)-1))].Start
	}
	return res.at(), text, nil
}

// PasteBlocks puts outline markdown into a file as blocks, after the block at
// the address — or before it — at that block's level. Blocks nested in the
// text stay nested under the block they were under. Text that is not an
// outline at all becomes one block.
//
// An anchor that the file already has is dropped from the pasted copy: a
// block's name is unique within its page, and a copy is not the block a link
// points at. Cut and pasted back, a block keeps its anchor, because the cut
// took the original away first.
//
// Pasting next to an empty block with nothing under it replaces that block,
// so pasting into a fresh page or day does not leave a blank bullet above.
// The result is the first block pasted.
func (s *Service) PasteBlocks(a Addr, text string, before bool) (*Result, error) {
	blocks := parseFragment(text)
	if len(blocks) == 0 {
		return nil, refuse("nothing to paste")
	}
	return s.mutateTo(a, func(d *markdown.Document, at *markdown.Block) (*markdown.Block, error) {
		dropTakenAnchors(d, blocks)
		ref := at
		for i, nb := range blocks {
			var err error
			switch {
			case before:
				err = d.InsertBefore(at, nb)
			case i == 0:
				err = d.InsertAfter(at, nb)
			default:
				err = d.InsertAfter(ref, nb)
			}
			if err != nil {
				return nil, err
			}
			ref = nb
		}
		if at.Text == "" && len(at.Children) == 0 && len(at.Props) == 0 && at.Anchor == "" {
			if err := d.Remove(at); err != nil {
				return nil, err
			}
		}
		return blocks[0], nil
	})
}

// mutateTo is mutate for a change that lands the cursor on a block other than
// the one addressed.
func (s *Service) mutateTo(a Addr, fn func(*markdown.Document, *markdown.Block) (*markdown.Block, error)) (*Result, error) {
	d, b, err := s.Resolve(a)
	if err != nil {
		return nil, err
	}
	land, err := fn(d.Doc, b)
	if err != nil {
		return nil, err
	}
	return s.commit(d, land)
}

// selection resolves offsets in one file to the blocks to take, in document
// order, leaving out any block already inside another one that is taken.
func (s *Service) selection(rel, hash string, offsets []int) (*Doc, []*markdown.Block, error) {
	if len(offsets) == 0 {
		return nil, nil, refuse("no block selected")
	}
	d, err := s.Load(rel)
	if err != nil {
		return nil, nil, err
	}
	if hash != "" && d.Hash != hash {
		return nil, nil, &store.ErrConflict{Rel: rel, Expected: hash, Actual: d.Hash}
	}
	picked := map[*markdown.Block]bool{}
	for _, off := range offsets {
		b := d.Doc.FindByOffset(off)
		if b == nil {
			return nil, nil, fmt.Errorf("no block at offset %d in %s", off, rel)
		}
		picked[b] = true
	}
	var roots []*markdown.Block
	d.Doc.Walk(func(b *markdown.Block) bool {
		if picked[b] && !insidePicked(b, picked) {
			roots = append(roots, b)
		}
		return true
	})
	return d, roots, nil
}

func insidePicked(b *markdown.Block, picked map[*markdown.Block]bool) bool {
	for p := b.Parent; p != nil; p = p.Parent {
		if picked[p] {
			return true
		}
	}
	return false
}

func indexIn(list []*markdown.Block, b *markdown.Block) int {
	for i, x := range list {
		if x == b {
			return i
		}
	}
	return 0
}

// renderFragment writes blocks as a file would, the shallowest at the top
// level and the others as far in as they were relative to it.
func renderFragment(roots []*markdown.Block) string {
	top := roots[0].Depth
	for _, b := range roots {
		top = min(top, b.Depth)
	}
	var sb strings.Builder
	for _, b := range roots {
		sb.Write(markdown.RenderBlock(b, b.Depth-top))
	}
	return sb.String()
}

// parseFragment reads outline markdown into top-level blocks, detached from
// the document they were parsed into so they can be put into another.
func parseFragment(text string) []*markdown.Block {
	text = strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	doc := markdown.Parse([]byte(text + "\n"))
	if len(doc.Frontmatter) > 0 {
		// "---" lines in pasted text are prose, not a page's frontmatter.
		doc = markdown.Parse([]byte(" " + text + "\n"))
	}
	for _, b := range doc.Blocks {
		b.Parent = nil
	}
	return doc.Blocks
}

// dropTakenAnchors removes from pasted blocks any anchor the document already
// uses, so no two blocks on a page answer to the same name.
func dropTakenAnchors(d *markdown.Document, pasted []*markdown.Block) {
	taken := map[string]bool{}
	d.Walk(func(b *markdown.Block) bool {
		if b.Anchor != "" {
			taken[b.Anchor] = true
		}
		return true
	})
	for _, root := range pasted {
		root.Walk(func(b *markdown.Block) bool {
			if b.Anchor != "" && taken[b.Anchor] {
				b.Anchor = ""
			}
			return true
		})
	}
}

// ShiftBlocks indents (or outdents) a selection of blocks in one write, each
// with everything under it, so they keep their places relative to one
// another: two siblings indented together end up side by side under the block
// above them, as two lines shifted together in an editor do. A block that
// cannot move — the first of its siblings has nothing to nest under, a
// top-level one has nowhere to go out to — stays where it is; if none can,
// the shift is refused. The result is the first block of the selection.
func (s *Service) ShiftBlocks(rel, hash string, offsets []int, outdent bool) (*Result, error) {
	d, roots, err := s.selection(rel, hash, offsets)
	if err != nil {
		return nil, err
	}
	moved := false
	if outdent {
		// Last first: each one lands just after its parent, so going
		// backwards keeps their order.
		for i := len(roots) - 1; i >= 0; i-- {
			moved = d.Doc.Outdent(roots[i]) || moved
		}
	} else {
		for _, b := range roots {
			moved = d.Doc.Indent(b) || moved
		}
	}
	if !moved {
		if outdent {
			return nil, refuse("already at the top level")
		}
		return nil, refuse("nothing above to nest under")
	}
	return s.commit(d, roots[0])
}
