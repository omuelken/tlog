package app

import (
	"strings"

	"github.com/tilman-schieber/tlog/internal/markdown"
	"github.com/tilman-schieber/tlog/internal/store"
)

// Block references. The syntax and the graph have been here from the start —
// [[Page#^anchor]] parses, resolves and shows up in backlinks — with no way to
// make one. This is that way.
//
// The anchor is materialised at the moment a reference is made and not before,
// which is why Anchor writes and reading never does: a corpus that has never
// been referred to has no ^anchors in it at all.

// BlockRef is a block that could be referred to: what it says, so a chooser can
// show it, and where it is, so a link to it can be asked for.
type BlockRef struct {
	Addr   Addr   `json:"addr"`
	Text   string `json:"text"`
	Page   string `json:"page"`
	Rel    string `json:"rel"`
	Offset int    `json:"offset"`
	Hash   string `json:"hash"`
}

// Blocks finds blocks to refer to. It is the same search the palette uses, so
// what you can find you can refer to, with empty blocks left out because a
// reference to one says nothing.
func (l *Lookup) Blocks(query string, limit int) []BlockRef {
	out := []BlockRef{}
	for _, h := range l.g.Search(query, 0) {
		if strings.TrimSpace(h.Block.Text) == "" {
			continue
		}
		out = append(out, BlockRef{
			Addr:   Addr{Rel: h.Page.Rel, Offset: h.Offset, Hash: h.Hash},
			Text:   h.Block.Text,
			Page:   h.Page.Name,
			Rel:    h.Page.Rel,
			Offset: h.Offset,
			Hash:   h.Hash,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Blocks is Lookup.Blocks for a caller that wants one answer and no snapshot.
func (s *Service) Blocks(query string, limit int) ([]BlockRef, error) {
	l, err := s.Lookup()
	if err != nil {
		return nil, err
	}
	return l.Blocks(query, limit), nil
}

// RefTo gives a block a durable name and returns the link text that points at
// it. Calling it twice on the same block returns the same link and writes once,
// because EnsureAnchor keeps the anchor a block already has.
func (s *Service) RefTo(a Addr) (string, error) {
	link, _, err := s.Name(a)
	return link, err
}

// Name is RefTo together with where the block is afterwards. Naming a block
// rewrites its file, so the address it was named by is stale the moment the
// link exists; a caller that wants to go on working with the block needs the
// new one.
func (s *Service) Name(a Addr) (string, *Result, error) {
	d, b, err := s.Resolve(a)
	if err != nil {
		return "", nil, err
	}
	anchor := d.Doc.EnsureAnchor(b)
	res, err := s.commit(d, b)
	if err != nil {
		return "", nil, err
	}
	link := markdown.Link{Page: store.PageName(d.Rel), Anchor: anchor}
	return link.String(), res, nil
}
