package main

// Copying, cutting and pasting blocks, for a selection made in the window.
// The core does the work, so the window and the outliner put the same text on
// the clipboard — outline markdown, as the file stores it — and read it back
// the same way. offsets are where the selected blocks start in the file rel,
// as drawn with the hash.

// CopyBlocks is the selected blocks as outline markdown, each with everything
// under it. Nothing is written.
func (a *API) CopyBlocks(rel, hash string, offsets []int) (string, error) {
	return a.svc.Fragment(rel, hash, offsets)
}

// CutEdit is a cut: the page as it now stands, and the text that was cut.
type CutEdit struct {
	Edit *Edit  `json:"edit"`
	Text string `json:"text"`
}

// CutBlocks removes the selected blocks in one write and hands back their
// text for the clipboard.
func (a *API) CutBlocks(rel, hash string, offsets []int) (*CutEdit, error) {
	res, text, err := a.svc.CutBlocks(rel, hash, offsets)
	e, err := a.after(res, err)
	if err != nil {
		return nil, err
	}
	return &CutEdit{Edit: e, Text: text}, nil
}

// PasteBlocks puts outline markdown in as blocks below the block at offset, or
// above it. Text that is not an outline becomes one block.
func (a *API) PasteBlocks(rel string, offset int, hash, text string, before bool) (*Edit, error) {
	return a.after(a.svc.PasteBlocks(addr(rel, offset, hash), text, before))
}

// ShiftBlocks indents, or outdents, the selected blocks together.
func (a *API) ShiftBlocks(rel, hash string, offsets []int, outdent bool) (*Edit, error) {
	return a.after(a.svc.ShiftBlocks(rel, hash, offsets, outdent))
}
