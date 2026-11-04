package main

import (
	"strings"
	"testing"
)

func TestTheWindowCopiesCutsAndPastesBlocks(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, err := a.AppendBlock(p.Rel, p.Hash, "a")
	if err != nil {
		t.Fatal(err)
	}
	e, _ = a.NewBlock(e.Page.Rel, e.Offset, e.Page.Hash, "b", false)
	e, _ = a.NewBlock(e.Page.Rel, e.Offset, e.Page.Hash, "c", false)
	pg := e.Page
	off := func(text string) int {
		t.Helper()
		for _, b := range pg.Blocks {
			if b.Text == text {
				return b.Offset
			}
		}
		t.Fatalf("no block %q", text)
		return 0
	}

	text, err := a.CopyBlocks(pg.Rel, pg.Hash, []int{off("a"), off("b")})
	if err != nil || text != "- a\n- b\n" {
		t.Fatalf("copy: %q %v", text, err)
	}

	cut, err := a.CutBlocks(pg.Rel, pg.Hash, []int{off("a")})
	if err != nil || cut.Text != "- a\n" || len(cut.Edit.Page.Blocks) != 2 {
		t.Fatalf("cut: %+v %v", cut, err)
	}
	pg = cut.Edit.Page

	e, err = a.PasteBlocks(pg.Rel, off("c"), pg.Hash, "- x\n  - y\n", false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range e.Page.Blocks {
		got = append(got, strings.Repeat(" ", b.Depth)+b.Text)
	}
	if strings.Join(got, "|") != "b|c|x| y" {
		t.Fatalf("after paste: %v", got)
	}

	pg = e.Page
	e, err = a.ShiftBlocks(pg.Rel, pg.Hash, []int{off("c")}, false)
	if err != nil || e.Page.Blocks[1].Depth != 1 {
		t.Fatalf("shift: %+v %v", e, err)
	}
}
