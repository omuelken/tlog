package app

import (
	"fmt"
	"sync"

	"github.com/tilman-schieber/tlog/internal/markdown"
	"github.com/tilman-schieber/tlog/internal/store"
)

// Undo, for every adapter at once.
//
// Every write goes through Save, so Save is where the history is kept: the
// file as it was and as it became. An adapter marks where one thing a person
// did ends and the next begins (Checkpoint), so that pressing undo once takes
// back the whole of it — Tab mid-sentence is a save of the text and then an
// indent, and undoing half of that would be its own kind of damage.
//
// Undoing is a write like any other and is held to the same rule: it only
// happens if the file still says what this program last wrote. If something
// else has written to it since — nvim, a git pull, the other window — the undo
// is refused rather than putting the old text over theirs. The history lives
// in memory, for this session; git holds everything older.

// UndoDepth is how many steps are kept.
const UndoDepth = 200

// change is one file, before and after.
type change struct {
	rel           string
	before, after []byte
}

// step is everything one action wrote.
type step []change

type history struct {
	mu   sync.Mutex
	undo []step
	redo []step
	// open says the last step can still be added to: no checkpoint since.
	open bool
	// replaying is set while undo or redo writes, so that those writes are
	// not themselves recorded as something to undo.
	replaying bool
}

// Undone says what an undo or redo changed: every file, and the one to show.
type Undone struct {
	Rels []string `json:"rels"`
	Rel  string   `json:"rel"` // the file whose change was undone last
}

// Checkpoint ends the current step: whatever is written next is undone
// separately from what was written before. Adapters call it once per thing a
// person does — a key, a click — and a step with no writes in it costs
// nothing and is never seen.
func (s *Service) Checkpoint() {
	s.hist.mu.Lock()
	s.hist.open = false
	s.hist.mu.Unlock()
}

// record adds a write to the current step. Two writes to one file in one step
// are one change: the first before, the last after.
func (s *Service) record(rel string, before, after []byte) {
	h := &s.hist
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.replaying {
		return
	}
	h.redo = nil // a new change is a new branch; what was undone stays undone
	if !h.open || len(h.undo) == 0 {
		h.undo = append(h.undo, step{})
		if len(h.undo) > UndoDepth {
			h.undo = h.undo[len(h.undo)-UndoDepth:]
		}
		h.open = true
	}
	last := &h.undo[len(h.undo)-1]
	for i := range *last {
		if (*last)[i].rel == rel {
			(*last)[i].after = after
			return
		}
	}
	*last = append(*last, change{rel: rel, before: before, after: after})
}

// CanUndo and CanRedo say whether there is anything to take back or put back.
func (s *Service) CanUndo() bool {
	s.hist.mu.Lock()
	defer s.hist.mu.Unlock()
	return len(s.hist.undo) > 0
}

func (s *Service) CanRedo() bool {
	s.hist.mu.Lock()
	defer s.hist.mu.Unlock()
	return len(s.hist.redo) > 0
}

// Undo takes back the last step.
func (s *Service) Undo() (*Undone, error) { return s.replay(true) }

// Redo puts back the last step that was undone.
func (s *Service) Redo() (*Undone, error) { return s.replay(false) }

func (s *Service) replay(undo bool) (*Undone, error) {
	h := &s.hist
	h.mu.Lock()
	from, to := &h.redo, &h.undo
	word := "redo"
	if undo {
		from, to = &h.undo, &h.redo
		word = "undo"
	}
	if len(*from) == 0 {
		h.mu.Unlock()
		return nil, refuse("nothing to %s", word)
	}
	st := (*from)[len(*from)-1]
	h.mu.Unlock()

	// Every file first, then any writing: a step that cannot be taken back
	// whole is not taken back at all.
	for _, c := range st {
		want := c.after
		if !undo {
			want = c.before
		}
		cur, err := s.Store.Read(c.rel)
		if err != nil {
			return nil, err
		}
		if cur.Hash != markdown.Hash(want) {
			return nil, &store.ErrConflict{Rel: c.rel, Expected: markdown.Hash(want), Actual: cur.Hash}
		}
	}

	h.mu.Lock()
	h.replaying = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.replaying = false
		h.mu.Unlock()
	}()

	done := &Undone{}
	order := make([]change, len(st))
	copy(order, st)
	if undo {
		// Back out in the reverse of the order things were written.
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	for _, c := range order {
		from, to := c.after, c.before
		if !undo {
			from, to = c.before, c.after
		}
		if err := s.Store.Write(c.rel, to, markdown.Hash(from)); err != nil {
			return done, fmt.Errorf("%s stopped partway, at %s: %w", word, c.rel, err)
		}
		s.Git.Touch(c.rel)
		done.Rels = append(done.Rels, c.rel)
		done.Rel = c.rel
	}

	h.mu.Lock()
	*from = (*from)[:len(*from)-1]
	*to = append(*to, st)
	h.open = false // what is written next is not part of what was replayed
	h.mu.Unlock()
	return done, nil
}
